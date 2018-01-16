package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"time"

	"github.com/bitly/go-simplejson"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/goasupport"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goaclient "github.com/goadesign/goa/client"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	httpClient      *http.Client
	tenantService   tenant.Service
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
	templateVars    map[string]string
	authURL         string
}

// NewTenantController creates a status controller.
func NewTenantController(service *goa.Service, tenantService tenant.Service, httpClient *http.Client, keycloakConfig keycloak.Config, openshiftConfig openshift.Config, templateVars map[string]string, authURL string) *TenantController {
	return &TenantController{
		Controller:      service.NewController("TenantController"),
		httpClient:      httpClient,
		tenantService:   tenantService,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
		templateVars:    templateVars,
		authURL:         authURL,
	}
}

// Setup runs the setup action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &TenantToken{token: token}
	exists := c.tenantService.Exists(ttoken.Subject())
	if exists {
		return ctx.Conflict()
	}

	openshiftUserToken, err := OpenshiftToken(c.keycloakConfig, c.openshiftConfig, token)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	openshiftUser, err := c.WhoAmI(token, openshiftUserToken)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	tenant := &tenant.Tenant{ID: ttoken.Subject(), Email: ttoken.Email()}
	err = c.tenantService.SaveTenant(tenant)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to store tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	oc, err := c.loadUserTenantConfiguration(ctx, c.openshiftConfig)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to load user tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	go func() {
		ctx := ctx
		t := tenant
		err = openshift.RawInitTenant(
			ctx,
			oc,
			InitTenant(ctx, c.openshiftConfig.MasterURL, c.tenantService, t),
			openshiftUser,
			openshiftUserToken,
			c.templateVars)

		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":     err,
				"os_user": openshiftUser,
			}, "unable initialize tenant")
		}
	}()

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

// Update runs the setup action.
func (c *TenantController) Update(ctx *app.UpdateTenantContext) error {
	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &TenantToken{token: token}
	tenant, err := c.tenantService.GetTenant(ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", ttoken.Subject().String()))
	}

	openshiftUserToken, err := OpenshiftToken(c.keycloakConfig, c.openshiftConfig, token)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	userConfig := c.openshiftConfig.WithToken(openshiftUserToken)
	openshiftUser, err := c.WhoAmI(token, openshiftUserToken)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	rawOC := &c.openshiftConfig
	if openshift.KubernetesMode() {
		rawOC = &userConfig
	}
	oc, err := c.loadUserTenantConfiguration(ctx, *rawOC)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to load user tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	go func() {
		ctx := ctx
		t := tenant
		err = openshift.RawUpdateTenant(
			ctx,
			oc,
			InitTenant(ctx, c.openshiftConfig.MasterURL, c.tenantService, t),
			openshiftUser,
			c.templateVars)

		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":     err,
				"os_user": openshiftUser,
			}, "unable initialize tenant")
		}
	}()

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

// loadUserTenantConfiguration loads the tenant configuration for `auth`,
// allowing for config overrides based on the content of his profile (in auth) if the user is allowed
func (c *TenantController) loadUserTenantConfiguration(ctx context.Context, config openshift.Config) (openshift.Config, error) {
	// restrict access to users with a `featureLevel` set to `internal`
	user, err := c.getCurrentUser(ctx)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"auth_url": c.authURL}, "unable get current user")
		return config, err
	}

	if user.Data.Attributes.FeatureLevel != nil && *user.Data.Attributes.FeatureLevel == "internal" {
		log.Debug(ctx,
			map[string]interface{}{
				"auth_url":      auth.ShowUserPath(),
				"user_name":     *user.Data.Attributes.Username,
				"feature_level": *user.Data.Attributes.FeatureLevel},
			"user is allowed to update tenant config")
		if tenantConfig, exists := user.Data.Attributes.ContextInformation["tenantConfig"]; exists {
			if tenantConfigMap, ok := tenantConfig.(map[string]interface{}); ok {
				var cheVersion, jenkinsVersion, teamVersion, mavenRepoURL *string
				if v, ok := tenantConfigMap["cheVersion"].(string); ok {
					cheVersion = &v
				}
				if v, ok := tenantConfigMap["jenkinsVersion"].(string); ok {
					jenkinsVersion = &v
				}
				if v, ok := tenantConfigMap["teamVersion"].(string); ok {
					teamVersion = &v
				}
				if v, ok := tenantConfigMap["mavenRepo"].(string); ok {
					mavenRepoURL = &v
				}
				return config.WithUserSettings(cheVersion, jenkinsVersion, teamVersion, mavenRepoURL), nil
			}
		}
	} else {
		curentFeatureLevel := "undefined"
		if user.Data.Attributes.FeatureLevel != nil {
			curentFeatureLevel = *user.Data.Attributes.FeatureLevel
		}
		log.Error(ctx,
			map[string]interface{}{
				"auth_url":      auth.ShowUserPath(),
				"user_name":     *user.Data.Attributes.Username,
				"feature_level": curentFeatureLevel},
			"user is not allowed to update tenant config")
	}
	return config, nil
}

func (c *TenantController) getCurrentUser(ctx context.Context) (*auth.User, error) {
	log.Info(ctx, map[string]interface{}{"auth_url": c.authURL, "http_client_transport": reflect.TypeOf(c.httpClient.Transport)}, "retrieving user's profile...")
	authClient, err := newAuthClient(ctx, c.httpClient, c.authURL)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"auth_url": c.authURL}, "unable to parse auth URL")
		return nil, err
	}
	resp, err := authClient.ShowUser(ctx, auth.ShowUserPath(), nil, nil)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"auth_url": auth.ShowUserPath()}, "unable to get user info")
		return nil, errs.Wrapf(err, "failed to GET %s due to error", auth.ShowUserPath())
	}
	if err != nil {
		log.Error(ctx, map[string]interface{}{"auth_url": auth.ShowUserPath()}, "unable to read auth response")
		return nil, errs.Wrapf(err, "failed to read auth response due to error", auth.ShowUserPath())
	}
	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		return nil, fmt.Errorf("failed to GET %s due to status code %d", resp.Request.URL, resp.StatusCode)
	}
	defer resp.Body.Close()
	user, err := authClient.DecodeUser(resp)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"auth_url": auth.ShowUserPath()}, "failed to decode user")
		return nil, errs.Wrapf(err, "failed to decode user")
	}
	return user, nil
}

func getJsonStringOrBlank(json *simplejson.Json, key string) string {
	text, _ := json.Get(key).String()
	return text
}

func (c *TenantController) WhoAmI(token *jwt.Token, openshiftUserToken string) (string, error) {
	return OpenShiftWhoAmI(token, c.openshiftConfig, openshiftUserToken)
}

func OpenShiftWhoAmI(token *jwt.Token, oc openshift.Config, openshiftUserToken string) (string, error) {
	if openshift.KubernetesMode() {
		// We don't currently store the Kubernetes token into KeyCloak for now
		// so lets try load the token for the ServiceAccount for the KeyCloak username
		// or lazily create a ServiceAccount if there is none created yet
		ttoken := &TenantToken{token: token}
		userName := ttoken.Username()
		if len(userName) == 0 {
			return "", fmt.Errorf("No username or preferred_username associated with the JWT token!")
		}
		return userName, nil
	}
	return openshift.WhoAmI(oc.WithToken(openshiftUserToken))
}

// Show runs the setup action.
func (c *TenantController) Show(ctx *app.ShowTenantContext) error {
	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}

	ttoken := &TenantToken{token: token}
	tenantID := ttoken.Subject()
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	result := &app.TenantSingle{Data: convertTenant(tenant, namespaces)}
	return ctx.OK(result)
}

// Clean runs the setup action for the tenant namespaces.
func (c *TenantController) Clean(ctx *app.CleanTenantContext) error {

	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}

	openshiftUserToken, err := OpenshiftToken(c.keycloakConfig, c.openshiftConfig, token)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	openshiftUser, err := c.WhoAmI(token, openshiftUserToken)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	err = openshift.CleanTenant(ctx, c.openshiftConfig, openshiftUser, c.templateVars)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	// TODO (xcoulon): respond with `204 No Content` instead ?
	return ctx.OK([]byte{})
}

// InitTenant is a Callback that assumes a new tenant is being created
func InitTenant(ctx context.Context, masterURL string, service tenant.Service, currentTenant *tenant.Tenant) openshift.Callback {
	return func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
		log.Info(ctx, map[string]interface{}{
			"status":    statusCode,
			"method":    method,
			"namespace": openshift.GetNamespace(request),
			"name":      openshift.GetName(request),
			"kind":      openshift.GetKind(request),
		}, "resource requested")
		if statusCode == http.StatusConflict {
			if openshift.GetKind(request) == openshift.ValKindNamespace {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindProjectRequest {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindPersistenceVolumeClaim {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindServiceAccount {
				return "", nil
			}
			return "DELETE", request
		} else if statusCode == http.StatusCreated {
			if openshift.GetKind(request) == openshift.ValKindProjectRequest {
				name := openshift.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   openshift.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name),
					MasterURL: masterURL,
				})

				// HACK to workaround osio applying some dsaas-user permissions async
				// Should loop on a Check if allowed type of call instead
				time.Sleep(time.Second * 2)

			} else if openshift.GetKind(request) == openshift.ValKindNamespace {
				name := openshift.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   openshift.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name),
					MasterURL: masterURL,
				})
			}
			return "", nil
		} else if statusCode == http.StatusOK {
			if method == "DELETE" {
				return "POST", request
			}
			return "", nil
		}
		log.Info(ctx, map[string]interface{}{
			"status":    statusCode,
			"method":    method,
			"namespace": openshift.GetNamespace(request),
			"name":      openshift.GetName(request),
			"kind":      openshift.GetKind(request),
			"request":   request,
			"response":  response,
		}, "unhandled resource response")
		return "", nil
	}
}

func OpenshiftToken(keycloakConfig keycloak.Config, openshiftConfig openshift.Config, token *jwt.Token) (string, error) {
	if openshift.KubernetesMode() {
		// We don't currently store the Kubernetes token into KeyCloak for now
		// so lets try load the token for the ServiceAccount for the KeyCloak username
		// or lazily create a ServiceAccount if there is none created yet
		ttoken := &TenantToken{token: token}
		kcUserName := ttoken.Username()
		return openshift.GetOrCreateKubeToken(openshiftConfig, kcUserName)
	}
	return keycloak.OpenshiftToken(keycloakConfig, token.Raw)
}

type TenantToken struct {
	token *jwt.Token
}

func (t TenantToken) Subject() uuid.UUID {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		id, err := uuid.FromString(claims["sub"].(string))
		if err != nil {
			return uuid.UUID{}
		}
		return id
	}
	return uuid.UUID{}
}

func (t TenantToken) Username() string {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		answer := claims["preferred_username"].(string)
		if len(answer) == 0 {
			answer = claims["username"].(string)
		}
		return answer
	}
	return ""
}

func (t TenantToken) Email() string {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		return claims["email"].(string)
	}
	return ""
}

func convertTenant(tenant *tenant.Tenant, namespaces []*tenant.Namespace) *app.Tenant {
	result := app.Tenant{
		ID:   &tenant.ID,
		Type: "tenants",
		Attributes: &app.TenantAttributes{
			CreatedAt:  &tenant.CreatedAt,
			Email:      &tenant.Email,
			Profile:    &tenant.Profile,
			Namespaces: []*app.NamespaceAttributes{},
		},
	}
	for _, ns := range namespaces {
		tenantType := string(ns.Type)
		result.Attributes.Namespaces = append(
			result.Attributes.Namespaces,
			&app.NamespaceAttributes{
				CreatedAt:  &ns.CreatedAt,
				UpdatedAt:  &ns.UpdatedAt,
				ClusterURL: &ns.MasterURL,
				Name:       &ns.Name,
				Type:       &tenantType,
				Version:    &ns.Version,
				State:      &ns.State,
			})
	}
	return &result
}

// NewAuthClient initializes a new client to the `auth` service
func newAuthClient(ctx context.Context, httpClient *http.Client, authURL string) (*auth.Client, error) {
	log.Info(ctx, map[string]interface{}{"auth_url": authURL, "http_client_transport": reflect.TypeOf(httpClient.Transport)}, "initializing a new auth client...")
	u, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	c := auth.New(goaclient.HTTPClientDoer(httpClient))
	c.Host = u.Host
	c.Scheme = u.Scheme
	c.SetJWTSigner(goasupport.NewForwardSigner(ctx))
	return c, nil
}
