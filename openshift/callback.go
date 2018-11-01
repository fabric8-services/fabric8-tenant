package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/retry"
	"github.com/fabric8-services/fabric8-tenant/environment"
	ghodssYaml "github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"net/http"
	"time"
	"github.com/pkg/errors"
)

type BeforeDoCallback struct {
	Call BeforeDoCallbackFunc
	Name string
}

type AfterDoCallback struct {
	Call AfterDoCallbackFunc
	Name string
}

type BeforeDoCallbackFunc func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error)
type AfterDoCallbackFunc func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error

const (
	GetObjectAndMergeName = "GetObjectAndMerge"
	WhenConflictThenDeleteAndRedoName = "WhenConflictThenDeleteAndRedo"
	IgnoreConflictsName = "IgnoreConflicts"
	GetObjectName = "GetObject"
)

// Before callbacks
var GetObjectAndMerge = BeforeDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error) {
		result, err := objEndpoints.Apply(client, object, http.MethodGet)
		if err != nil {
			if result != nil && result.response.StatusCode == http.StatusNotFound {
				return getMethodAndMarshalObject(objEndpoints, http.MethodPost, object)
			}
			return nil, nil, err
		}
		modifiedJson, err := marshalYAMLToJSON(object)
		if err != nil {
			return nil, nil, err
		}
		return method, modifiedJson, nil
	},
	Name: GetObjectAndMergeName,
}

func getMethodAndMarshalObject(objEndpoints *ObjectEndpoints, method string, object environment.Object) (*MethodDefinition, []byte, error) {
	post, err := objEndpoints.GetMethodDefinition(method, object)
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
var WhenConflictThenDeleteAndRedo = AfterDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
		if result.response != nil && result.response.StatusCode == http.StatusConflict {
			log.Warn("There was a conflict, trying to delete the object and re-do the operation")
			err := checkHTTPCode(objEndpoints.Apply(client, object, http.MethodDelete))
			if err != nil {
				return errors.Wrap(err, "delete request failed while removing an object because of a conflict")
			}
			redoMethod := *method
			for idx, callback := range redoMethod.afterDoCallbacks {
				if callback.Name == WhenConflictThenDeleteAndRedoName {
					redoMethod.afterDoCallbacks = append(redoMethod.afterDoCallbacks[:idx], redoMethod.afterDoCallbacks[idx+1:]...)
					break
				}
			}
			err = checkHTTPCode(objEndpoints.apply(client, object, &redoMethod))
			if err != nil {
				return errors.Wrapf(err, "redoing an action %s failed after the object was successfully removed because of a previous conflict", method.action)
			}
			return nil
		}
		return checkHTTPCode(result, result.err)
	},
	Name: WhenConflictThenDeleteAndRedoName,
}

var IgnoreConflicts = AfterDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
		if result.response.StatusCode == http.StatusConflict {
			return nil
		}
		return checkHTTPCode(result, result.err)
	},
	Name: IgnoreConflictsName,
}

var GetObject = AfterDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
		err := checkHTTPCode(result, result.err)
		if err != nil {
			return err
		}
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
				"to get the created objects from the cluster %s", method.action, object, retries, client.MasterURL)
		}
		return nil
	},
	Name: GetObjectName,
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
