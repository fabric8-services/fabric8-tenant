package openshift

import (
	"context"

	"sync"

	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"gopkg.in/yaml.v2"
)

const (
	varProjectName           = "PROJECT_NAME"
	varProjectTemplateName   = "PROJECT_TEMPLATE_NAME"
	varProjectDisplayName    = "PROJECT_DISPLAYNAME"
	varProjectDescription    = "PROJECT_DESCRIPTION"
	varProjectUser           = "PROJECT_USER"
	varProjectRequestingUser = "PROJECT_REQUESTING_USER"
	varProjectAdminUser      = "PROJECT_ADMIN_USER"
	varProjectNamespace      = "PROJECT_NAMESPACE"
	varKeycloakURL           = "KEYCLOAK_URL"
)

func RawInitTenant(ctx context.Context, config Config, callback Callback, openshiftUsername, usertoken string, templateVars map[string]string) error {
	templs, err := LoadProcessedTemplates(ctx, config, openshiftUsername, templateVars)
	if err != nil {
		return err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return err
	}
	masterOpts := ApplyOptions{Config: config, Callback: callback}
	userOpts := ApplyOptions{Config: config.WithToken(usertoken), Callback: callback}
	var wg sync.WaitGroup
	wg.Add(len(mapped))
	username := CreateName(openshiftUsername)
	for key, val := range mapped {
		namespaceType := tenant.GetNamespaceType(key, username)
		if namespaceType == tenant.TypeUser {
			go func(namespace string, objects []map[interface{}]interface{}, opts, userOpts ApplyOptions) {
				defer wg.Done()
				err := ApplyProcessed(Filter(objects, IsOfKind(ValKindProjectRequest, ValKindNamespace)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, ProjectRequest")
				}
				err = ApplyProcessed(Filter(objects, IsOfKind(ValKindRoleBindingRestriction)), opts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, RoleBindingRestrictions")
				}
				err = ApplyProcessed(Filter(objects, IsNotOfKind(ValKindProjectRequest, ValKindNamespace, ValKindRoleBindingRestriction)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, Other")
				}
				_, err = apply(
					CreateAdminRoleBinding(namespace),
					"DELETE",
					opts.WithCallback(
						func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
							log.Info(ctx, map[string]interface{}{
								"status":    statusCode,
								"method":    method,
								"namespace": GetNamespace(request),
								"name":      GetName(request),
								"kind":      GetKind(request),
							}, "resource requested")
							return "", nil
						},
					),
				)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error unable to delete Admin role from project")
				}
			}(key, val, masterOpts, userOpts)
		} else {
			go func(namespace string, objects []map[interface{}]interface{}, opts ApplyOptions) {
				defer wg.Done()
				err := ApplyProcessed(objects, opts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error dsaas project")
				}
			}(key, val, masterOpts)
		}
	}
	wg.Wait()
	return nil
}

func RawUpdateTenant(ctx context.Context, config Config, callback Callback, username string, templateVars map[string]string) error {
	templs, err := LoadProcessedTemplates(ctx, config, username, templateVars)
	if err != nil {
		return err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return err
	}
	masterOpts := ApplyOptions{Config: config, Callback: callback}
	var wg sync.WaitGroup
	wg.Add(len(mapped))
	for key, val := range mapped {
		go func(namespace string, objects []map[interface{}]interface{}, opts ApplyOptions) {
			defer wg.Done()
			output, err := executeProccessedNamespaceCMD(
				listToTemplate(
					//RemoveReplicas(
					Filter(
						objects,
						IsNotOfKind(ValKindProjectRequest),
					),
					//),
				),
				opts.WithNamespace(namespace),
			)
			if err != nil {
				sentry.LogError(ctx, map[string]interface{}{
					"output":    output,
					"namespace": namespace,
				}, err, "ns update failed")
				return
			}
			log.Info(ctx, map[string]interface{}{
				"output":    output,
				"namespace": namespace,
			}, "applied")
		}(key, val, masterOpts)
	}
	wg.Wait()
	return nil
}

func listToTemplate(objects []map[interface{}]interface{}) string {
	template := map[interface{}]interface{}{
		"apiVersion": "v1",
		"kind":       "Template",
		"objects":    objects,
	}

	b, _ := yaml.Marshal(template)
	return string(b)
}
