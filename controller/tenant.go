package controller

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	env "github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	errs "github.com/pkg/errors"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	tenantService     tenant.Service
	clusterService    cluster.Service
	authClientService auth.Service
	config            *configuration.Data
}

// NewTenantController creates a status controller.
func NewTenantController(
	service *goa.Service,
	tenantService tenant.Service,
	clusterService cluster.Service,
	authClientService auth.Service,
	config *configuration.Data) *TenantController {

	return &TenantController{
		Controller:        service.NewController("TenantController"),
		tenantService:     tenantService,
		clusterService:    clusterService,
		authClientService: authClientService,
		config:            config,
	}
}

// Setup runs the setup action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &auth.TenantToken{Token: userToken}
	exists := c.tenantService.Exists(ttoken.Subject())
	if exists {
		return ctx.Conflict()
	}

	// fetch the cluster the user belongs to
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	if user.UserData.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster")))
	}

	// fetch the cluster info
	cluster, err := c.clusterService.GetCluster(ctx, *user.UserData.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.UserData.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	nsBaseName, err := tenant.ConstructNsBaseName(c.tenantService, env.RetrieveUserName(user.OpenShiftUsername))
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"os_username": user.OpenShiftUsername,
		}, "unable to construct namespace base name")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfigForUser(c.config, user.UserData, cluster.User, cluster.Token, cluster.APIURL)
	tenant := &tenant.Tenant{
		ID:         ttoken.Subject(),
		Email:      ttoken.Email(),
		OSUsername: user.OpenShiftUsername,
		NsBaseName: nsBaseName,
	}
	err = c.tenantService.CreateTenant(tenant)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to store tenant configuration")
		return ctx.Conflict()
	}

	go func() {
		ctx := ctx
		t := tenant
		_, err = openshift.RawInitTenant(ctx, openshiftConfig, t, user.OpenShiftUserToken, c.tenantService, true)

		if err != nil {
			sentry.LogError(ctx, map[string]interface{}{
				"os_user": user.OpenShiftUsername,
			}, err, "unable initialize tenant")
		}
	}()

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

// Update runs the setup action.
func (c *TenantController) Update(ctx *app.UpdateTenantContext) error {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &auth.TenantToken{Token: userToken}
	tenant, err := c.tenantService.GetTenant(ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", ttoken.Subject().String()))
	}

	// fetch the cluster the user belongs to
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	if user.UserData.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster")))
	}

	cluster, err := c.clusterService.GetCluster(ctx, *user.UserData.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.UserData.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfigForUser(c.config, user.UserData, cluster.User, cluster.Token, cluster.APIURL)

	// update tenant config
	tenant.OSUsername = user.OpenShiftUsername
	if tenant.NsBaseName == "" {
		tenant.NsBaseName = env.RetrieveUserName(user.OpenShiftUsername)
	}
	if err = c.tenantService.SaveTenant(tenant); err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to update tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to update tenant configuration: %v", err)))
	}

	err = openshift.UpdateTenant(&TenantUpdater{}, ctx, c.tenantService, openshiftConfig, tenant, user.OpenShiftUserToken, true, env.DefaultEnvTypes...)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":       err,
			"os_user":   tenant.OSUsername,
			"tenant_id": tenant.ID,
		}, "unable update tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, errs.Wrap(err, "unable update tenant")))
	}

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
}

type TenantUpdater struct {
}

func (u TenantUpdater) Update(ctx context.Context, tenantService tenant.Service, openshiftConfig openshift.Config, t *tenant.Tenant,
	envTypes []env.Type, usertoken string, allowSelfHealing bool) (map[env.Type]string, error) {

	return openshift.RawUpdateTenant(ctx, openshiftConfig, t, envTypes, usertoken, tenantService, allowSelfHealing)
}

// Clean runs the setup action for the tenant namespaces.
func (c *TenantController) Clean(ctx *app.CleanTenantContext) error {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &auth.TenantToken{Token: userToken}

	// fetch the cluster the user belongs to
	user, err := c.authClientService.GetUser(ctx)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	tenant, err := c.tenantService.GetTenant(ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", ttoken.Subject().String()))
	}

	// restrict deprovision from cluster to internal users only
	removeFromCluster := false
	if user.UserData.FeatureLevel != nil && *user.UserData.FeatureLevel == "internal" {
		removeFromCluster = ctx.Remove
	}

	cluster, err := c.clusterService.GetCluster(ctx, *user.UserData.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.UserData.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfigForUser(c.config, user.UserData, cluster.User, cluster.Token, cluster.APIURL)

	nsBaseName := tenant.NsBaseName
	if nsBaseName == "" {
		nsBaseName = env.RetrieveUserName(user.OpenShiftUsername)
	}

	err = openshift.CleanTenant(ctx, openshiftConfig, user.OpenShiftUsername, nsBaseName, removeFromCluster)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	if removeFromCluster {
		err = c.tenantService.DeleteNamespaces(ttoken.Subject())
		if err != nil {
			return jsonapi.JSONErrorResponse(ctx, err)
		}
		err = c.tenantService.DeleteTenant(ttoken.Subject())
		if err != nil {
			return jsonapi.JSONErrorResponse(ctx, err)
		}
	}
	return ctx.NoContent()
}

// Show runs the setup action.
func (c *TenantController) Show(ctx *app.ShowTenantContext) error {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}

	ttoken := &auth.TenantToken{Token: userToken}
	tenantID := ttoken.Subject()
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", tenantID.String()))
	}

	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	result := &app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.clusterService.GetCluster)}
	return ctx.OK(result)
}

func convertTenant(ctx context.Context, tenant *tenant.Tenant, namespaces []*tenant.Namespace, resolveCluster cluster.GetCluster) *app.Tenant {
	result := app.Tenant{
		ID:   &tenant.ID,
		Type: "tenants",
		Attributes: &app.TenantAttributes{
			CreatedAt:  &tenant.CreatedAt,
			Email:      &tenant.Email,
			Profile:    &tenant.Profile,
			Namespaces: []*app.NamespaceAttributes{},
		},
	}
	for _, ns := range namespaces {
		c, err := resolveCluster(ctx, ns.MasterURL)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":         err,
				"cluster_url": ns.MasterURL,
			}, "unable to resolve cluster")
			c = cluster.Cluster{}
		}
		tenantType := string(ns.Type)
		namespaceState := ns.State.String()
		result.Attributes.Namespaces = append(
			result.Attributes.Namespaces,
			&app.NamespaceAttributes{
				CreatedAt:                &ns.CreatedAt,
				UpdatedAt:                &ns.UpdatedAt,
				ClusterURL:               &ns.MasterURL,
				ClusterAppDomain:         &c.AppDNS,
				ClusterConsoleURL:        &c.ConsoleURL,
				ClusterMetricsURL:        &c.MetricsURL,
				ClusterLoggingURL:        &c.LoggingURL,
				Name:                     &ns.Name,
				Type:                     &tenantType,
				Version:                  &ns.Version,
				State:                    &namespaceState,
				ClusterCapacityExhausted: &c.CapacityExhausted,
			})
	}
	return &result
}
