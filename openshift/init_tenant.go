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

func RawInitTenant(ctx context.Context, config Config, callback Callback, openshiftUsername, username, usertoken string) error {
	templs, err := LoadProcessedTemplates(ctx, config, openshiftUsername, username)
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
	for key, val := range mapped {
		namespaceType := tenant.GetNamespaceType(key, username)
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
				_, err = apply(
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

func RawUpdateTenant(ctx context.Context, config Config, callback Callback, osUsername, username string) error {
	templs, err := LoadProcessedTemplates(ctx, config, osUsername, username)
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
	return nil
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
