package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"gopkg.in/yaml.v2"
)

// ObjectEndpoints is list of MethodDefinitions for a particular object endpoint (eg. `/oapi/v1/projectrequests`).
// In other words, is saying which methods (Post/Delete/Get/Patch) are allowed to be performed for the endpoint
type ObjectEndpoints struct {
	Methods map[string]MethodDefinition
}

var (
	AllObjectEndpoints = map[string]*ObjectEndpoints{
		environment.ValKindNamespace: endpoints(
			endpoint(`/api/v1/namespaces`, POST(BeforeDo(FailIfAlreadyExists), AfterDo(GetObject))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindProject: endpoints(
			endpoint(`/oapi/v1/projects`, POST(BeforeDo(FailIfAlreadyExists), AfterDo(GetObject))),
			endpoint(`/oapi/v1/projects/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindProjectRequest: endpoints(
			endpoint(`/oapi/v1/projectrequests`, POST(BeforeDo(FailIfAlreadyExists), AfterDo(GetObject))),
			endpoint(`/oapi/v1/projects/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindRole: endpoints(
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/roles/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindRoleBinding: endpoints(
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindings/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE(Require(MasterToken)))),

		environment.ValKindRoleBindingRestriction: endpoints(
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions`, POST(Require(MasterToken), AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/rolebindingrestrictions/{{ index . "metadata" "name"}}`,
				PATCH(Require(MasterToken)), GET(Require(MasterToken)), DELETE(Require(MasterToken)))),

		environment.ValKindRoute: endpoints(
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/oapi/v1/namespaces/{{ index . "metadata" "namespace"}}/routes/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindDeployment: endpoints(
			endpoint(`/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/apis/extensions/v1beta1/namespaces/{{ index . "metadata" "namespace"}}/deployments/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindDeploymentConfig: endpoints(
			endpoint(`/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/apis/apps.openshift.io/v1/namespaces/{{ index . "metadata" "namespace"}}/deploymentconfigs/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindPersistenceVolumeClaim: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/persistentvolumeclaims/{{ index . "metadata" "name"}}`,
				PATCH(), GET(), DELETE(AfterDo(TryToWaitUntilIsGone)))),

		environment.ValKindService: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/services/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindSecret: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/secrets/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindServiceAccount: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/serviceaccounts/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindConfigMap: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/configmaps/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindResourceQuota: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas`, POST(AfterDo(WhenConflictThenDeleteAndRedo, GetObject))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/resourcequotas/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindLimitRange: endpoints(
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/api/v1/namespaces/{{ index . "metadata" "namespace"}}/limitranges/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),

		environment.ValKindJob: endpoints(
			endpoint(`/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs`, POST(AfterDo(WhenConflictThenDeleteAndRedo))),
			endpoint(`/apis/batch/v1/namespaces/{{ index . "metadata" "namespace"}}/jobs/{{ index . "metadata" "name"}}`, PATCH(), GET(), DELETE())),
	}
	deleteOptions = `apiVersion: v1
kind: DeleteOptions
gracePeriodSeconds: 0
orphanDependents: false`

	adminRole = `apiVersion: v1
kind: RoleBinding
metadata:
  name: admin`
)

func endpoint(endpoint string, methodsDefCreators ...methodDefCreator) func(methods map[string]MethodDefinition) {
	return func(methods map[string]MethodDefinition) {
		for _, methodDefCreator := range methodsDefCreators {
			methodDef := methodDefCreator(endpoint)
			methods[methodDef.action] = methodDef
		}
	}
}

func endpoints(endpoints ...func(methods map[string]MethodDefinition)) *ObjectEndpoints {
	methods := make(map[string]MethodDefinition)
	for _, endpoint := range endpoints {
		endpoint(methods)
	}
	return &ObjectEndpoints{Methods: methods}
}

func (r *Result) bodyToObject() (environment.Object, error) {
	var obj environment.Object
	err := yaml.Unmarshal(r.Body, &obj)
	return obj, err
}

func (e *ObjectEndpoints) Apply(client *Client, object environment.Object, action string) (*Result, error) {
	// get method definition for the object
	method, err := e.GetMethodDefinition(action, object)
	if err != nil {
		return nil, err
	}
	return e.apply(client, object, method)
}

func (e *ObjectEndpoints) apply(client *Client, object environment.Object, method *MethodDefinition) (*Result, error) {
	var (
		reqBody []byte
		result  *Result
		err     error
	)

	// handle before callbacks if any defined (that could change the request Body)
	if len(method.beforeDoCallbacks) != 0 {
		for _, beforeCallback := range method.beforeDoCallbacks {
			method, reqBody, err = beforeCallback.Call(client, object, e, method)
			if err != nil {
				return nil, err
			}
		}
	} else {
		reqBody, err = yaml.Marshal(object)
		if err != nil {
			return nil, err
		}
	}

	// do the request
	result, err = client.Do(method.requestCreator, object, reqBody)

	// if error occurred and no response was retrieved (probably error before doing a request)
	logParams := logParams(object, method, result)
	if err != nil && result == nil {
		logParams["requestObject"] = yamlString(object)
		logParams["err"] = err
		log.Error(nil, logParams, "unable request resource")
		return result, err
	}
	log.Info(nil, logParams, "resource requested")

	// handle after callbacks and let them handle errors in their way
	if len(method.afterDoCallbacks) == 0 {
		// if none, then just check the error code and return any possible error
		return result, checkHTTPCode(result, err)
	} else {
		for _, afterCallback := range method.afterDoCallbacks {
			err := afterCallback.Call(client, object, e, method, result)
			if err != nil {
				return result, err
			}
		}
		return result, nil
	}
}

func (e *ObjectEndpoints) GetMethodDefinition(method string, object environment.Object) (*MethodDefinition, error) {
	methodDef, found := e.Methods[method]
	if !found {
		return nil, fmt.Errorf("method definition %s for %s not supported", method, environment.GetKind(object))
	}
	return &methodDef, nil
}

func logParams(object environment.Object, method *MethodDefinition, result *Result) map[string]interface{} {
	var status, reqURL string
	if result != nil && result.response != nil {
		status = result.response.Status
		if result.response.Request != nil {
			reqURL = result.response.Request.URL.String()
		}
	}
	return map[string]interface{}{
		"status":      status,
		"method":      method.action,
		"request_url": reqURL,
		"namespace":   environment.GetNamespace(object),
		"name":        environment.GetName(object),
		"kind":        environment.GetKind(object),
	}
}
