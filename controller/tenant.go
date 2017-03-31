package controller

import (
	"github.com/almighty/almighty-core/errors"
	"github.com/almighty/almighty-core/log"
	"github.com/fabric8io/fabric8-init-tenant/app"
	"github.com/fabric8io/fabric8-init-tenant/jsonapi"
	"github.com/fabric8io/fabric8-init-tenant/keycloak"
	"github.com/fabric8io/fabric8-init-tenant/openshift"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/jinzhu/gorm"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	db              *gorm.DB
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
}

// NewTenantController creates a status controller.
func NewTenantController(service *goa.Service, db *gorm.DB, keycloakConfig keycloak.Config, openshiftConfig openshift.Config) *TenantController {
	return &TenantController{
		Controller:      service.NewController("TenantController"),
		db:              db,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
	}
}

// Setup runs the show action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	authorization := goajwt.ContextJWT(ctx).Raw
	if authorization == "" {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	openshiftUserToken, err := keycloak.OpenshiftToken(c.keycloakConfig, authorization)
	if err != nil {
		log.Error(nil, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with keycloak")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not authorization against keycloak"))
	}

	openshiftUser, err := openshift.WhoAmI(c.openshiftConfig.WithToken(openshiftUserToken))
	if err != nil {
		log.Error(nil, map[string]interface{}{
			"err": err,
		}, "unable to authenticate user with tenant target server")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("unknown/unauthorized openshift user"))
	}

	err = openshift.InitTenant(c.openshiftConfig, openshiftUser, openshiftUserToken)
	if err != nil {
		log.Error(nil, map[string]interface{}{
			"err":     err,
			"os_user": openshiftUser,
		}, "unable initialize tenant")
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	return ctx.Created()
}
