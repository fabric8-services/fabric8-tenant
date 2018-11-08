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
	errs "github.com/pkg/errors"
	"github.com/satori/go.uuid"
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
	tenantRepository tenant.Service,
	clusterService cluster.Service,
	authClientService *auth.Service,
	config *configuration.Data) *TenantController {

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
		log.Error(ctx, map[string]interface{}{"err": err}, "creation of the user failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// gets list of existing namespaces in DB
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of existing namespaces from DB failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// get tenant entity
	dbTenant, err := c.getExistingTenant(ctx, user.ID, user.OpenShiftUsername)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of tenant entity from DB failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenant", user.ID.String()))
	}

	// checks if the namespaces should be only cleaned or totally removed - restrict deprovision from cluster to internal users only
	removeFromCluster := false
	if user.UserData.FeatureLevel != nil && *user.UserData.FeatureLevel == "internal" {
		removeFromCluster = ctx.Remove
	}

	// creates openshift services
	openShiftService, err := c.newOpenShiftService(ctx, user, dbTenant.NsBaseName)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "unable to create OpenShift service")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform delete method on the list of existing namespaces
	err = openShiftService.WithDeleteMethod(namespaces, removeFromCluster).ApplyAll()
	if err != nil {
		namespaces, getErr := c.tenantRepository.GetNamespaces(dbTenant.ID)
		if getErr != nil {
			log.Error(ctx, map[string]interface{}{
				"err":      err,
				"tenantID": user.ID,
			}, "retrieval of existing namespaces from DB after the removal attempt failed")
			return jsonapi.JSONErrorResponse(ctx, errs.Wrap(err, err.Error()))
		}
		params := namespacesToParams(namespaces)
		params["err"] = err
		log.Error(ctx, params, "deletion of namespaces failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	return ctx.NoContent()
}

// Setup runs the setup action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	// gets user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"err": err}, "creation of the user failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	var dbTenant *tenant.Tenant
	var namespaces []*tenant.Namespace
	// check if tenant already exists
	if c.tenantRepository.Exists(user.ID) {
		// if exists, then check existing namespace (if all of them are created or if any is missing)
		namespaces, err = c.tenantRepository.GetNamespaces(user.ID)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":      err,
				"tenantID": user.ID,
			}, "retrieval of existing namespaces from DB failed")
			return jsonapi.JSONErrorResponse(ctx, err)
		}
		dbTenant, err = c.getExistingTenant(ctx, user.ID, user.OpenShiftUsername)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":      err,
				"tenantID": user.ID,
			}, "retrieval of tenant entity from DB failed")
			return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenant", user.ID.String()))
		}
	} else {
		nsBaseName, err := tenant.ConstructNsBaseName(c.tenantRepository, environment.RetrieveUserName(user.OpenShiftUsername))
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":         err,
				"os_username": user.OpenShiftUsername,
			}, "unable to construct namespace base name")
			return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
		}
		// if does not exist then create a new tenant
		dbTenant = &tenant.Tenant{
			ID:         user.ID,
			Email:      *user.UserData.Email,
			OSUsername: user.OpenShiftUsername,
			NsBaseName: nsBaseName,
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
	service, err := c.newOpenShiftService(ctx, user, dbTenant.NsBaseName)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "unable to create OpenShift service")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform post method on the list of missing environment types
	err = service.WithPostMethod().ApplyAll(missing...)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":             err,
			"tenantID":        user.ID,
			"envTypeToCreate": missing,
		}, "creation of namespaces failed")
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
		log.Error(ctx, map[string]interface{}{"err": err}, "creation of the user failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// gets tenant from DB
	tenant, err := c.getExistingTenant(ctx, user.ID, user.OpenShiftUsername)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of tenant entity from DB failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", user.ID.String()))
	}

	// gets tenant's namespaces
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of existing namespaces from DB failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	return ctx.OK(&app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.clusterService.GetCluster)})
}

// Update runs the update action.
func (c *TenantController) Update(ctx *app.UpdateTenantContext) error {
	// get user info
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		log.Error(ctx, map[string]interface{}{"err": err}, "creation of the user failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// getting tenant from DB
	dbTenant, err := c.getExistingTenant(ctx, user.ID, user.OpenShiftUsername)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of tenant entity from DB failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenant", user.ID.String()))
	}

	// get tenant's namespaces
	namespaces, err := c.tenantRepository.GetNamespaces(user.ID)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "retrieval of existing namespaces from DB failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// update tenant OS username
	dbTenant.OSUsername = user.OpenShiftUsername
	if err = c.tenantRepository.SaveTenant(dbTenant); err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to update tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to update tenant configuration: %v", err)))
	}

	// create openshift service
	openShiftService, err := c.newOpenShiftService(ctx, user, dbTenant.NsBaseName)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":      err,
			"tenantID": user.ID,
		}, "unable to create OpenShift service")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// perform patch method on the list of exiting namespaces
	err = openShiftService.WithPatchMethod(namespaces).ApplyAll()
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":                err,
			"tenantID":           user.ID,
			"namespacesToUpdate": listNames(namespaces),
		}, "update of namespaces failed")
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

func listNames(namespaces []*tenant.Namespace) []string {
	var names []string
	for _, ns := range namespaces {
		names = append(names, ns.Name)
	}
	return names
}

func (c *TenantController) getExistingTenant(ctx context.Context, id uuid.UUID, osUsername string) (*tenant.Tenant, error) {
	dbTenant, err := c.tenantRepository.GetTenant(id)
	if err != nil {
		return nil, err
	}
	if dbTenant.NsBaseName == "" {
		dbTenant.NsBaseName = environment.RetrieveUserName(osUsername)
	}
	return dbTenant, nil
}

func (c *TenantController) newOpenShiftService(ctx context.Context, user *auth.User, nsBaseName string) (*openshift.ServiceBuilder, error) {
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

	serviceContext := openshift.NewServiceContext(
		ctx, c.config, clusterNsMapping, user.OpenShiftUsername, user.OpenShiftUserToken, nsBaseName)
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
	nsTypes := map[environment.Type]*tenant.Namespace{}
	for _, namespace := range namespaces {
		nsTypes[namespace.Type] = namespace
	}
	return nsTypes
}
