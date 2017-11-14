package controller

import (
	"strings"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"

	"github.com/goadesign/goa"
)

// ListNamespacesController implements the tenantAll resource.
type ListNamespacesController struct {
	*goa.Controller
	tenantService   tenant.Service
	keycloakConfig  keycloak.Config
	openshiftConfig openshift.Config
	templateVars    map[string]string
	usersURL        string
	aclListTenants  string
}

func CheckAccess(allowedusername, env string) error {
	allowed := false
	for _, username := range strings.Split(strings.TrimSpace(env), ",") {
		if username == allowedusername {
			allowed = true
		}
	}

	if !allowed {
		return errors.NewUnauthorizedError("you are not allowed to access to private apis request")
	} else {
		return nil
	}
}

func NewListNamespacesController(service *goa.Service, tenantService tenant.Service, keycloakConfig keycloak.Config, openshiftConfig openshift.Config, templateVars map[string]string, aclListTenants string) *ListNamespacesController {
	return &ListNamespacesController{
		Controller:      service.NewController("ListNamespacesController"),
		tenantService:   tenantService,
		keycloakConfig:  keycloakConfig,
		openshiftConfig: openshiftConfig,
		templateVars:    templateVars,
		aclListTenants:  aclListTenants,
	}
}

// Show runs the show action.
func (c *ListNamespacesController) Show(ctx *app.ShowListNamespacesContext) error {
	if len(ctx.Request.Header["Authorization"]) == 0 {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Expecting Authorization Header"))
	}
	aheader := strings.Split(ctx.Request.Header["Authorization"][0], " ")
	if len(aheader) != 2 || strings.ToUpper(aheader[0]) != "BEARER" {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Invalid Authorization Header"))
	}
	osconfig := c.openshiftConfig
	osconfig.Token = aheader[1]
	osusername, err := openshift.WhoAmI(osconfig)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Cannot check OpenShift identity"))
	}

	err = CheckAccess(osusername, c.aclListTenants)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError(
			"You don't have access to namespaces listing"))
	}

	namespaces, err := c.tenantService.GetAllNamespaces()
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}

	response := app.NamespacesAll{
		Namespaces: []*app.NamespaceAttributes{},
	}

	for _, ns := range namespaces {
		tenantType := string(ns.Type)
		response.Namespaces = append(
			response.Namespaces,
			&app.NamespaceAttributes{
				CreatedAt:  &ns.CreatedAt,
				UpdatedAt:  &ns.UpdatedAt,
				ClusterURL: &ns.MasterURL,
				Name:       &ns.Name,
				Type:       &tenantType,
				Version:    &ns.Version,
				State:      &ns.State,
			})
	}

	return ctx.OK(&response)
}
