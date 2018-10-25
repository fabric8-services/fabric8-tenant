package openshift

import (
	"fmt"
	"github.com/arquillian/ike-prow-plugins/pkg/retry"
	"github.com/fabric8-services/fabric8-tenant/environment"
	ghodssYaml "github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"net/http"
	"time"
)

type BeforeDoCallback func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error)
type AfterDoCallback func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error

// Before callbacks

func GetObjectAndMerge(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error) {
	result, err := objEndpoints.Apply(client, object, http.MethodGet)
	if err != nil {
		if result.response.StatusCode == http.StatusNotFound {
			return getMethodAndMarshalObject(objEndpoints, http.MethodPost, object)
		}
		return nil, nil, err
	}
	modifiedJson, err := marshalYAMLToJSON(object)
	if err != nil {
		return nil, nil, err
	}
	return method, modifiedJson, nil
}

func getMethodAndMarshalObject(objEndpoints *ObjectEndpoints, method string, object environment.Object) (*MethodDefinition, []byte, error) {
	post, err := objEndpoints.getMethodDefinition(method, object)
	if err != nil {
		return nil, nil, err
	}
	bytes, err := yaml.Marshal(object)
	if err != nil {
		return nil, nil, err
	}
	return post, bytes, nil
}

// After callbacks

func WhenConflictThenDeleteAndRedo(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
	if result.response.StatusCode == http.StatusConflict {
		log.Warn("There was a conflict, trying to delete the object and re-do the operation")
		err := checkHTTPCode(objEndpoints.Apply(client, object, http.MethodDelete))
		if err != nil {
			return err
		}
		return checkHTTPCode(objEndpoints.Apply(client, object, method.action))
	}
	return checkHTTPCode(result, nil)
}

func IgnoreConflicts(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
	if result.response.StatusCode == http.StatusConflict {
		return nil
	}
	return checkHTTPCode(result, nil)
}

func GetObject(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
	retries := 50
	errs := retry.Do(retries, time.Millisecond*100, func() error {
		getResponse, err := objEndpoints.Apply(client, object, http.MethodGet)
		err = checkHTTPCode(getResponse, err)
		if err != nil {
			return err
		}
		getObject, err := getResponse.bodyToObject()
		if err != nil {
			return err
		}
		if !environment.HasValidStatus(getObject) {
			return fmt.Errorf("not ready yet")
		}
		return nil
	})
	if len(errs) > 0 {
		return fmt.Errorf("unable to finish the action %s on a object %s as there were %d of unsuccessful retries "+
			"to get the created objects from the cluster %s", method.action, object, retries, result.response.Request.URL.Host)
	}
	return nil
}

func checkHTTPCode(result *Result, e error) error {
	if e == nil && result.response != nil && (result.response.StatusCode < 200 || result.response.StatusCode >= 300) {
		return fmt.Errorf("server responded with status: %d for the request %s %s with the body %s",
			result.response.StatusCode, result.response.Request.Method, result.response.Request.URL, result.body)
	}
	return e
}

func yamlString(data environment.Object) string {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Could not marshal yaml %v", data)
	}
	return string(b)
}

func marshalYAMLToJSON(object interface{}) ([]byte, error) {
	bytes, err := yaml.Marshal(object)
	if err != nil {
		return nil, err
	}
	return ghodssYaml.YAMLToJSON(bytes)

}
