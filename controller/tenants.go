package controller

import (
	"fmt"
	"reflect"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-tenant/user"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/goadesign/goa"
)

// TenantsController implements the tenants resource.
type TenantsController struct {
	*goa.Controller
	tenantService          tenant.Service
	resolveTenant          tenant.Resolve
	userService            user.Service
	openshiftService       openshift.Service
	resolveCluster         cluster.Resolve
	defaultOpenshiftConfig openshift.Config
}

// NewTenantsController creates a tenants controller.
func NewTenantsController(service *goa.Service,
	tenantService tenant.Service,
	userService user.Service,
	openshiftService openshift.Service,
	resolveTenant tenant.Resolve,
	resolveCluster cluster.Resolve,
	defaultOpenshiftConfig openshift.Config,
) *TenantsController {
	return &TenantsController{
		Controller:             service.NewController("TenantsController"),
		tenantService:          tenantService,
		resolveTenant:          resolveTenant,
		userService:            userService,
		openshiftService:       openshiftService,
		resolveCluster:         resolveCluster,
		defaultOpenshiftConfig: defaultOpenshiftConfig,
	}
}

// Show runs the show action.
func (c *TenantsController) Show(ctx *app.ShowTenantsContext) error {
	if !token.IsSpecificServiceAccount(ctx, "fabric8-jenkins-idler") {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	tenantID := ctx.TenantID
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"tenant_id": tenantID, "error_type": reflect.TypeOf(err)}, "error while looking-up tenant record")
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
	if !token.IsSpecificServiceAccount(ctx, "fabric8-jenkins-idler") {
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

// Delete runs the `delete` action to deprovision a user
func (c *TenantsController) Delete(ctx *app.DeleteTenantsContext) error {
	if !token.IsSpecificServiceAccount(ctx, "fabric8-auth") {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}
	tenantID := ctx.TenantID
	// fetch the cluster the user belongs to
	usr, err := c.userService.GetUser(ctx, tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	if usr.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster")))
	}
	// fetch the cluster info
	clustr, err := c.resolveCluster(ctx, *usr.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *usr.Cluster,
			"tenant_id":   tenantID,
		}, "unable to fetch cluster for user")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}
	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	openshiftConfig := openshift.NewConfig(c.defaultOpenshiftConfig, usr, clustr.User, clustr.Token, clustr.APIURL)
	for _, namespace := range namespaces {
		log.Info(ctx, map[string]interface{}{"tenant_id": tenantID, "namespace": namespace.Name}, "deleting namespace...")
		// delete the namespace in the cluster
		err = c.openshiftService.DeleteNamespace(ctx, openshiftConfig, namespace.Name)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":         err,
				"cluster_url": *usr.Cluster,
				"namespace":   namespace.Name,
				"tenant_id":   tenantID,
			}, "failed to delete namespace")
			return jsonapi.JSONErrorResponse(ctx, err)
		}
		// then delete the corresponding record in the DB
	}
	// finally, delete the tenant record (all NS were already deleted, but that's fine)
	err = c.tenantService.DeleteAll(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	log.Info(ctx, map[string]interface{}{"tenant_id": tenantID}, "tenant deleted")
	return ctx.NoContent()
}
