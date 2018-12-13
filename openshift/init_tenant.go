package openshift

import (
	"context"

	"sync"

	"github.com/fabric8-services/fabric8-common/log"
	env "github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"gopkg.in/yaml.v2"
)

func RawInitTenant(ctx context.Context, config Config, callback Callback, openshiftUsername, nsBaseName, usertoken string) error {
	templs, versionMapping, err := LoadProcessedTemplates(ctx, config, openshiftUsername, nsBaseName, env.DefaultEnvTypes)
	if err != nil {
		return err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return err
	}

	callbackWithVersionMapping := func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
		return callback(statusCode, method, request, response, versionMapping)
	}
	masterOpts := ApplyOptions{Config: config, Callback: callbackWithVersionMapping}
	userOpts := ApplyOptions{Config: config.WithToken(usertoken), Callback: callbackWithVersionMapping}

	var wg sync.WaitGroup
	wg.Add(len(mapped))
	for key, val := range mapped {
		namespaceType := tenant.GetNamespaceType(key, nsBaseName)

		if namespaceType == tenant.TypeUser {
			go func(namespace string, objects env.Objects, opts, userOpts ApplyOptions) {
				defer wg.Done()
				err := ApplyProcessed(Filter(objects, IsOfKind(env.ValKindProjectRequest, env.ValKindNamespace)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, ProjectRequest")
				}
				err = ApplyProcessed(Filter(objects, IsOfKind(env.ValKindRoleBindingRestriction)), opts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, RoleBindingRestrictions")
				}
				err = ApplyProcessed(Filter(objects, IsNotOfKind(env.ValKindProjectRequest, env.ValKindNamespace, env.ValKindRoleBindingRestriction)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, Other")
				}
				_, err = Apply(
					CreateAdminRoleBinding(namespace),
					"DELETE",
					opts.WithCallback(
						func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
							log.Info(ctx, map[string]interface{}{
								"status":    statusCode,
								"method":    method,
								"namespace": env.GetNamespace(request),
								"name":      env.GetName(request),
								"kind":      env.GetKind(request),
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
			go func(namespace string, objects env.Objects, opts ApplyOptions) {
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

func RawUpdateTenant(ctx context.Context, config Config, callback Callback, osUsername, nsBaseName string, envTypes []string) (map[string]string, error) {
	templs, versionMapping, err := LoadProcessedTemplates(ctx, config, osUsername, nsBaseName, envTypes)
	if err != nil {
		return versionMapping, err
	}

	callbackWithVersionMapping := func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
		return callback(statusCode, method, request, response, versionMapping)
	}
	masterOpts := ApplyOptions{Config: config, Callback: callbackWithVersionMapping}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return versionMapping, err
	}
	var wg sync.WaitGroup
	wg.Add(len(mapped))
	for key, val := range mapped {

		go func(namespace string, objects env.Objects, opts ApplyOptions) {
			defer wg.Done()
			output, err := executeProccessedNamespaceCMD(
				listToTemplate(
					Filter(
						objects,
						IsNotOfKind(env.ValKindProjectRequest),
					),
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
	return versionMapping, nil
}

func listToTemplate(objects env.Objects) string {
	template := env.Object{
		"apiVersion": "v1",
		"kind":       "Template",
		"objects":    objects,
	}

	b, _ := yaml.Marshal(template)
	return string(b)
}
