package controller

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	env "github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/user"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/fabric8-services/fabric8-wit/rest"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"
)

// TenantController implements the status resource.
type TenantController struct {
	*goa.Controller
	tenantService  tenant.Service
	resolveTenant  tenant.Resolve
	userService    user.Service
	resolveCluster cluster.Resolve
	config         *configuration.Data
}

// NewTenantController creates a status controller.
func NewTenantController(
	service *goa.Service,
	tenantService tenant.Service,
	userService user.Service,
	resolveTenant tenant.Resolve,
	resolveCluster cluster.Resolve,
	config *configuration.Data) *TenantController {

	return &TenantController{
		Controller:     service.NewController("TenantController"),
		tenantService:  tenantService,
		userService:    userService,
		resolveTenant:  resolveTenant,
		resolveCluster: resolveCluster,
		config:         config,
	}
}

// Setup runs the setup action.
func (c *TenantController) Setup(ctx *app.SetupTenantContext) error {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}
	ttoken := &TenantToken{token: userToken}
	exists := c.tenantService.Exists(ttoken.Subject())
	if exists {
		return ctx.Conflict()
	}

	// fetch the cluster the user belongs to
	user, err := c.userService.GetUser(ctx, ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	if user.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster")))
	}

	// fetch the users cluster token
	openshiftUsername, openshiftUserToken, err := c.resolveTenant(ctx, *user.Cluster, userToken.Raw)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch tenant token from auth")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not resolve user token"))
	}

	// fetch the cluster info
	cluster, err := c.resolveCluster(ctx, *user.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfig(c.config, user, cluster.User, cluster.Token, cluster.APIURL, Commit)
	tenant := &tenant.Tenant{
		ID:         ttoken.Subject(),
		Email:      ttoken.Email(),
		OSUsername: openshiftUsername,
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
			InitTenant(ctx, openshiftConfig.MasterURL, c.tenantService, t),
			openshiftUsername,
			openshiftUserToken)

		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":     err,
				"os_user": openshiftUsername,
			}, "unable initialize tenant")
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
	ttoken := &TenantToken{token: userToken}
	tenant, err := c.tenantService.GetTenant(ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", ttoken.Subject().String()))
	}

	// fetch the cluster the user belongs to
	user, err := c.userService.GetUser(ctx, ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	if user.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster")))
	}

	// fetch the users cluster token
	openshiftUsername, _, err := c.resolveTenant(ctx, *user.Cluster, userToken.Raw)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch tenant token from auth")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not resolve user token"))
	}

	cluster, err := c.resolveCluster(ctx, *user.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfig(c.config, user, cluster.User, cluster.Token, cluster.APIURL, Commit)

	// update tenant config
	tenant.OSUsername = openshiftUsername

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
			InitTenant(ctx, openshiftConfig.MasterURL, c.tenantService, t),
			openshiftUsername)

		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":     err,
				"os_user": openshiftUsername,
			}, "unable initialize tenant")
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
	ttoken := &TenantToken{token: userToken}

	// fetch the cluster the user belongs to
	user, err := c.userService.GetUser(ctx, ttoken.Subject())
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	// restrict deprovision from cluster to internal users only
	removeFromCluster := false
	if user.FeatureLevel != nil && *user.FeatureLevel == "internal" {
		removeFromCluster = ctx.Remove
	}

	// fetch the users cluster token
	openshiftUsername, _, err := c.resolveTenant(ctx, *user.Cluster, userToken.Raw)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch tenant token from auth")
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Could not resolve user token"))
	}

	cluster, err := c.resolveCluster(ctx, *user.Cluster)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *user.Cluster,
		}, "unable to fetch cluster")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	// create openshift config
	openshiftConfig := openshift.NewConfig(c.config, user, cluster.User, cluster.Token, cluster.APIURL, Commit)

	err = openshift.CleanTenant(ctx, openshiftConfig, openshiftUsername, removeFromCluster)
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
	token := goajwt.ContextJWT(ctx)
	if token == nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Missing JWT token"))
	}

	ttoken := &TenantToken{token: token}
	tenantID := ttoken.Subject()
	tenant, err := c.tenantService.GetTenant(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewNotFoundError("tenants", tenantID.String()))
	}

	namespaces, err := c.tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	result := &app.TenantSingle{Data: convertTenant(ctx, tenant, namespaces, c.resolveCluster)}
	return ctx.OK(result)
}

// InitTenant is a Callback that assumes a new tenant is being created
func InitTenant(ctx context.Context, masterURL string, service tenant.Service, currentTenant *tenant.Tenant) openshift.Callback {
	var maxResourceQuotaStatusCheck int32 = 50 // technically a global retry count across all ResourceQuota on all Tenant Namespaces
	var currentResourceQuotaStatusCheck int32  // default is 0
	return func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
		log.Info(ctx, map[string]interface{}{
			"status":      statusCode,
			"method":      method,
			"cluster_url": masterURL,
			"namespace":   env.GetNamespace(request),
			"name":        env.GetName(request),
			"kind":        env.GetKind(request),
			"request":     yamlString(request),
			"response":    yamlString(response),
		}, "resource requested")
		if statusCode == http.StatusConflict {
			if env.GetKind(request) == env.ValKindNamespace {
				return "", nil
			}
			if env.GetKind(request) == env.ValKindProjectRequest {
				return "", nil
			}
			if env.GetKind(request) == env.ValKindPersistenceVolumeClaim {
				return "", nil
			}
			if env.GetKind(request) == env.ValKindServiceAccount {
				return "", nil
			}
			return "DELETE", request
		} else if statusCode == http.StatusCreated {
			if env.GetKind(request) == env.ValKindProjectRequest {
				name := env.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   env.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name),
					MasterURL: masterURL,
				})

				// HACK to workaround osio applying some dsaas-user permissions async
				// Should loop on a Check if allowed type of call instead
				time.Sleep(time.Second * 5)

			} else if env.GetKind(request) == env.ValKindNamespace {
				name := env.GetName(request)
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     "created",
					Version:   env.GetLabelVersion(request),
					Type:      tenant.GetNamespaceType(name),
					MasterURL: masterURL,
				})
			} else if env.GetKind(request) == env.ValKindResourceQuota {
				// trigger a check status loop
				time.Sleep(time.Millisecond * 50)
				return "GET", response
			}
			return "", nil
		} else if statusCode == http.StatusOK {
			if method == "DELETE" {
				return "POST", request
			} else if method == "GET" {
				if env.GetKind(request) == env.ValKindResourceQuota {

					if env.HasValidStatus(response) || atomic.LoadInt32(&currentResourceQuotaStatusCheck) >= maxResourceQuotaStatusCheck {
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
			"namespace":   env.GetNamespace(request),
			"cluster_url": masterURL,
			"name":        env.GetName(request),
			"kind":        env.GetKind(request),
			"request":     yamlString(request),
			"response":    yamlString(response),
		}, "unhandled resource response")
		return "", nil
	}
}

func OpenshiftToken(openshiftConfig openshift.Config, token *jwt.Token) (string, error) {
	return "", nil
}

// TenantToken the token on the tenant
type TenantToken struct {
	token *jwt.Token
}

// Subject returns the value of the `sub` claim in the token
func (t TenantToken) Subject() uuid.UUID {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		id, err := uuid.FromString(claims["sub"].(string))
		if err != nil {
			return uuid.UUID{}
		}
		return id
	}
	return uuid.UUID{}
}

// Username returns the value of the `preferred_username` claim in the token
func (t TenantToken) Username() string {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		answer := claims["preferred_username"].(string)
		if len(answer) == 0 {
			answer = claims["username"].(string)
		}
		return answer
	}
	return ""
}

// Email returns the value of the `email` claim in the token
func (t TenantToken) Email() string {
	if claims, ok := t.token.Claims.(jwt.MapClaims); ok {
		return claims["email"].(string)
	}
	return ""
}

func convertTenant(ctx context.Context, tenant *tenant.Tenant, namespaces []*tenant.Namespace, resolveCluster cluster.Resolve) *app.Tenant {
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
