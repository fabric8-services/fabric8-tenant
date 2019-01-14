package controller

import (
	"reflect"

	commonauth "github.com/fabric8-services/fabric8-common/auth"
	"github.com/fabric8-services/fabric8-common/errors"
	errs "github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/goadesign/goa"
)

var SERVICE_ACCOUNTS = []string{"fabric8-jenkins-idler", "rh-che"}

// TenantsController implements the tenants resource.
type TenantsController struct {
	*goa.Controller
	tenantService     tenant.Service
	clusterService    cluster.Service
	authClientService auth.Service
}

// NewTenantsController creates a tenants controller.
func NewTenantsController(service *goa.Service,
	tenantService tenant.Service,
	clusterService cluster.Service,
	authClientService auth.Service,
) *TenantsController {
	return &TenantsController{
		Controller:        service.NewController("TenantsController"),
		tenantService:     tenantService,
		clusterService:    clusterService,
		authClientService: authClientService,
	}
}

// Show runs the show action.
func (c *TenantsController) Show(ctx *app.ShowTenantsContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, SERVICE_ACCOUNTS...) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	tenantID := ctx.TenantID
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		serviceAccountName, _ := commonauth.ExtractServiceAccountName(ctx)
		log.Error(ctx, map[string]interface{}{
			"tenant_id":  tenantID,
			"error_type": reflect.TypeOf(err),
			"caller":     serviceAccountName,
		}, "error while looking-up tenant record")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	result := &app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.clusterService.GetCluster)}
	return ctx.OK(result)
}

// Search runs the search action.
func (c *TenantsController) Search(ctx *app.SearchTenantsContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, SERVICE_ACCOUNTS...) {
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
			convertTenant(ctx, tenant, namespaces, c.clusterService.GetCluster),
		},
		// skipping the paging links for now
		Meta: &app.TenantListMeta{
			TotalCount: 1,
		},
	}
	return ctx.OK(&result)
}

// Delete runs the `delete` action to deprovision a user
func (c *TenantsController) Delete(ctx *app.DeleteTenantsContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, "fabric8-auth") {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}
	tenantID := ctx.TenantID
	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	if len(namespaces) > 0 {
		// fetch the cluster info
		clustr, err := c.clusterService.GetCluster(ctx, namespaces[0].MasterURL)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":         err,
				"cluster_url": namespaces[0].MasterURL,
				"tenant_id":   tenantID,
			}, "unable to fetch cluster for user")
			return jsonapi.JSONErrorResponse(ctx, errs.NewInternalError(ctx, err))
		}
		openshiftConfig := openshift.Config{
			MasterURL: clustr.APIURL,
			Token:     clustr.Token,
		}
		err = openshift.DeleteNamespaces(ctx, tenantID, openshiftConfig, c.tenantService)
		if err != nil {
			return err
		}
	}
	// finally, delete the tenant record (all NS were already deleted, but that's fine)
	err = c.tenantService.DeleteTenant(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	log.Info(ctx, map[string]interface{}{"tenant_id": tenantID}, "tenant deleted")
	return ctx.NoContent()
}
