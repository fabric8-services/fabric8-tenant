package openshift

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"unsafe"

	yaml "gopkg.in/yaml.v2"
)

const (
	fieldKind       = "kind"
	fieldAPIVersion = "apiVersion"
	fieldObjects    = "objects"
	fieldItems      = "items"
	fieldMetadata   = "metadata"
	fieldNamespace  = "namespace"
	fieldName       = "name"

	valTemplate = "Template"
	valList     = "List"
)

var (
	endpoints = map[string]string{
		"Project":               `/oapi/v1/projects`,
		"RoleBinding":           `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings`,
		"Route":                 `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes`,
		"DeploymentConfig":      `/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs`,
		"PersistentVolumeClaim": `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims`,
		"Service":               `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services`,
		"Secret":                `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets`,
		"ServiceAccount":        `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts`,
		"ConfigMap":             `/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps`,
	}
)

// ApplyOptions contains options for connecting to the target API
type ApplyOptions struct {
	Config
	Namespace string
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
	for _, obj := range objects {
		err := apply(obj, opts)
		if err != nil {
			return err
		}
	}
	return nil
}

func apply(object map[interface{}]interface{}, opts ApplyOptions) error {
	body, err := yaml.Marshal(object)
	if err != nil {
		return err
	}

	url, err := createURL(opts.MasterURL, object)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
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
		return err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unknown response:\n%v\n%v", *(*string)(unsafe.Pointer(&b)), string(rb))
	}

	var respType map[interface{}]interface{}
	err = yaml.Unmarshal(b, &respType)
	if err != nil {
		return err
	}

	fmt.Printf("Created %v %v in %v\n", respType[fieldKind], getName(respType), opts.Namespace)
	return nil
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
		if template[fieldKind] == valList && namespace != "" {
			for _, obj := range objs {
				if val, ok := obj[fieldMetadata].(map[interface{}]interface{}); ok {
					val[fieldNamespace] = namespace
				}
			}
		}

		return objs, nil
	}
	return []map[interface{}]interface{}{template}, nil
}

func allKnownTypes(objects []map[interface{}]interface{}) error {
	m := multiError{}
	for _, obj := range objects {
		if _, ok := endpoints[obj[fieldKind].(string)]; !ok {
			m.Errors = append(m.Errors, fmt.Errorf("Unknown type: %v", obj[fieldKind]))
		}
	}
	if len(m.Errors) > 0 {
		return m
	}
	return nil
}

func createURL(hostURL string, object map[interface{}]interface{}) (string, error) {
	urlTemplate := endpoints[object[fieldKind].(string)]
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
