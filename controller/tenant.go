package controller

import (
	"context"
	"net/http"

	"strings"

	"github.com/almighty/almighty-core/errors"
	"github.com/almighty/almighty-core/log"
	"github.com/almighty/almighty-core/rest"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8io/fabric8-init-tenant/app"
	"github.com/fabric8io/fabric8-init-tenant/jsonapi"
	"github.com/fabric8io/fabric8-init-tenant/keycloak"
	"github.com/fabric8io/fabric8-init-tenant/openshift"
	"github.com/fabric8io/fabric8-init-tenant/tenant"
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
}

// NewTenantController creates a status controller.
func NewTenantController(service *goa.Service, tenantService tenant.Service, keycloakConfig keycloak.Config, openshiftConfig openshift.Config) *TenantController {
	return &TenantController{
		Controller:      service.NewController("TenantController"),
		tenantService:   tenantService,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
	}
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

func (t TenantToken) Email() string {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		return claims["email"].(string)
	}
	return ""
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

	openshiftUserToken, err := keycloak.OpenshiftToken(c.keycloakConfig, token.Raw)
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
		err = openshift.InitTenant(oc, InitTenant(ctx, c.openshiftConfig.MasterURL, c.tenantService, t), openshiftUser, openshiftUserToken)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":     err,
				"os_user": openshiftUser,
			}, "unable initialize tenant")
			//return jsonapi.JSONErrorResponse(ctx, err)
		}
	}()

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData, app.TenantHref()))
	return ctx.Accepted()
}

// Show runs the setup action.
func (c *TenantController) Show(ctx *app.ShowTenantContext) error {
	authorization := goajwt.ContextJWT(ctx).Raw
	if authorization == "" {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	return ctx.OK(nil)
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
			return "PUT", request
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
		}
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
