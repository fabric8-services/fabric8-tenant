package controller

import (
	"reflect"

	"fmt"
	commonauth "github.com/fabric8-services/fabric8-common/auth"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/goadesign/goa"
	errs "github.com/pkg/errors"
)

var SERVICE_ACCOUNTS = []string{"fabric8-jenkins-idler", "rh-che"}

// TenantsController implements the tenants resource.
type TenantsController struct {
	*goa.Controller
	tenantService     tenant.Service
	clusterService    cluster.Service
	authClientService auth.Service
	config            *configuration.Data
}

// NewTenantsController creates a tenants controller.
func NewTenantsController(
	service *goa.Service,
	tenantService tenant.Service,
	clusterService cluster.Service,
	authClientService auth.Service,
	config *configuration.Data) *TenantsController {
	return &TenantsController{
		Controller:        service.NewController("TenantsController"),
		tenantService:     tenantService,
		clusterService:    clusterService,
		authClientService: authClientService,
		config:            config,
	}
}

// Show runs the show action.
func (c *TenantsController) Show(ctx *app.ShowTenantsContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, SERVICE_ACCOUNTS...) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	// get tenant from DB
	tenantID := ctx.TenantID
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"tenant_id": tenantID, "error_type": reflect.TypeOf(err)}, "error while looking-up tenant record")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// gets tenant's namespaces
	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": tenantID,
		}, "retrieval of existing namespaces from DB failed")
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

	// find tenant in DB
	tenant, err := c.tenantService.LookupTenantByClusterAndNamespace(ctx.MasterURL, ctx.Namespace)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":        err,
			"clusterURL": ctx.MasterURL,
			"namespace":  ctx.Namespace,
		}, "lookup for a tenant entity failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// gets tenant's namespaces
	namespaces, err := c.tenantService.GetNamespaces(tenant.ID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": tenant.ID,
		}, "retrieval of existing namespaces from DB failed")
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

	// find tenant in DB
	tenantID := ctx.TenantID
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": tenant,
		}, "retrieval of tenant entity from DB failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	nsBaseName := tenant.NsBaseName
	if nsBaseName == "" {
		nsBaseName = environment.RetrieveUserName(tenant.OSUsername)
	}

	// gets tenant's namespaces
	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": tenantID,
		}, "retrieval of existing namespaces from DB failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// map target cluster for every environment type
	clusterMapping, err := GetClusterMapping(ctx, c.clusterService, namespaces)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift service
	// we don't need token as DELETE uses cluster token
	context := openshift.NewServiceContext(ctx, c.config, clusterMapping, tenant.OSUsername, nsBaseName, openshift.TokenResolver())
	service := openshift.NewService(context, c.tenantService.NewTenantRepository(tenantID), environment.NewService())

	// perform delete method on the list of existing namespaces
	err = service.WithDeleteMethod(namespaces, true).ApplyAll(environment.DefaultEnvTypes)
	if err != nil {
		namespaces, getErr := c.tenantService.GetNamespaces(tenantID)
		if getErr != nil {
			log.Error(ctx, map[string]interface{}{
				"err":      err,
				"tenantID": tenantID,
			}, "retrieval of existing namespaces from DB after the removal attempt failed")
			return jsonapi.JSONErrorResponse(ctx, errs.Wrap(err, err.Error()))
		}
		params := namespacesToParams(namespaces)
		params["err"] = err
		log.Error(ctx, params, "deletion of namespaces failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// the tenant should have been deleted - check it
	if c.tenantService.Exists(tenantID) {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": tenantID,
		}, "deletion of the tenant failed - it still exists")
		return jsonapi.JSONErrorResponse(ctx, fmt.Errorf("unable to delete tenant %s", tenantID))
	}

	log.Info(ctx, map[string]interface{}{"tenant_id": tenantID}, "tenant deleted")
	return ctx.NoContent()
}

func namespacesToParams(namespaces []*tenant.Namespace) map[string]interface{} {
	params := make(map[string]interface{})
	for idx, ns := range namespaces {
		key := fmt.Sprintf("namespace#%d", idx)
		params[key] = fmt.Sprintf("%+v", *ns)
	}
	return params
}
