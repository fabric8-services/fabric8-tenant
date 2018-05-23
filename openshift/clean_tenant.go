package openshift

import (
	"context"
	"fmt"
	"sync"

	"github.com/fabric8-services/fabric8-wit/log"
)

// CleanTenant clean or remove
func CleanTenant(ctx context.Context, config Config, username string, templateVars map[string]string, remove bool) error {
	templs, err := LoadProcessedTemplates(ctx, config, username, templateVars)
	if err != nil {
		return err
	}

	mapped, err := MapByNamespaceAndSort(templs)
	if err != nil {
		return err
	}
	masterOpts := ApplyOptions{Config: config}
	var wg sync.WaitGroup
	wg.Add(len(mapped))
	for key, val := range mapped {
		go func(namespace string, objects []map[interface{}]interface{}, opts ApplyOptions, remove bool) {
			defer wg.Done()
			cleaner := NamespaceCleaner
			if remove {
				log.Info(ctx, map[string]interface{}{"namespace": namespace}, "deleting namespace")
				cleaner = NamespaceDeleter
			}
			output, err := cleaner.ExecCmd(
				namespace,
				opts.WithNamespace(namespace),
			)
			if err != nil {
				log.Error(ctx, map[string]interface{}{
					"output":      output,
					"cluster_url": opts.MasterURL,
					"namespace":   namespace,
					"error":       err,
				}, "clean failed")
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
	return nil
}

// OCCommandExecutor the interface for running a command
type OCCommandExecutor interface {
	ExecCmd(namespace string, opt ApplyOptions) (string, error)
}

// NamespaceCleaner the namespace cleaner implementation
var NamespaceCleaner OCCommandExecutor = defaultNamespaceCleaner{}

// NamespaceCleaner the default namespace cleaner
type defaultNamespaceCleaner struct {
}

// ExecCmd executes the `oc delete...` command on the given namespace in the given cluster
func (c defaultNamespaceCleaner) ExecCmd(namespace string, opt ApplyOptions) (string, error) {
	return executeCMD(nil, []string{"-c", fmt.Sprintf("oc delete all,pvc,cm --all --now=true --namespace=%v --server=%v --token=%v", namespace, opt.MasterURL, opt.Token)})
}

// NamespaceDeleter the current namespace deleter implementation
var NamespaceDeleter OCCommandExecutor = defaultNamespaceDeleter{}

// NamespaceDeleter the default namespace deleter implementation
type defaultNamespaceDeleter struct {
}

// ExecCmd executes the `oc delete...` command on the given namespace in the given cluster
func (c defaultNamespaceDeleter) ExecCmd(namespace string, opt ApplyOptions) (string, error) {
	return executeCMD(nil, []string{"-c", fmt.Sprintf("oc delete project %v --server=%v --token=%v", namespace, opt.MasterURL, opt.Token)})
}
