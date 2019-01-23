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
	"strings"
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
	FailIfExistsName                  = "FailIfAlreadyExists"
	WhenConflictThenDeleteAndRedoName = "WhenConflictThenDeleteAndRedo"
	IgnoreConflictsName               = "IgnoreConflicts"
	GetObjectName                     = "GetObject"
	IgnoreWhenDoesNotExistName        = "IgnoreWhenDoesNotExistOrConflicts"
	TryToWaitUntilIsGoneName          = "TryToWaitUntilIsGone"
)

// Before callbacks
var GetObjectAndMerge = BeforeDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error) {
		retries := 10
		var methodToUse = method
		var bodyToSend []byte

		errorChan := retry.Do(retries, time.Second, func() error {
			result, err := objEndpoints.Apply(client, object, http.MethodGet)
			if result != nil && isNotPresent(result.response.StatusCode) {
				methodToUse, bodyToSend, err = getMethodAndMarshalObject(objEndpoints, http.MethodPost, object)
				if err != nil {
					return err
				}
				return nil
			}
			if err != nil {
				return err
			}
			var returnedObj environment.Object
			err = yaml.Unmarshal(result.Body, &returnedObj)
			if err != nil {
				return errors.Wrapf(err, "unable unmarshal object responded from OS while doing GET method")
			}
			if isInTerminatingState(returnedObj) {
				return fmt.Errorf("the object %s is in terminating state - cannot create PATCH for it - need to wait till it is completely removed", returnedObj)
			}
			environment.GetStatus(returnedObj)
			bodyToSend, err = marshalYAMLToJSON(object)
			if err != nil {
				return errors.Wrapf(err, "unable marshal object to be send to OS as part of %s request", method.action)
			}
			return nil
		})

		msg := utils.ListErrorsInMessage(errorChan)
		if len(msg) > 0 {
			return nil, nil, fmt.Errorf("unable to finish the action %s on a object %s as there were %d of unsuccessful retries "+
				"to get object and create a patch for the cluster %s. The retrieved errors:%s",
				method.action, object, retries, client.MasterURL, msg)
		}
		return methodToUse, bodyToSend, nil
	},
	Name: GetObjectAndMergeName,
}

func isInTerminatingState(obj environment.Object) bool {
	status := environment.GetStatus(obj)
	if status != nil && len(status) > 0 {
		phase, ok := status["phase"]
		if ok && strings.ToLower(phase.(string)) == "terminating" {
			return true
		}
	}
	return false
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

var FailIfAlreadyExists = BeforeDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition) (*MethodDefinition, []byte, error) {

		masterClient := *client
		masterClient.TokenProducer = func(forceMasterToken bool) string {
			return client.TokenProducer(true)
		}

		result, err := objEndpoints.Apply(&masterClient, object, http.MethodGet)
		if err != nil {
			if result != nil && isNotPresent(result.response.StatusCode) {
				bodyToSend, err := yaml.Marshal(object)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "unable marshal object to be send to OS as part of %s request", method.action)
				}
				return method, bodyToSend, nil
			}
		}
		return nil, nil, fmt.Errorf("the object [%s] already exists", object.ToString())

	},
	Name: FailIfExistsName,
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
			fmt.Println(fmt.Sprintf("WARNING: there was a conflict when doing %s on object %s, trying to delete the object and re-do the operation",
				method.action, object.ToString()))
			err := checkHTTPCode(objEndpoints.Apply(client, object, http.MethodDelete))
			if err != nil {
				return errors.Wrap(err, "delete request failed while removing an object because of a conflict")
			}
			redoMethod := removeAfterDoCallback(*method, WhenConflictThenDeleteAndRedoName)

			redoResult, err := objEndpoints.apply(client, object, redoMethod)
			result.err = redoResult.err
			result.Body = redoResult.Body
			result.response = redoResult.response
			err = checkHTTPCode(result, err)
			if err != nil {
				return errors.Wrapf(err, "redoing an action %s failed after the object was successfully removed because of a previous conflict", method.action)
			}
			return nil
		}
		return checkHTTPCode(result, result.err)
	},
	Name: WhenConflictThenDeleteAndRedoName,
}

func removeAfterDoCallback(method MethodDefinition, callbackName string) *MethodDefinition {
	withoutCallback := NewMethodDefinition(method.action, method.beforeDoCallbacks, []AfterDoCallback{}, method.requestCreator)
	for _, callback := range method.afterDoCallbacks {
		if callback.Name != callbackName {
			withoutCallback.afterDoCallbacks = append(withoutCallback.afterDoCallbacks, callback)
			break
		}
	}
	return &withoutCallback
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
		if code == http.StatusNotFound || code == http.StatusConflict {
			// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
			//log.Warn(nil, map[string]interface{}{
			//	"action":  method.action,
			//	"status":  result.response.Status,
			//	"object":  object.ToString(),
			//	"message": result.Body,
			//}, "failed to %s the object. Ignoring this error because it probably does not exist or is being removed", method.action)
			fmt.Println(fmt.Sprintf("WARNING: failed to %s the object %s - reveived response %s. "+
				"Ignoring this error because it probably does not exist or is being removed", method.action, object.ToString(), result.response.Status))
			return nil
		}
		return checkHTTPCode(result, result.err)
	},
	Name: IgnoreWhenDoesNotExistName,
}

var TryToWaitUntilIsGone = AfterDoCallback{
	Call: func(client *Client, object environment.Object, objEndpoints *ObjectEndpoints, method *MethodDefinition, result *Result) error {
		err := checkHTTPCode(result, result.err)
		if err != nil {
			return err
		}
		retries := 30
		errorChan := retry.Do(retries, time.Millisecond*500, func() error {
			result, err := objEndpoints.Apply(client, object, http.MethodGet)
			if result != nil && isNotPresent(result.response.StatusCode) {
				return nil
			}
			if err != nil {
				return err
			}
			var returnedObj environment.Object
			err = yaml.Unmarshal(result.Body, &returnedObj)
			if err != nil {
				return errors.Wrapf(err, "unable unmarshal object responded from OS while doing GET method")
			}
			if isInTerminatingState(returnedObj) {
				return nil
			}
			return fmt.Errorf("the object %s hasn't been removed nor set to terminating state "+
				"- waiting till it is completely removed to finish the %s action", returnedObj, method.action)
		})
		msg := utils.ListErrorsInMessage(errorChan)
		if len(msg) > 0 {
			// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
			//log.Warn(nil, map[string]interface{}{
			//	"action":        method.action,
			//	"object":        object.ToString(),
			//	"cluster":       client.MasterURL,
			//	"error-message": msg,
			//}, "unable to finish the action %s for an object as there were %d of unsuccessful retries to completely remove the objects from the cluster", method.action)
			fmt.Println(fmt.Sprintf("WARNING: unable to finish the action %s for an object %s as there were %d of unsuccessful retries "+
				"to completely remove the objects from the cluster %s. The retrieved errors:%s",
				method.action, object, retries, client.MasterURL, msg))
		}
		return nil
	},
	Name: TryToWaitUntilIsGoneName,
}

func isNotPresent(statusCode int) bool {
	return statusCode == http.StatusNotFound || statusCode == http.StatusForbidden
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
