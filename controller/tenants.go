package controller

import (
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/goadesign/goa"
)

// TenantsController implements the tenants resource.
type TenantsController struct {
	*goa.Controller
	tenantService  tenant.Service
	resolveCluster auth.ResolveCluster
}

// NewTenantsController creates a tenants controller.
func NewTenantsController(service *goa.Service, tenantService tenant.Service, resolveCluster auth.ResolveCluster) *TenantsController {
	return &TenantsController{
		Controller:     service.NewController("TenantsController"),
		tenantService:  tenantService,
		resolveCluster: resolveCluster,
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
	result := &app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.resolveCluster)}
	return ctx.OK(result)
}

// Search runs the search action.
func (c *TenantsController) Search(ctx *app.SearchTenantsContext) error {
	if !keycloak.IsSpecificServiceAccount(ctx, "fabric8-jenkins-idler") {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	tenant, err := c.tenantService.LookupTenantByClusterAndNamespace(ctx.MasterURL, ctx.Namespace)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	namespaces, err := c.tenantService.GetNamespaces(tenant.ID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	result := app.TenantList{
		Data: []*app.Tenant{
			convertTenant(ctx, tenant, namespaces, c.resolveCluster),
		},
		// skipping the paging links for now
		Meta: &app.TenantListMeta{
			TotalCount: 1,
		},
	}
	return ctx.OK(&result)
}
