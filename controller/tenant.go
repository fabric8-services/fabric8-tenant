package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"gopkg.in/yaml.v2"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	tenantService          tenant.Service
	clusterService         cluster.Service
	authClientService      *auth.Service
	defaultOpenshiftConfig openshift.Config
	templateVars           map[string]string
}

// NewTenantController creates a status controller.
func NewTenantController(
	service *goa.Service,
	tenantService tenant.Service,
	clusterService cluster.Service,
	authClientService *auth.Service,
	defaultOpenshiftConfig openshift.Config,
	templateVars map[string]string) *TenantController {

	return &TenantController{
		Controller:             service.NewController("TenantController"),
		tenantService:          tenantService,
		clusterService:         clusterService,
		authClientService:      authClientService,
		defaultOpenshiftConfig: defaultOpenshiftConfig,
		templateVars:           templateVars,
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

	// create openshift config
	openshiftConfig := openshift.NewConfig(c.defaultOpenshiftConfig, user.UserData, cluster.User, cluster.Token, cluster.APIURL)
	tenant := &tenant.Tenant{
		ID:         ttoken.Subject(),
		Email:      ttoken.Email(),
		OSUsername: user.OpenshiftUsername,
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
		err = openshift.RawInitTenant(
			ctx,
			openshiftConfig,
			InitTenant(ctx, openshiftConfig.MasterURL, c.tenantService, t, user.OpenshiftUsername),
			user.OpenshiftUsername,
			user.OpenshiftUserToken,
			c.templateVars)

		if err != nil {
			sentry.LogError(ctx, map[string]interface{}{
				"os_user": user.OpenshiftUsername,
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
	openshiftConfig := openshift.NewConfig(c.defaultOpenshiftConfig, user.UserData, cluster.User, cluster.Token, cluster.APIURL)

	// update tenant config
	tenant.OSUsername = user.OpenshiftUsername

	if err = c.tenantService.SaveTenant(tenant); err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "unable to update tenant configuration")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to update tenant configuration: %v", err)))
	}

	go func() {
		ctx := ctx
		t := tenant
		err = openshift.RawUpdateTenant(
			ctx,
			openshiftConfig,
			InitTenant(ctx, openshiftConfig.MasterURL, c.tenantService, t, user.OpenshiftUsername),
			user.OpenshiftUsername,
			c.templateVars)

		if err != nil {
			sentry.LogError(ctx, map[string]interface{}{
				"os_user": user.OpenshiftUsername,
			}, err, "unable initialize tenant")
		}
	}()

	ctx.ResponseData.Header().Set("Location", rest.AbsoluteURL(ctx.RequestData.Request, app.TenantHref()))
	return ctx.Accepted()
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
	openshiftConfig := openshift.NewConfig(c.defaultOpenshiftConfig, user.UserData, cluster.User, cluster.Token, cluster.APIURL)

	err = openshift.CleanTenant(ctx, openshiftConfig, user.OpenshiftUsername, c.templateVars, removeFromCluster)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	if removeFromCluster {
		err = c.tenantService.DeleteAll(ttoken.Subject())
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

// InitTenant is a Callback that assumes a new tenant is being created
func InitTenant(ctx context.Context, masterURL string, service tenant.Service, currentTenant *tenant.Tenant, openshiftUsername string) openshift.Callback {
	var maxResourceQuotaStatusCheck int32 = 50 // technically a global retry count across all ResourceQuota on all Tenant Namespaces
	var currentResourceQuotaStatusCheck int32  // default is 0
	username := openshift.CreateName(openshiftUsername)
	return func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
		log.Info(ctx, map[string]interface{}{
			"status":      statusCode,
			"method":      method,
			"cluster_url": masterURL,
			"namespace":   openshift.GetNamespace(request),
			"name":        openshift.GetName(request),
			"kind":        openshift.GetKind(request),
			"request":     yamlString(request),
			"response":    yamlString(response),
		}, "resource requested")
		if statusCode == http.StatusConflict {
			if openshift.GetKind(request) == openshift.ValKindNamespace {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindProjectRequest {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindPersistenceVolumeClaim {
				return "", nil
			}
			if openshift.GetKind(request) == openshift.ValKindServiceAccount {
				return "", nil
			}
			return "DELETE", request
		} else if statusCode == http.StatusCreated {
			if openshift.GetKind(request) == openshift.ValKindProjectRequest {
				name := openshift.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   openshift.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name, username),
					MasterURL: masterURL,
				})

				// HACK to workaround osio applying some dsaas-user permissions async
				// Should loop on a Check if allowed type of call instead
				time.Sleep(time.Second * 5)

			} else if openshift.GetKind(request) == openshift.ValKindNamespace {
				name := openshift.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   openshift.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name, username),
					MasterURL: masterURL,
				})
			} else if openshift.GetKind(request) == openshift.ValKindResourceQuota {
				// trigger a check status loop
				time.Sleep(time.Millisecond * 50)
				return "GET", response
			}
			return "", nil
		} else if statusCode == http.StatusOK {
			if method == "DELETE" {
				return "POST", request
			} else if method == "GET" {
				if openshift.GetKind(request) == openshift.ValKindResourceQuota {

					if openshift.HasValidStatus(response) || atomic.LoadInt32(&currentResourceQuotaStatusCheck) >= maxResourceQuotaStatusCheck {
						return "", nil
					}
					atomic.AddInt32(&currentResourceQuotaStatusCheck, 1)
					time.Sleep(time.Millisecond * 50)
					return "GET", response
				}
			}
			return "", nil
		}
		log.Info(ctx, map[string]interface{}{
			"status":      statusCode,
			"method":      method,
			"namespace":   openshift.GetNamespace(request),
			"cluster_url": masterURL,
			"name":        openshift.GetName(request),
			"kind":        openshift.GetKind(request),
			"request":     yamlString(request),
			"response":    yamlString(response),
		}, "unhandled resource response")
		return "", nil
	}
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
		result.Attributes.Namespaces = append(
			result.Attributes.Namespaces,
			&app.NamespaceAttributes{
				CreatedAt:         &ns.CreatedAt,
				UpdatedAt:         &ns.UpdatedAt,
				ClusterURL:        &ns.MasterURL,
				ClusterAppDomain:  &c.AppDNS,
				ClusterConsoleURL: &c.ConsoleURL,
				ClusterMetricsURL: &c.MetricsURL,
				ClusterLoggingURL: &c.LoggingURL,
				Name:              &ns.Name,
				Type:              &tenantType,
				Version:           &ns.Version,
				State:             &ns.State,
				ClusterCapacityExhausted: &c.CapacityExhausted,
			})
	}
	return &result
}

func yamlString(data map[interface{}]interface{}) string {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Could not marshal yaml %v", data)
	}
	return string(b)
}
