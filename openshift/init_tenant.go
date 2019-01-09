package openshift

import (
	"context"

	"sync"

	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	env "github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"net/http"
	"sync/atomic"
	"time"
)

func RawInitTenant(ctx context.Context, config Config, tnnt *tenant.Tenant, usertoken string, repo tenant.Service, allowSelfHealing bool) (map[env.Type]string, error) {

	templs, versionMapping, err := LoadProcessedTemplates(ctx, config, tnnt.OSUsername, tnnt.NsBaseName, env.DefaultEnvTypes)
	if err != nil {
		return versionMapping, err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return versionMapping, err
	}

	callbackWithVersionMapping := func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}, error) {
		return InitTenant(ctx, config.MasterURL, repo, tnnt)(statusCode, method, request, response, versionMapping)
	}
	masterOpts := ApplyOptions{Config: config, Callback: callbackWithVersionMapping}
	userOpts := ApplyOptions{Config: config.WithToken(usertoken), Callback: callbackWithVersionMapping}

	var wg sync.WaitGroup
	wg.Add(len(mapped))
	errorChan := make(chan error, len(mapped))
	for key, val := range mapped {
		namespaceType := tenant.GetNamespaceType(key, tnnt.NsBaseName)

		if namespaceType == env.TypeUser {
			go func(namespace string, objects env.Objects, opts, userOpts ApplyOptions) {
				defer wg.Done()
				err := ApplyProcessed(Filter(objects, IsOfKind(env.ValKindProjectRequest, env.ValKindNamespace)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, ProjectRequest")
					errorChan <- err
					return
				}
				err = ApplyProcessed(Filter(objects, IsOfKind(env.ValKindRoleBindingRestriction)), opts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, RoleBindingRestrictions")
					errorChan <- err
				}
				err = ApplyProcessed(Filter(objects, IsNotOfKind(env.ValKindProjectRequest, env.ValKindNamespace, env.ValKindRoleBindingRestriction)), userOpts)
				if err != nil {
					sentry.LogError(ctx, map[string]interface{}{
						"namespace": namespace,
					}, err, "error init user project, Other")
					errorChan <- err
					return
				}
				_, err = Apply(
					CreateAdminRoleBinding(namespace),
					"DELETE",
					opts.WithCallback(
						func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}, error) {
							log.Info(ctx, map[string]interface{}{
								"status":    statusCode,
								"method":    method,
								"namespace": env.GetNamespace(request),
								"name":      env.GetName(request),
								"kind":      env.GetKind(request),
							}, "resource requested")
							return "", nil, nil
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
					errorChan <- err
					return
				}
			}(key, val, masterOpts)
		}
	}
	wg.Wait()
	vm, err := handleErrors(errorChan, "creation", ctx, config, tnnt, usertoken, repo, allowSelfHealing)
	if vm != nil {
		return vm, err
	}
	return versionMapping, err
}

func RawUpdateTenant(ctx context.Context, config Config, tnnt *tenant.Tenant, envTypes []env.Type, usertoken string,
	repo tenant.Service, allowSelfHealing bool) (map[env.Type]string, error) {

	templs, versionMapping, err := LoadProcessedTemplates(ctx, config, tnnt.OSUsername, tnnt.NsBaseName, envTypes)
	if err != nil {
		return versionMapping, err
	}

	masterOpts := ApplyOptions{Config: config}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return versionMapping, err
	}
	var wg sync.WaitGroup
	errorChan := make(chan error, len(mapped))
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
				errorChan <- errors.Wrap(err, output)
				return
			}
			log.Info(ctx, map[string]interface{}{
				"output":    output,
				"namespace": namespace,
			}, "applied")
		}(key, val, masterOpts)
	}
	wg.Wait()
	vm, err := handleErrors(errorChan, "update", ctx, config, tnnt, usertoken, repo, allowSelfHealing)
	if vm != nil {
		return vm, err
	}
	return versionMapping, err
}

func handleErrors(errorChan chan error, action string, ctx context.Context, config Config, tnnt *tenant.Tenant,
	usertoken string, repo tenant.Service, allowSelfHealing bool) (map[env.Type]string, error) {

	var errs []string
	close(errorChan)
	for er := range errorChan {
		if er != nil {
			errs = append(errs, er.Error())
		}
	}
	if len(errs) > 0 {
		log.Error(ctx, map[string]interface{}{
			"errs": errs,
		}, "%s of namespaces failed with one or more errors", action)
		if allowSelfHealing && usertoken != "" {
			err := DeleteNamespaces(ctx, tnnt.ID, config, repo)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to cleanup namespaces in order to create a new ones")
			}
			newNsBaseName, err := tenant.ConstructNsBaseName(repo, env.RetrieveUserName(tnnt.OSUsername))
			if err != nil {
				return nil, errors.Wrapf(err, "unable to construct namespace base name for user wit OSname %s", tnnt.OSUsername)
			}
			tnnt.NsBaseName = newNsBaseName
			err = repo.SaveTenant(tnnt)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to update tenant %s", tnnt.ID)
			}
			return RawInitTenant(ctx, config, tnnt, usertoken, repo, false)
		}
		return nil, fmt.Errorf("%s of namespaces failed with one or more errors %s", action, errs)
	}
	return nil, nil
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

// InitTenant is a Callback that assumes a new tenant is being created
func InitTenant(ctx context.Context, masterURL string, service tenant.Service, currentTenant *tenant.Tenant) Callback {
	var maxResourceQuotaStatusCheck int32 = 50 // technically a global retry count across all ResourceQuota on all Tenant Namespaces
	var currentResourceQuotaStatusCheck int32  // default is 0
	return func(statusCode int, method string, request, response map[interface{}]interface{}, versionMapping map[env.Type]string) (string, map[interface{}]interface{}, error) {
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
		if statusCode == http.StatusConflict || statusCode == http.StatusForbidden {
			if env.GetKind(request) == env.ValKindNamespace || env.GetKind(request) == env.ValKindProjectRequest ||
				env.GetKind(request) == env.ValKindPersistenceVolumeClaim {
				return "", nil, fmt.Errorf("unable to create %s - should create with other base-name", env.GetNamespace(request))
			}
			return "DELETE", request, nil
		} else if statusCode == http.StatusCreated {
			if env.GetKind(request) == env.ValKindProjectRequest {
				name := env.GetName(request)
				envType := tenant.GetNamespaceType(name, currentTenant.NsBaseName)
				templatesVersion := versionMapping[envType]
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     tenant.Ready,
					Version:   templatesVersion,
					Type:      envType,
					MasterURL: masterURL,
					UpdatedBy: configuration.Commit,
				})

				// HACK to workaround osio applying some dsaas-user permissions async
				// Should loop on a Check if allowed type of call instead
				time.Sleep(time.Second * 5)

			} else if env.GetKind(request) == env.ValKindNamespace {
				name := env.GetName(request)
				envType := tenant.GetNamespaceType(name, currentTenant.NsBaseName)
				templatesVersion := versionMapping[envType]
				service.SaveNamespace(&tenant.Namespace{
					TenantID:  currentTenant.ID,
					Name:      name,
					State:     tenant.Ready,
					Version:   templatesVersion,
					Type:      envType,
					MasterURL: masterURL,
					UpdatedBy: configuration.Commit,
				})
			} else if env.GetKind(request) == env.ValKindResourceQuota {
				// trigger a check status loop
				time.Sleep(time.Millisecond * 50)
				return "GET", response, nil
			}
			return "", nil, nil
		} else if statusCode == http.StatusOK {
			if method == "DELETE" {
				return "POST", request, nil
			} else if method == "GET" {
				if env.GetKind(request) == env.ValKindResourceQuota {

					if env.HasValidStatus(response) || atomic.LoadInt32(&currentResourceQuotaStatusCheck) >= maxResourceQuotaStatusCheck {
						return "", nil, nil
					}
					atomic.AddInt32(&currentResourceQuotaStatusCheck, 1)
					time.Sleep(time.Millisecond * 50)
					return "GET", response, nil
				}
			}
			return "", nil, nil
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
		return "", nil, nil
	}
}

func yamlString(data map[interface{}]interface{}) string {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Could not marshal yaml %v", data)
	}
	return string(b)
}
