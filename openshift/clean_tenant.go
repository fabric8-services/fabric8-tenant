package openshift

import (
	"context"
	"fmt"
	"sync"

	"github.com/almighty/almighty-core/log"
)

func CleanTenant(ctx context.Context, config Config, username string, templateVars map[string]string) error {
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
		go func(namespace string, objects []map[interface{}]interface{}, opts ApplyOptions) {
			defer wg.Done()
			output, err := executeCleanNamespaceCMD(
				namespace,
				opts.WithNamespace(namespace),
			)
			if err != nil {
				log.Error(ctx, map[string]interface{}{
					"output":    output,
					"namespace": namespace,
					"error":     err,
				}, "clean failed")
				return
			}
			log.Info(ctx, map[string]interface{}{
				"output":    output,
				"namespace": namespace,
			}, "clean ok")
		}(key, val, masterOpts)
	}
	wg.Wait()
	return nil
}

func executeCleanNamespaceCMD(namespace string, opt ApplyOptions) (string, error) {
	return executeCMD(nil, []string{"-c", fmt.Sprintf("oc delete all,pvc,cm --all --now=true --namespace=%v --server=%v --token=%v", namespace, opt.MasterURL, opt.Token)})
}
