package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/retry"
	"github.com/fabric8-services/fabric8-tenant/utils"
	ghodssYaml "github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"net/http"
	"strings"
	"time"
)

type BeforeDoCallback struct {
	Create BeforeDoCallbackFuncCreator
	Name   string
}

type AfterDoCallback struct {
	Create AfterDoCallbackFuncCreator
	Name   string
}

const (
	GetObjectAndMergeName             = "GetObjectAndMerge"
	FailIfExistsName                  = "FailIfAlreadyExists"
	WhenConflictThenDeleteAndRedoName = "WhenConflictThenDeleteAndRedo"
	GetObjectName                     = "GetObject"
	IgnoreWhenDoesNotExistName        = "IgnoreWhenDoesNotExistOrConflicts"
	TryToWaitUntilIsGoneName          = "TryToWaitUntilIsGone"
)

// Before callbacks
var GetObjectAndMerge = BeforeDoCallback{
	Create: func(previousCallback BeforeDoCallbackFunc) BeforeDoCallbackFunc {
		return func(context CallbackContext) (*MethodDefinition, []byte, error) {
			method, body, err := previousCallback(context)
			if err != nil {
				return method, body, err
			}
			retries := 10
			errorChan := retry.Do(retries, time.Second, func() error {
				result, err := context.ObjEndpoints.Apply(context.Client, context.Object, http.MethodGet)
				if result != nil && isNotPresent(result.Response.StatusCode) {
					method, err = context.ObjEndpoints.GetMethodDefinition(http.MethodPost, context.Object)
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
				body, err = marshalYAMLToJSON(context.Object)
				if err != nil {
					return errors.Wrapf(err, "unable marshal object to be send to OS as part of %s request", method.action)
				}
				return nil
			})

			msg := utils.ListErrorsInMessage(errorChan, 100)
			if len(msg) > 0 {
				return nil, nil, fmt.Errorf("unable to finish the action %s on a object %s as there were %d of unsuccessful retries "+
					"to get object and create a patch for the cluster %s. The retrieved errors:%s",
					method.action, context.Object, retries, context.Client.MasterURL, msg)
			}
			return method, body, nil
		}
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

var FailIfAlreadyExists = BeforeDoCallback{
	Create: func(previousCallback BeforeDoCallbackFunc) BeforeDoCallbackFunc {
		return func(context CallbackContext) (*MethodDefinition, []byte, error) {
			method, body, err := previousCallback(context)
			if err != nil {
				return method, body, err
			}
			masterClient := *context.Client
			masterClient.TokenProducer = func(forceMasterToken bool) string {
				return context.Client.TokenProducer(true)
			}

			result, err := context.ObjEndpoints.Apply(&masterClient, context.Object, http.MethodGet)
			if err != nil {
				if result != nil && isNotPresent(result.Response.StatusCode) {
					bodyToSend, err := yaml.Marshal(context.Object)
					if err != nil {
						return nil, nil, errors.Wrapf(err, "unable marshal object to be send to OS as part of %s request", method.action)
					}
					return method, bodyToSend, nil
				}
			}
			return nil, nil, fmt.Errorf("the object [%s] already exists", context.Object.ToString())

		}
	},
	Name: FailIfExistsName,
}

// After callbacks
var WhenConflictThenDeleteAndRedo = AfterDoCallback{
	Create: func(previousCallback AfterDoCallbackFunc) AfterDoCallbackFunc {
		return func(context CallbackContext) (*Result, error) {
			result, err := previousCallback(context)
			if result.Response != nil && result.Response.StatusCode == http.StatusConflict {
				// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
				logrus.WithFields(map[string]interface{}{
					"method":      context.Method.action,
					"object-kind": environment.GetKind(context.Object),
					"object-name": environment.GetName(context.Object),
					"namespace":   environment.GetNamespace(context.Object),
				}).Warnf("there was a conflict, trying to delete the object and re-do the operation")
				err := CheckHTTPCode(context.ObjEndpoints.Apply(context.Client, context.Object, http.MethodDelete))
				if err != nil {
					return result, errors.Wrap(err, "delete request failed while removing an object because of a conflict")
				}
				redoMethod := removeAfterDoCallback(*context.Method, WhenConflictThenDeleteAndRedoName)

				redoResult, err := context.ObjEndpoints.apply(context.Client, context.Object, redoMethod)
				err = CheckHTTPCode(redoResult, err)
				if err != nil {
					return redoResult, errors.Wrapf(err, "redoing an action %s failed after the object was successfully removed because of a previous conflict",
						context.Method.action)
				}
				return redoResult, nil
			}
			return result, err
		}
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

var GetObject = AfterDoCallback{
	Create: func(previousCallback AfterDoCallbackFunc) AfterDoCallbackFunc {
		return func(context CallbackContext) (*Result, error) {
			result, err := previousCallback(context)
			err = CheckHTTPCode(result, err)
			if err != nil {
				return result, err
			}
			retries := 50
			errorChan := retry.Do(retries, time.Millisecond*100, func() error {
				getResponse, err := context.ObjEndpoints.Apply(context.Client, context.Object, http.MethodGet)
				err = CheckHTTPCode(getResponse, err)
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
			msg := utils.ListErrorsInMessage(errorChan, 100)
			if len(msg) > 0 {
				return result, fmt.Errorf("unable to finish the action %s on a object %s as there were %d of unsuccessful retries "+
					"to get the created objects from the cluster %s. The retrieved errors:%s",
					context.Method.action, context.Object, retries, context.Client.MasterURL, msg)
			}
			return result, nil
		}
	},
	Name: GetObjectName,
}

var IgnoreWhenDoesNotExistOrConflicts = AfterDoCallback{
	Create: func(previousCallback AfterDoCallbackFunc) AfterDoCallbackFunc {
		return func(context CallbackContext) (*Result, error) {
			result, err := previousCallback(context)
			code := result.Response.StatusCode
			if code == http.StatusNotFound || code == http.StatusConflict {
				// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
				logrus.WithFields(map[string]interface{}{
					"action":      context.Method.action,
					"status":      result.Response.Status,
					"object-kind": environment.GetKind(context.Object),
					"object-name": environment.GetName(context.Object),
					"namespace":   environment.GetNamespace(context.Object),
					"message":     result.Body,
				}).Warnf("failed to %s the object. Ignoring this error because it probably does not exist or is being removed",
					context.Method.action)
				return &Result{}, nil
			}
			return result, err
		}
	},
	Name: IgnoreWhenDoesNotExistName,
}

var TryToWaitUntilIsGone = AfterDoCallback{
	Create: func(previousCallback AfterDoCallbackFunc) AfterDoCallbackFunc {
		return func(context CallbackContext) (*Result, error) {
			result, err := previousCallback(context)
			err = CheckHTTPCode(result, err)
			if err != nil {
				return result, err
			}
			retries := 60
			errorChan := retry.Do(retries, time.Millisecond*500, func() error {
				result, err := context.ObjEndpoints.Apply(context.Client, context.Object, http.MethodGet)
				if result != nil && isNotPresent(result.Response.StatusCode) {
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
					"- waiting till it is completely removed to finish the %s action", returnedObj, context.Method.action)
			})
			msg := utils.ListErrorsInMessage(errorChan, 5)
			if len(msg) > 0 {
				// todo investigate why logging here ends with panic: runtime error: index out of range in common logic
				logrus.WithFields(map[string]interface{}{
					"action":         context.Method.action,
					"object-kind":    environment.GetKind(context.Object),
					"object-name":    environment.GetName(context.Object),
					"namespace":      environment.GetNamespace(context.Object),
					"cluster":        context.Client.MasterURL,
					"first-5-errors": msg,
				}).Warnf("unable to finish the action %s for an object as there were %d of unsuccessful retries to completely remove the objects from the cluster",
					context.Method.action, retries)
			}
			return result, nil
		}
	},
	Name: TryToWaitUntilIsGoneName,
}

func isNotPresent(statusCode int) bool {
	return statusCode == http.StatusNotFound || statusCode == http.StatusForbidden
}

func CheckHTTPCode(result *Result, e error) error {
	if e == nil && result.Response != nil && (result.Response.StatusCode < 200 || result.Response.StatusCode >= 300) {
		return fmt.Errorf("server responded with status: %d for the %s request %s with the Body [%s]",
			result.Response.StatusCode, result.Response.Request.Method, result.Response.Request.URL, string(result.Body))
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
