package openshift

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"

	env "github.com/fabric8-services/fabric8-tenant/environment"
	"gopkg.in/yaml.v2"
)

var (
	deleteOptions = `apiVersion: v1
kind: DeleteOptions
gracePeriodSeconds: 0
orphanDependents: false`

	adminRole = `apiVersion: v1
kind: RoleBinding
metadata:
  name: admin`

	endpoints = map[string]map[string]string{
		"POST": {
			"Namespace":              `/api/v1/namespaces`,
			"Project":                `/oapi/v1/projects`,
			"ProjectRequest":         `/oapi/v1/projectrequests`,
			"Role":                   `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes`,
			"Deployment":             `/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments`,
			"DeploymentConfig":       `/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges`,
			"Job":                    `/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs`,
		},
		"PUT": {
			"Namespace":              `/api/v1/namespaces/{{ index . "metadata" "name"}}`,
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"Role":                   `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"Deployment":             `/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/apis/apps.openshift.io/v1/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
			"Job":                    `/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs/{{ index . "metadata" "name"}}`,
		},
		"PATCH": {
			"Namespace":              `/api/v1/namespaces/{{ index . "metadata" "name"}}`,
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"Role":                   `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"Deployment":             `/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
			"Job":                    `/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs/{{ index . "metadata" "name"}}`,
		},
		"GET": {
			"Namespace":              `/api/v1/namespaces/{{ index . "metadata" "name"}}`,
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"Role":                   `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"Deployment":             `/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
			"Job":                    `/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs/{{ index . "metadata" "name"}}`,
		},
		"DELETE": {
			"Namespace":              `/api/v1/namespaces/{{ index . "metadata" "name"}}`,
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"Role":                   `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"Deployment":             `/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
			"Job":                    `/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs/{{ index . "metadata" "name"}}`,
		},
	}
)

// Callback is called after initial action
type Callback func(statusCode int, method string, request, response map[interface{}]interface{}, versionMapping map[string]string) (string, map[interface{}]interface{})
type CallbackWithVersionMapping func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{})

// ApplyOptions contains options for connecting to the target API
type ApplyOptions struct {
	Config
	Namespace string
	Callback  CallbackWithVersionMapping
}

// WithNamespace returns a new ApplyOptions with the specified namespace
func (a *ApplyOptions) WithNamespace(namespace string) ApplyOptions {
	return ApplyOptions{
		Config:    a.Config,
		Callback:  a.Callback,
		Namespace: namespace,
	}
}

// WithCallback returns a new ApplyOptions with the specified callback
func (a *ApplyOptions) WithCallback(callback CallbackWithVersionMapping) ApplyOptions {
	return ApplyOptions{
		Config:    a.Config,
		Callback:  callback,
		Namespace: a.Namespace,
	}
}

func ApplyProcessed(objects env.Objects, opts ApplyOptions) error {

	err := allKnownTypes(objects)
	if err != nil {
		return err
	}

	err = applyAll(objects, opts)
	if err != nil {
		return err
	}

	return nil
}

func applyAll(objects env.Objects, opts ApplyOptions) error {
	for _, obj := range objects {
		_, err := Apply(obj, "POST", opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func Apply(object env.Object, action string, opts ApplyOptions) (env.Object, error) {
	body, err := yaml.Marshal(object)
	if err != nil {
		return nil, err
	}
	if action == "DELETE" {
		body = []byte(deleteOptions)
	}

	url, err := CreateURL(opts.MasterURL, action, object)
	if url == "" {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(action, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/yaml")
	req.Header.Set("Content-Type", "application/yaml")
	if action == "PATCH" {
		req.Header.Set("Content-Type", "application/merge-patch+json")
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)

	// for debug only
	if false {
		rb, _ := httputil.DumpRequest(req, true)
		fmt.Println(string(rb))
	}

	client := opts.CreateHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	var respType env.Object
	err = yaml.Unmarshal(b, &respType)
	if err != nil {
		return nil, err
	}

	if opts.Callback != nil {
		act, newObject := opts.Callback(resp.StatusCode, action, object, respType)
		if act != "" {
			return Apply(newObject, act, opts)
		}

	}
	return respType, nil
}

func CreateAdminRoleBinding(namespace string) env.Object {
	objs, err := env.ParseObjects(adminRole)
	if err == nil {
		obj := objs[0]
		if val, ok := obj[env.FieldMetadata].(env.Object); ok {
			val[env.FieldNamespace] = namespace
		}
		return obj
	}
	return env.Object{}
}

// TODO: a bit off now that there are multiple Action methods
func allKnownTypes(objects env.Objects) error {
	m := multiError{}
	for _, obj := range objects {
		if _, ok := endpoints["POST"][env.GetKind(obj)]; !ok {
			m.Errors = append(m.Errors, fmt.Errorf("unknown type: %v", env.GetKind(obj)))
		}
	}
	if len(m.Errors) > 0 {
		return m
	}
	return nil
}

func CreateURL(hostURL, action string, object env.Object) (string, error) {
	urlTemplate, found := endpoints[action][env.GetKind(object)]
	if !found {
		return "", nil
	}
	target, err := template.New("url").Parse(urlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = target.Execute(&buf, object)
	if err != nil {
		return "", err
	}
	str := buf.String()
	if strings.HasSuffix(hostURL, "/") {
		hostURL = hostURL[0 : len(hostURL)-1]
	}

	return hostURL + str, nil
}
