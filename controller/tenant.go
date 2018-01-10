package controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bitly/go-simplejson"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	tenantService   tenant.Service
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
	templateVars    map[string]string
	usersURL        string
}

// NewTenantController creates a status controller.
func NewTenantController(service *goa.Service, tenantService tenant.Service, keycloakConfig keycloak.Config, templateVars map[string]string, usersURL string) *TenantController {
	return &TenantController{
		Controller:     service.NewController("TenantController"),
		tenantService:  tenantService,
		keycloakConfig: keycloakConfig,
		templateVars:   templateVars,
		usersURL:       usersURL,
	}
}

func (c *TenantController) setupOpenShiftConfig(ctx *app.SetupTenantContext, userid string) (openshift.Config, error) {
	config, err := configuration.GetData()
	if err != nil {
		return openshift.Config{}, err
	}

	// fetch the user cluster information
	uc := token.UserController{Config: config}
	usercluster, err := uc.GetUserCluster(userid)
	if err != nil {
		return openshift.Config{}, err
	}

	// fetch that cluster's token
	sa := token.ServiceAccountTokenClient{Config: config}
	if err := sa.Get(); err != nil {
		return openshift.Config{}, err
	}

	ctc := token.ClusterTokenClient{
		Config:      config,
		AccessToken: sa.AuthServiceAccountToken,
	}
	clusterToken, err := ctc.Get(usercluster)
	if err != nil {
		return openshift.Config{}, err
	}

	var tr *http.Transport
	if config.APIServerInsecureSkipTLSVerify() {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	openshiftConfig := openshift.Config{
		MasterURL:      usercluster,
		ConsoleURL:     config.GetConsoleURL(), // something to do with cluster console url
		Token:          clusterToken,           // cluster token returned by auth service.
		HttpTransport:  tr,
		CheVersion:     config.GetOpenshiftCheVersion(),
		JenkinsVersion: config.GetOpenshiftJenkinsVersion(),
		TeamVersion:    config.GetOpenshiftTeamVersion(),
		TemplateDir:    config.GetOpenshiftTemplateDir(),
	}

	openshiftMasterUser, err := openshift.WhoAmI(openshiftConfig)
	if err != nil {
		logrus.Panic(nil, map[string]interface{}{
			"err": err,
		}, "unknown master user based on service token")
	}
	openshiftConfig.MasterUser = openshiftMasterUser

	return openshiftConfig, nil
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

	userid := ttoken.Subject().String()
	var err error
	c.openshiftConfig, err = c.setupOpenShiftConfig(ctx, userid)
	if err != nil {
		return err
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

	oc, err := c.loadUserTenantConfiguration(token, c.openshiftConfig)
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
	oc, err := c.loadUserTenantConfiguration(token, *rawOC)
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

func (c *TenantController) loadUserTenantConfiguration(token *jwt.Token, config openshift.Config) (openshift.Config, error) {
	authHeader := token.Raw
	if len(c.usersURL) > 0 {
		url := strings.TrimSuffix(c.usersURL, "/") + "/user"
		status, body, err := openshift.GetJSON(config, url, authHeader)
		if err != nil {
			return config, err
		}
		if status < 200 || status > 300 {
			return config, fmt.Errorf("Failed to GET url %s due to status code %d", url, status)
		}
		js, err := simplejson.NewJson([]byte(body))
		if err != nil {
			return config, err
		}

		tenantConfig := js.GetPath("data", "attributes", "contextInformation", "tenantConfig")
		if tenantConfig.Interface() != nil {
			cheVersion := getJsonStringOrBlank(tenantConfig, "cheVersion")
			jenkinsVersion := getJsonStringOrBlank(tenantConfig, "jenkinsVersion")
			teamVersion := getJsonStringOrBlank(tenantConfig, "teamVersion")
			mavenRepoURL := getJsonStringOrBlank(tenantConfig, "mavenRepo")
			return config.WithUserSettings(cheVersion, jenkinsVersion, teamVersion, mavenRepoURL), nil
		}
	}
	return config, nil
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
