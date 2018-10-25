package controller

import (
	"context"
	"fmt"
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
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
)

// TenantController implements the tenant resource.
type TenantController struct {
	*goa.Controller
	config            *configuration.Data
	clusterService    cluster.Service
	authClientService *auth.Service
	tenantRepository  tenant.Service
}

// NewTenantController creates a tenant controller.
func NewTenantController(
	service *goa.Service,
	clusterService cluster.Service,
	authClientService *auth.Service,
	config *configuration.Data,
	tenantRepository tenant.Service) *TenantController {

	return &TenantController{
		Controller:        service.NewController("TenantsController"),
		config:            config,
		clusterService:    clusterService,
		authClientService: authClientService,
		tenantRepository:  tenantRepository,
	}
}

// Clean runs the clean action.
func (c *TenantController) Clean(ctx *app.CleanTenantContext) error {
	// gets user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// gets list of existing namespaces in DB
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// checks if the namespaces should be only cleaned or totally removed - restrict deprovision from cluster to internal users only
	removeFromCluster := false
	if user.UserData.FeatureLevel != nil && *user.UserData.FeatureLevel == "internal" {
		removeFromCluster = ctx.Remove
	}

	// creates openshift services
	openShiftService, err := c.newOpenShiftService(ctx, user)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform delete method on the list of existing namespaces
	err = openShiftService.WithDeleteMethod(namespaces, removeFromCluster).ApplyAll()
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	return ctx.NoContent()
}

// Setup runs the setup action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	// gets user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	var dbTenant *tenant.Tenant
	var namespaces []*tenant.Namespace
	// check if tenant already exists
	if c.tenantRepository.Exists(user.ID) {
		// if exists, then check existing namespace (if all of them are created or if any is missing)
		namespaces, err = c.tenantRepository.GetNamespaces(user.ID)
		if err != nil {
			return jsonapi.JSONErrorResponse(ctx, err)
		}
	} else {
		// if does not exist then create a new tenant
		dbTenant = &tenant.Tenant{
			ID:         user.ID,
			Email:      *user.UserData.Email,
			OSUsername: user.OpenShiftUsername,
		}
		err = c.tenantRepository.CreateTenant(dbTenant)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err": err,
			}, "unable to store tenant configuration")
			return jsonapi.JSONErrorResponse(ctx, err)
		}
	}

	// check if any environment type is missing - should be provisioned
	missing, _ := filterMissingAndExisting(namespaces)
	if len(missing) == 0 {
		return ctx.Conflict()
	}

	// create openshift service
	service, err := c.newOpenShiftService(ctx, user)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform post method on the list of missing environment types
	err = service.WithPostMethod().ApplyAll(missing...)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

// Show runs the show action.
func (c *TenantController) Show(ctx *app.ShowTenantContext) error {
	// get user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return err
	}

	// gets tenant from DB
	tenant, err := c.tenantRepository.GetTenant(user.ID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", user.ID.String()))
	}

	// gets tenant's namespaces
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	return ctx.OK(&app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.clusterService.GetCluster)})
}

// Update runs the update action.
func (c *TenantController) Update(ctx *app.UpdateTenantContext) error {
	// get user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// getting tenant from DB
	tenant, err := c.tenantRepository.GetTenant(user.ID)
	if err != nil {
		return errors.NewNotFoundError("tenant", *user.UserData.IdentityID)
	}

	// get tenant's namespaces
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// update tenant OS username
	tenant.OSUsername = user.OpenShiftUsername
	if err = c.tenantRepository.SaveTenant(tenant); err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to update tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to update tenant configuration: %v", err)))
	}

	// create openshift service
	openShiftService, err := c.newOpenShiftService(ctx, user)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform patch method on the list of exiting namespaces
	err = openShiftService.WithPatchMethod(namespaces).ApplyAll()
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

func (c *TenantController) newOpenShiftService(ctx context.Context, user *auth.User) (*openshift.ServiceBuilder, error) {
	clusterNsMapping, err := c.clusterService.GetUserClusterForType(ctx, user)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"tenant":      user.ID,
			"cluster_url": *user.UserData.Cluster,
		}, "unable to fetch cluster for tenant")
		return nil, jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	nsRepo := c.tenantRepository.NewTenantRepository(user.ID)

	envService := environment.NewServiceForUserData(user.UserData)

	serviceContext := openshift.NewServiceContext(ctx, c.config, clusterNsMapping, user.OpenShiftUsername, user.OpenShiftUserToken)
	return openshift.NewService(serviceContext, nsRepo, envService), nil
}

func filterMissingAndExisting(namespaces []*tenant.Namespace) (missing []environment.Type, existing []environment.Type) {
	exitingTypes := GetNamespaceByType(namespaces)

	missingNamespaces := make([]environment.Type, 0)
	existingNamespaces := make([]environment.Type, 0)
	for _, nsType := range environment.DefaultEnvTypes {
		_, exits := exitingTypes[nsType]
		if !exits {
			missingNamespaces = append(missingNamespaces, nsType)
		} else {
			existingNamespaces = append(existingNamespaces, nsType)
		}
	}
	return missingNamespaces, existingNamespaces
}

func GetNamespaceByType(namespaces []*tenant.Namespace) map[environment.Type]*tenant.Namespace {
	var nsTypes map[environment.Type]*tenant.Namespace
	for _, namespace := range namespaces {
		nsTypes[namespace.Type] = namespace
	}
	return nsTypes
}
