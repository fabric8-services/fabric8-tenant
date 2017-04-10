package openshift

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"reflect"
	"unsafe"

	"time"

	yaml "gopkg.in/yaml.v2"
)

const (
	fieldKind            = "kind"
	fieldAPIVersion      = "apiVersion"
	fieldObjects         = "objects"
	fieldItems           = "items"
	fieldMetadata        = "metadata"
	fieldNamespace       = "namespace"
	fieldName            = "name"
	fieldResourceVersion = "resourceVersion"

	valTemplate       = "Template"
	valProjectRequest = "ProjectRequest"
	valList           = "List"
)

var (
	endpoints = map[string]map[string]string{
		"POST": {
			"Project":                `/oapi/v1/projects`,
			"ProjectRequest":         `/oapi/v1/projectrequests`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes`,
			"DeploymentConfig":       `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges`,
		},
		"PUT": {
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
		},
		"GET": {
			"Project":                `/oapi/v1/projects/{{ index . "metadata" "name"}}`,
			"RoleBinding":            `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`,
			"RoleBindingRestriction": `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
			"Route":                  `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`,
			"DeploymentConfig":       `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`,
			"PersistentVolumeClaim":  `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
			"Service":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`,
			"Secret":                 `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`,
			"ServiceAccount":         `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`,
			"ConfigMap":              `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`,
			"ResourceQuota":          `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`,
			"LimitRange":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`,
		},
	}
)

// ApplyOptions contains options for connecting to the target API
type ApplyOptions struct {
	Config
	Overwrite bool
	Namespace string
}

func (a ApplyOptions) withNamespace(namespace string) ApplyOptions {
	return ApplyOptions{
		Config:    a.Config,
		Overwrite: a.Overwrite,
		Namespace: namespace,
	}
}

// Apply a given template structure to a target API
func Apply(source string, opts ApplyOptions) error {

	objects, err := parseObjects(source, opts.Namespace)
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

func applyAll(objects []map[interface{}]interface{}, opts ApplyOptions) error {
	for index, obj := range objects {
		_, err := apply(obj, "POST", opts)
		if err != nil {
			return err
		}
		if index == 0 {
			time.Sleep(time.Second * 2)
		}
	}
	return nil
}

func apply(object map[interface{}]interface{}, action string, opts ApplyOptions) (map[interface{}]interface{}, error) {
	body, err := yaml.Marshal(object)
	if err != nil {
		return nil, err
	}

	url, err := createURL(opts.MasterURL, action, object)
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
	req.Header.Set("Authorization", "Bearer "+opts.Token)

	// for debug only
	rb, _ := httputil.DumpRequest(req, true)
	if false {
		fmt.Println(string(rb))
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	var respType map[interface{}]interface{}
	err = yaml.Unmarshal(b, &respType)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusConflict {
		if object[fieldKind] == valProjectRequest {
			return respType, nil
		}
		/*
			fmt.Println("Conflict-Update")
			resp, err := apply(object, "GET", opts)
			if err != nil {
				return nil, err
			}
			fmt.Println(resp)
			updateResourceVersion(resp, object)
			fmt.Println(object)
		*/
		return apply(object, "PUT", opts)
	}
	/*
		if resp.StatusCode == http.StatusForbidden && opts.Overwrite {

		} else
	*/
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unknown response:\n%v\n%v", *(*string)(unsafe.Pointer(&b)), string(rb))
	}

	fmt.Printf("%v %v %v in %v\n", action, respType[fieldKind], getName(respType), opts.Namespace)
	return respType, nil
}

func updateResourceVersion(source, target map[interface{}]interface{}) {
	if sourceMeta, sourceMetaFound := source[fieldMetadata].(map[interface{}]interface{}); sourceMetaFound {
		if sourceVersion, sourceVersionFound := sourceMeta[fieldResourceVersion]; sourceVersionFound {
			if targetMeta, targetMetaFound := target[fieldMetadata].(map[interface{}]interface{}); targetMetaFound {
				fmt.Println("setting v", sourceVersion, reflect.TypeOf(sourceVersion).Kind())
				targetMeta[fieldResourceVersion] = sourceVersion
			}
		}
	}
}

func getName(obj map[interface{}]interface{}) string {
	if meta, metaFound := obj[fieldMetadata].(map[interface{}]interface{}); metaFound {
		if name, nameFound := meta[fieldName].(string); nameFound {
			return name
		}
	}
	return ""
}

func parseObjects(source string, namespace string) ([]map[interface{}]interface{}, error) {
	var template map[interface{}]interface{}

	err := yaml.Unmarshal([]byte(source), &template)
	if err != nil {
		return nil, err
	}

	if template[fieldKind] == valTemplate || template[fieldKind] == valList {
		var ts []interface{}
		if template[fieldKind] == valTemplate {
			ts = template[fieldObjects].([]interface{})
		} else if template[fieldKind] == valList {
			ts = template[fieldItems].([]interface{})
		}
		var objs []map[interface{}]interface{}
		for _, obj := range ts {
			objs = append(objs, obj.(map[interface{}]interface{}))
		}
		if namespace != "" {
			for _, obj := range objs {
				if val, ok := obj[fieldMetadata].(map[interface{}]interface{}); ok {
					if _, ok := val[fieldNamespace]; !ok {
						val[fieldNamespace] = namespace
					}
				}
			}
		}

		return objs, nil
	}
	return []map[interface{}]interface{}{template}, nil
}

// TODO: a bit off now that there are multiple Action methods
func allKnownTypes(objects []map[interface{}]interface{}) error {
	m := multiError{}
	for _, obj := range objects {
		if _, ok := endpoints["POST"][obj[fieldKind].(string)]; !ok {
			m.Errors = append(m.Errors, fmt.Errorf("Unknown type: %v", obj[fieldKind]))
		}
	}
	if len(m.Errors) > 0 {
		return m
	}
	return nil
}

func createURL(hostURL, action string, object map[interface{}]interface{}) (string, error) {
	urlTemplate, found := endpoints[action][object[fieldKind].(string)]
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
	return hostURL + str, nil
}
