package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/retry"
	"github.com/fabric8-services/fabric8-tenant/utils"
	ghodssYaml "github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"net/http"
	"time"
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
	GetObjectAndMergeName             = "GetObjectAndMerge"
	WhenConflictThenDeleteAndRedoName = "WhenConflictThenDeleteAndRedo"
	IgnoreConflictsName               = "IgnoreConflicts"
	GetObjectName                     = "GetObject"
	IgnoreWhenDoesNotExistName        = "IgnoreWhenDoesNotExistOrConflicts"
)

// Before callbacks
var GetObjectAndMerge = BeforeDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error) {
		result, err := objEndpoints.Apply(client, object, http.MethodGet)
		fmt.Println("####################################")
		fmt.Println(string(result.Body))
		// should check state
		fmt.Println("####################################")
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
			// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
			//log.Warn(nil, map[string]interface{}{
			//	"method": method.action,
			//	"object": object,
			//}, "there was a conflict, trying to delete the object and re-do the operation")
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

var WhenConflictThenFail = AfterDoCallback{
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
		errorChan := retry.Do(retries, time.Millisecond*100, func() error {
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
		msg := utils.ListErrorsInMessage(errorChan)
		if len(msg) > 0 {
			return fmt.Errorf("unable to finish the action %s on a object %s as there were %d of unsuccessful retries "+
				"to get the created objects from the cluster %s. The retrieved errors:%s",
				method.action, object, retries, client.MasterURL, msg)
		}
		return nil
	},
	Name: GetObjectName,
}

var IgnoreWhenDoesNotExistOrConflicts = AfterDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
		code := result.response.StatusCode
		if code == http.StatusNotFound || code == http.StatusForbidden || code == http.StatusConflict{
			// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
			//log.Warn(nil, map[string]interface{}{
			//	"action":  method.action,
			//	"status":  result.response.Status,
			//	"object":  object.ToString(),
			//	"message": result.Body,
			//}, "failed to %s the object. Ignoring this error because it probably does not exist or is being removed", method.action)
			return nil
		}
		return checkHTTPCode(result, result.err)
	},
	Name: IgnoreWhenDoesNotExistName,
}

func checkHTTPCode(result *Result, e error) error {
	if e == nil && result.response != nil && (result.response.StatusCode < 200 || result.response.StatusCode >= 300) {
		return fmt.Errorf("server responded with status: %d for the %s request %s with the Body [%s]",
			result.response.StatusCode, result.response.Request.Method, result.response.Request.URL, string(result.Body))
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
