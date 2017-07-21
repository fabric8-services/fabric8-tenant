package controller

import (
	"fmt"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
)

// TenantKubeController implements the tenantKube resource.
type TenantKubeController struct {
	*goa.Controller
	tenantService   tenant.Service
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
}

// NewTenantKubeController creates a tenantKube controller.
func NewTenantKubeController(service *goa.Service, tenantService tenant.Service, keycloakConfig keycloak.Config, openshiftConfig openshift.Config) *TenantKubeController {
	return &TenantKubeController{
		Controller:      service.NewController("TenantKubeController"),
		tenantService:   tenantService,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
	}
}

// KubeConnected checks that kubernetes tenant is connected with KeyCloak.
func (c *TenantKubeController) KubeConnected(ctx *app.KubeConnectedTenantKubeContext) error {
	fmt.Println("\n\nKubeConnected Looking for token!")
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

	openshiftUser, err :=  OpenShiftWhoAmI(token, c.openshiftConfig, openshiftUserToken)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	fmt.Println("\n\nKubeConnected about to try check!")

/*
	ttoken := &TenantToken{token: token}

	tenant := &tenant.Tenant{ID: ttoken.Subject(), Email: ttoken.Email()}
	c.tenantService.UpdateTenant(tenant)
	*/

	err = openshift.KubeConnected(
		c.keycloakConfig,
		c.openshiftConfig,
		openshiftUser)

	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}
	res := &app.TenantSingle{}
	return ctx.OK(res)
}
