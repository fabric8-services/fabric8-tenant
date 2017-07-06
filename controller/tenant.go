package controller

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	uuid "github.com/satori/go.uuid"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	tenantService   tenant.Service
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
	templateVars    map[string]string
}

// NewTenantController creates a status controller.
func NewTenantController(service *goa.Service, tenantService tenant.Service, keycloakConfig keycloak.Config, openshiftConfig openshift.Config, templateVars map[string]string) *TenantController {
	return &TenantController{
		Controller:      service.NewController("TenantController"),
		tenantService:   tenantService,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
		templateVars:    templateVars,
	}
}

// AuthToken validates the KeyCloak token then returns the associated kubernetes token if kubernetes otherwise
// delegates to KeyCloak
func (c *TenantController) AuthToken(ctx *app.SetupTenantContext) error {
	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	params := ctx.Params
	broker := params.Get("broker")
	realm := params.Get("realm")
	if len(realm) == 0 {
		return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError("realm", "missing!"))
	}
	if len(broker) == 0 {
		return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError("broker", "missing!"))
	}
	if openshift.KubernetesMode() && realm == "fabric8" && broker == "openshift-v3" {
		// For Kubernetes lets serve the tokens from Kubernetes
		// for the KeyCloak username's associated ServiceAccount
		openshiftUserToken, err := c.OpenshiftToken(token)
		if len(openshiftUserToken) == 0 {
			return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
		}
		/*
			result := []byte("access_token=" + openshiftUserToken + "&scope=full&token_type=bearer")
			contentType := "application/octet-stream"
		*/
		result := []byte(`{"access_token":"` + openshiftUserToken + `","expires_in":31536000,"scope":"user:full","token_type":"Bearer"}`)
		contentType := "application/octet-stream"

		ctx.ResponseData.Header().Set("Content-Type", contentType)
		ctx.ResponseData.WriteHeader(200)
		ctx.ResponseData.Length = len(result)
		_, err = ctx.ResponseData.Write(result)
		return err
	}

	// delegate to the underlying KeyCloak server
	var body []byte
	fullUrl := strings.TrimSuffix(c.keycloakConfig.BaseURL, "/") + ctx.Request.RequestURI
	req, err := http.NewRequest("GET", fullUrl, bytes.NewReader(body))
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("Failed to forward request to KeyCloak: %v", err)))
	}
	copyHeaders := []string{"Authorization", "Content-Type", "Accept", "User-Agent", "Host", "Referrer"}
	for _, header := range copyHeaders {
		value := ctx.Request.Header.Get(header)
		if len(value) > 0 {
			req.Header.Set(header, value)
		}
	}
	client := c.CreateHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("Failed to invoke KeyCloak: %v", err)))
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()
	//result := string(b)
	status := resp.StatusCode
	ctx.ResponseData.Header().Set("Content-Type", "application/octet-stream")
	ctx.ResponseData.WriteHeader(status)
	ctx.ResponseData.Length = len(b)
	_, err = ctx.ResponseData.Write(b)
	return err
}

func (c *TenantController) CreateHttpClient() *http.Client {
	transport := c.openshiftConfig.HttpTransport
	if transport != nil {
		return &http.Client{
			Transport: transport,
		}
	}
	return http.DefaultClient
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

	openshiftUserToken, err := c.OpenshiftToken(token)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	openshiftUser, err := openshift.WhoAmI(c.openshiftConfig.WithToken(openshiftUserToken))
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	tenant := &tenant.Tenant{ID: ttoken.Subject(), Email: ttoken.Email()}
	c.tenantService.UpdateTenant(tenant)

	go func() {
		ctx := ctx
		t := tenant
		oc := c.openshiftConfig
		err = openshift.InitTenant(
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

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData, app.TenantHref()))
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

	openshiftUserToken, err := c.OpenshiftToken(token)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	openshiftUser, err := openshift.WhoAmI(c.openshiftConfig.WithToken(openshiftUserToken))
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	go func() {
		ctx := ctx
		t := tenant
		oc := c.openshiftConfig
		err = openshift.InitTenant(
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

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData, app.TenantHref()))
	return ctx.Accepted()
}

func (c *TenantController) OpenshiftToken(token *jwt.Token) (string, error) {
	if openshift.KubernetesMode() {
		// We don't currently store the Kubernetes token into KeyCloak for now
		// so lets try load the token for the ServiceAccount for the KeyCloak username
		// or lazily create a ServiceAccount if there is none created yet
		ttoken := &TenantToken{token: token}
		kcUserName := ttoken.Username()
		return openshift.GetOrCreateKubeToken(c.openshiftConfig, kcUserName)
	}
	return keycloak.OpenshiftToken(c.keycloakConfig, token.Raw)
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

	response := app.Tenant{
		ID:   &tenantID,
		Type: "tenants",
		Attributes: &app.TenantAttributes{
			CreatedAt:  &tenant.CreatedAt,
			Email:      &tenant.Email,
			Namespaces: []*app.NamespaceAttributes{},
		},
	}
	for _, ns := range namespaces {
		tenantType := string(ns.Type)
		response.Attributes.Namespaces = append(
			response.Attributes.Namespaces,
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

	return ctx.OK(&app.TenantSingle{Data: &response})
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
				service.UpdateNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   openshift.GetLabelVersion(request),
					Type:      GetNamespaceType(name),
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

// GetNamespaceType attempts to extract the namespace type based on namespace name
func GetNamespaceType(name string) tenant.NamespaceType {
	if strings.HasSuffix(name, "-jenkins") {
		return tenant.TypeJenkins
	}
	if strings.HasSuffix(name, "-che") {
		return tenant.TypeChe
	}
	if strings.HasSuffix(name, "-test") {
		return tenant.TypeTest
	}
	if strings.HasSuffix(name, "-stage") {
		return tenant.TypeStage
	}
	if strings.HasSuffix(name, "-run") {
		return tenant.TypeRun
	}
	return tenant.TypeUser
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
