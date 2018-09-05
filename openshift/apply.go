package openshift

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"reflect"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

const (
	FieldKind                     = "kind"
	FieldAPIVersion               = "apiVersion"
	FieldObjects                  = "objects"
	FieldSpec                     = "spec"
	FieldTemplate                 = "template"
	FieldItems                    = "items"
	FieldMetadata                 = "metadata"
	FieldLabels                   = "labels"
	FieldReplicas                 = "replicas"
	FieldVersion                  = "version"
	FieldNamespace                = "namespace"
	FieldName                     = "name"
	FieldStatus                   = "status"
	FieldResourceVersion          = "resourceVersion"
	ValKindTemplate               = "Template"
	ValKindNamespace              = "Namespace"
	ValKindProjectRequest         = "ProjectRequest"
	ValKindPersistenceVolumeClaim = "PersistentVolumeClaim"
	ValKindServiceAccount         = "ServiceAccount"
	ValKindRoleBindingRestriction = "RoleBindingRestriction"
	ValKindRoleBinding            = "RoleBinding"
	ValKindList                   = "List"
	ValKindDeploymentConfig       = "DeploymentConfig"
	ValKindResourceQuota          = "ResourceQuota"
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
type Callback func(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{})

// ApplyOptions contains options for connecting to the target API
type ApplyOptions struct {
	Config
	Namespace string
	Callback  Callback
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
func (a *ApplyOptions) WithCallback(callback Callback) ApplyOptions {
	return ApplyOptions{
		Config:    a.Config,
		Callback:  callback,
		Namespace: a.Namespace,
	}
}

// Apply a given template structure to a target API
func Apply(source string, opts ApplyOptions) error {

	objects, err := ParseObjects(source, opts.Namespace)
	if err != nil {
		return err
	}

	err = allKnownTypes(objects)
	if err != nil {
		return err
	}

	err = applyAll(objects, opts)
	if err != nil {
		return err
	}

	return nil
}

func ApplyProcessed(objects []map[interface{}]interface{}, opts ApplyOptions) error {

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

func applyAll(objects []map[interface{}]interface{}, opts ApplyOptions) error {
	for _, obj := range objects {
		_, err := apply(obj, "POST", opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func apply(object map[interface{}]interface{}, action string, opts ApplyOptions) (map[interface{}]interface{}, error) {
	//fmt.Println("apply ", action, GetKind(object), GetName(object), opts.Callback)

	body, err := yaml.Marshal(object)
	if err != nil {
		return nil, err
	}
	if action == "DELETE" {
		body = []byte(deleteOptions)
	}

	url, err := CreateURL(opts.MasterURL, action, object)
	if url == "" {
		return nil, nil
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

	var respType map[interface{}]interface{}
	err = yaml.Unmarshal(b, &respType)
	if err != nil {
		return nil, err
	}

	if opts.Callback != nil {
		act, newObject := opts.Callback(resp.StatusCode, action, object, respType)
		if act != "" {
			return apply(newObject, act, opts)
		}

	}
	return respType, nil
}

func updateResourceVersion(source, target map[interface{}]interface{}) {
	if sourceMeta, sourceMetaFound := source[FieldMetadata].(map[interface{}]interface{}); sourceMetaFound {
		if sourceVersion, sourceVersionFound := sourceMeta[FieldResourceVersion]; sourceVersionFound {
			if targetMeta, targetMetaFound := target[FieldMetadata].(map[interface{}]interface{}); targetMetaFound {
				fmt.Println("setting v", sourceVersion, reflect.TypeOf(sourceVersion).Kind())
				targetMeta[FieldResourceVersion] = sourceVersion
			}
		}
	}
}

func HasValidStatus(obj map[interface{}]interface{}) bool {
	return len(GetStatus(obj)) > 0
}

func GetStatus(obj map[interface{}]interface{}) map[interface{}]interface{} {
	if status, statusFound := obj[FieldStatus].(map[interface{}]interface{}); statusFound {
		return status
	}
	return nil
}

func GetName(obj map[interface{}]interface{}) string {
	if meta, metaFound := obj[FieldMetadata].(map[interface{}]interface{}); metaFound {
		if name, nameFound := meta[FieldName].(string); nameFound {
			return name
		}
	}
	return ""
}

func GetNamespace(obj map[interface{}]interface{}) string {
	if meta, metaFound := obj[FieldMetadata].(map[interface{}]interface{}); metaFound {
		if name, nameFound := meta[FieldNamespace].(string); nameFound {
			return name
		}
	}
	return ""
}

func GetKind(obj map[interface{}]interface{}) string {
	if kind, kindFound := obj[FieldKind].(string); kindFound {
		return kind
	}
	return ""
}

func GetLabelVersion(obj map[interface{}]interface{}) string {
	return GetLabel(obj, FieldVersion)
}

func GetLabel(obj map[interface{}]interface{}, name string) string {
	if meta, metaFound := obj[FieldMetadata].(map[interface{}]interface{}); metaFound {
		if labels, labelsFound := meta[FieldLabels].(map[interface{}]interface{}); labelsFound {
			if version, versionFound := labels[name].(string); versionFound {
				return version
			}
		}
	}
	return ""
}

func GetSpec(obj map[interface{}]interface{}) map[interface{}]interface{} {
	if spec, specFound := obj[FieldSpec].(map[interface{}]interface{}); specFound {
		return spec
	}
	return map[interface{}]interface{}{}
}

func GetTemplate(obj map[interface{}]interface{}) map[interface{}]interface{} {
	if template, templateFound := obj[FieldTemplate].(map[interface{}]interface{}); templateFound {
		return template
	}
	return map[interface{}]interface{}{}
}

// ParseObjects return a string yaml and return a array of the objects/items from a Template/List kind
func ParseObjects(source string, namespace string) ([]map[interface{}]interface{}, error) {
	var template map[interface{}]interface{}

	err := yaml.Unmarshal([]byte(source), &template)
	if err != nil {
		return nil, err
	}

	if GetKind(template) == ValKindTemplate || GetKind(template) == ValKindList {
		var ts []interface{}
		if GetKind(template) == ValKindTemplate {
			ts = template[FieldObjects].([]interface{})
		} else if GetKind(template) == ValKindList {
			ts = template[FieldItems].([]interface{})
		}
		var objs []map[interface{}]interface{}
		for _, obj := range ts {
			objs = append(objs, obj.(map[interface{}]interface{}))
		}
		if namespace != "" {
			for _, obj := range objs {
				kind := GetKind(obj)
				if val, ok := obj[FieldMetadata].(map[interface{}]interface{}); ok && kind != ValKindProjectRequest && kind != ValKindNamespace {
					if _, ok := val[FieldNamespace]; !ok {
						val[FieldNamespace] = namespace
					}
				}
			}
		}
		return objs, nil
	}
	return []map[interface{}]interface{}{template}, nil
}

func CreateAdminRoleBinding(namespace string) map[interface{}]interface{} {
	objs, err := ParseObjects(adminRole, namespace)
	if err == nil {
		obj := objs[0]
		if val, ok := obj[FieldMetadata].(map[interface{}]interface{}); ok {
			val[FieldNamespace] = namespace
		}
		return obj
	}
	return map[interface{}]interface{}{}
}

// TODO: a bit off now that there are multiple Action methods
func allKnownTypes(objects []map[interface{}]interface{}) error {
	m := multiError{}
	for _, obj := range objects {
		if _, ok := endpoints["POST"][GetKind(obj)]; !ok {
			m.Errors = append(m.Errors, fmt.Errorf("Unknown type: %v", GetKind(obj)))
		}
	}
	if len(m.Errors) > 0 {
		return m
	}
	return nil
}

func CreateURL(hostURL, action string, object map[interface{}]interface{}) (string, error) {
	urlTemplate, found := endpoints[action][GetKind(object)]
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

var sortOrder = map[string]int{
	"Namespace":              1,
	"ProjectRequest":         1,
	"RoleBindingRestriction": 2,
	"LimitRange":             3,
	"ResourceQuota":          4,
	"Secret":                 5,
	"ServiceAccount":         6,
	"Service":                7,
	"Role":                   8,
	"RoleBinding":            9,
	"PersistentVolumeClaim":  10,
	"ConfigMap":              11,
	"DeploymentConfig":       12,
	"Route":                  13,
	"Job":                    14,
}

// ByKind represents a list of Openshift objects sortable by Kind
type ByKind []map[interface{}]interface{}

func (a ByKind) Len() int      { return len(a) }
func (a ByKind) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByKind) Less(i, j int) bool {
	iO := 30
	jO := 30

	if val, ok := sortOrder[GetKind(a[i])]; ok {
		iO = val
	}
	if val, ok := sortOrder[GetKind(a[j])]; ok {
		jO = val
	}
	return iO < jO
}
