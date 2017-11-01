package controller

import (
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/goadesign/goa"
)

// TenantsController implements the tenants resource.
type TenantsController struct {
	*goa.Controller
	tenantService tenant.Service
}

// NewTenantsController creates a tenants controller.
func NewTenantsController(service *goa.Service, tenantService tenant.Service) *TenantsController {
	return &TenantsController{
		Controller:    service.NewController("TenantsController"),
		tenantService: tenantService,
	}
}

// Show runs the show action.
func (c *TenantsController) Show(ctx *app.ShowTenantsContext) error {
	if !keycloak.IsSpecificServiceAccount(ctx, "fabric8-jenkins-idler") {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	tenantID := ctx.TenantID
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	return ctx.OK(convertTenant(tenant, namespaces))
}
