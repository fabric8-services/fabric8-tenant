package openshift

import (
	"bytes"
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type FilterFunc func(environment.Object) bool

func Filter(vs environment.Objects, f FilterFunc) environment.Objects {
	vsf := make(environment.Objects, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func IsOfKind(kinds ...string) FilterFunc {
	return func(vs environment.Object) bool {
		kind := environment.GetKind(vs)
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
}

func IsNotOfKind(kinds ...string) FilterFunc {
	f := IsOfKind(kinds...)
	return func(vs environment.Object) bool {
		return !f(vs)
	}
}

func LoadProcessedTemplates(ctx context.Context, config Config, username string) (environment.Objects, error) {

	envService := environment.NewService(ctx, config.TemplatesRepo, config.TemplatesRepoBlob, config.TemplatesRepoDir)
	vars := environment.CollectVars(username, config.MasterUser, config.Commit, config.OriginalConfig)
	var objs environment.Objects

	for _, envType := range environment.DefaultEnvTypes {
		env, err := envService.GetEnvData(envType)
		if err != nil {
			return nil, err
		}
		for _, template := range env.Templates {
			if os.Getenv("DISABLE_OSO_QUOTAS") == "true" && strings.Contains(template.Filename, "quotas") {
				continue
			}

			objects, err := template.Process(vars)
			if err != nil {
				return nil, err
			}
			objs = append(objs, objects...)
		}
	}

	return objs, nil
}

func MapByNamespaceAndSort(objs environment.Objects) (map[string]environment.Objects, error) {
	ns := map[string]environment.Objects{}
	for _, obj := range objs {
		namespace := environment.GetNamespace(obj)
		if namespace == "" {
			// ProjectRequests and Namespaces are not bound to a Namespace, as it's a Namespace request
			kind := environment.GetKind(obj)
			if kind == environment.ValKindProjectRequest || kind == environment.ValKindNamespace {
				namespace = environment.GetName(obj)
			} else {
				return nil, fmt.Errorf("object is missing namespace %v", obj)
			}
		}

		if objects, found := ns[namespace]; found {
			objects = append(objects, obj)
			ns[namespace] = objects
		} else {
			objects = environment.Objects{obj}
			ns[namespace] = objects
		}
	}

	for key, val := range ns {
		sort.Sort(environment.ByKind(val))
		ns[key] = val
	}
	return ns, nil
}

func executeProccessedNamespaceCMD(t string, opts ApplyOptions) (string, error) {
	hostVerify := ""
	flag := os.Getenv("KEYCLOAK_SKIP_HOST_VERIFY")
	if strings.ToLower(flag) == "true" {
		hostVerify = " --insecure-skip-tls-verify=true"
	}
	serverFlag := "--server=" + opts.MasterURL + hostVerify
	cmdArgs := []string{"-c", "oc process -f - " + serverFlag + " --token=" + opts.Token + " --namespace=" + opts.Namespace + " | oc apply -f -  --overwrite=true --force=true --server=" + opts.MasterURL + hostVerify + " --token=" + opts.Token + " --namespace=" + opts.Namespace}
	return executeCMD(&t, cmdArgs)
}

func executeCMD(input *string, cmdArgs []string) (string, error) {
	cmdName := "/usr/bin/sh"

	var buf bytes.Buffer
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	if input != nil {
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, *input)

		}()
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return buf.String(), err
	}

	return buf.String(), nil
}
