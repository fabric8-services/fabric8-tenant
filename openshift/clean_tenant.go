package openshift

import (
	"context"
	"fmt"
	"sync"

	errs "github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

// CleanTenant clean or remove
func CleanTenant(ctx context.Context, config Config, osUsername, nsBaseName string, remove bool) error {
	templs, _, err := LoadProcessedTemplates(ctx, config, osUsername, nsBaseName, environment.DefaultEnvTypes)
	if err != nil {
		return err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return err
	}
	masterOpts := ApplyOptions{Config: config}
	var wg sync.WaitGroup
	errorChan := make(chan error, len(mapped))
	wg.Add(len(mapped))
	for key, val := range mapped {
		go func(namespace string, objects []map[interface{}]interface{}, opts ApplyOptions, remove bool) {
			defer wg.Done()
			var clean cleanFunc = executeCleanNamespaceCMD
			if remove {
				clean = executeDeleteNamespaceCMD
			}
			output, err := clean(
				namespace,
				opts.WithNamespace(namespace),
			)
			if err != nil {
				sentry.LogError(ctx, map[string]interface{}{
					"output":      output,
					"cluster_url": opts.MasterURL,
					"namespace":   namespace,
				}, err, "clean failed")
				errorChan <- errors.Wrap(err, output)
				return
			}
			log.Info(ctx, map[string]interface{}{
				"output":      output,
				"cluster_url": opts.MasterURL,
				"namespace":   namespace,
			}, "clean ok")
		}(key, val, masterOpts, remove)
	}
	wg.Wait()
	var errs []error
	close(errorChan)
	for er := range errorChan {
		if er != nil {
			errs = append(errs, er)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("cleanup of namespaces failed with one or more errors %s", errs)
	}
	return nil
}

type cleanFunc func(namespace string, opt ApplyOptions) (string, error)

func executeCleanNamespaceCMD(namespace string, opt ApplyOptions) (string, error) {
	return executeCMD(nil, []string{"-c", fmt.Sprintf("oc delete all,pvc,cm --all --now=true --namespace=%v --server=%v --token=%v", namespace, opt.MasterURL, opt.Token)})
}

func executeDeleteNamespaceCMD(namespace string, opt ApplyOptions) (string, error) {
	return executeCMD(nil, []string{"-c", fmt.Sprintf("oc delete project %v --server=%v --token=%v", namespace, opt.MasterURL, opt.Token)})
}

func DeleteNamespaces(ctx context.Context, tenantID uuid.UUID, openshiftConfig Config, tenantService tenant.Service) error {
	namespaces, err := tenantService.GetNamespaces(tenantID)
	if err != nil {
		return jsonapi.JSONErrorResponse(ctx, err)
	}
	for _, namespace := range namespaces {
		log.Info(ctx, map[string]interface{}{"tenant_id": tenantID, "namespace": namespace.Name}, "deleting namespace...")
		// delete the namespace in the cluster
		openshiftService := NewService()
		err = openshiftService.DeleteNamespace(ctx, openshiftConfig, namespace.Name)
		if err != nil {
			log.Error(ctx, map[string]interface{}{
				"err":         err,
				"cluster_url": namespace.MasterURL,
				"namespace":   namespace.Name,
				"tenant_id":   tenantID,
			}, "failed to delete namespace")
			return jsonapi.JSONErrorResponse(ctx, err)
		}
	}
	if err := tenantService.DeleteNamespaces(tenantID); err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":       err,
			"tenant_id": tenantID,
		}, "failed to delete namespaces entities from DB")
		return jsonapi.JSONErrorResponse(ctx, errs.NewInternalError(ctx, err))
	}
	return nil
}
