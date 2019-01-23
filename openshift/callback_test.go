package openshift_test

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/url"
	"testing"
)

var boundPVC = `{"kind":"PersistentVolumeClaim","apiVersion":"v1","metadata":{"name":"jenkins-home","namespace":"john-jenkins",
"selfLink":"/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home","uid":"e7c571fa-1598-11e9-aef5-525400d75155",
"resourceVersion":"360049","creationTimestamp":"2019-01-11T12:03:27Z","labels":{"app":"jenkins","provider":"fabric8","version":"123abc",
"version-quotas":"123abc"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"PersistentVolumeClaim\",
\"metadata\":{\"annotations\":{},\"labels\":{\"app\":\"jenkins\",\"provider\":\"fabric8\",\"version\":\"123abc\",\"version-quotas\":\"123abc\"},
\"name\":\"jenkins-home\",\"namespace\":\"john-jenkins\"},\"spec\":{\"accessModes\":[\"ReadWriteOnce\"],
\"resources\":{\"requests\":{\"storage\":\"1Gi\"}}}}\n","pv.kubernetes.io/bind-completed":"yes","pv.kubernetes.io/bound-by-controller":"yes"}},
"spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"1Gi"}},"volumeName":"pv0052"},"status":{"phase":"Bound",
"accessModes":["ReadWriteOnce","ReadWriteMany","ReadOnlyMany"],"capacity":{"storage":"100Gi"}}}`

var terminatingPVC = `{"kind":"PersistentVolumeClaim","apiVersion":"v1","metadata":{"name":"jenkins-home","namespace":"john-jenkins",
"selfLink":"/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home","uid":"e7c571fa-1598-11e9-aef5-525400d75155",
"resourceVersion":"360049","creationTimestamp":"2019-01-11T12:03:27Z","labels":{"app":"jenkins","provider":"fabric8","version":"123abc",
"version-quotas":"123abc"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"PersistentVolumeClaim\",
\"metadata\":{\"annotations\":{},\"labels\":{\"app\":\"jenkins\",\"provider\":\"fabric8\",\"version\":\"123abc\",\"version-quotas\":\"123abc\"},
\"name\":\"jenkins-home\",\"namespace\":\"john-jenkins\"},\"spec\":{\"accessModes\":[\"ReadWriteOnce\"],
\"resources\":{\"requests\":{\"storage\":\"1Gi\"}}}}\n","pv.kubernetes.io/bind-completed":"yes","pv.kubernetes.io/bound-by-controller":"yes"}},
"spec":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"1Gi"}},"volumeName":"pv0052"},"status":{"phase":"Terminating",
"accessModes":["ReadWriteOnce","ReadWriteMany","ReadOnlyMany"],"capacity":{"storage":"100Gi"}}}`

var pvcToSet = `
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    labels:
      app: jenkins
      provider: fabric8
      version: 123
      version-quotas: 456
    name: jenkins-home
    namespace: john-jenkins
  spec:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
    userrestriction:
      users:
      - master-user
      - john@ibm-redhat.com
`

var projectRequestJenkins = `- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: john Jenkins Environment
      openshift.io/display-name: john Jenkins
      openshift.io/requester: john
    labels:
      app: fabric8-tenant-jenkins
      provider: fabric8
      version: 123
      version-quotas: john
    name: john-jenkins
`

var projectRequestUser = `- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: john Environment
      openshift.io/display-name: john
      openshift.io/requester: john
    labels:
      app: fabric8-tenant
      provider: fabric8
      version: 123
      version-quotas: john
    name: john
`

var tokenProducer = func(forceMasterToken bool) string {
	if forceMasterToken {
		return "master-token"
	}
	return "user-token"
}

func TestGetExistingObjectAndMerge(t *testing.T) {
	// given
	defer gock.OffAll()
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "PATCH", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	gock.New("https://starter.com").
		Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
		Reply(200).
		BodyString(boundPVC)

	// when
	methodDef, body, err := openshift.GetObjectAndMerge.Call(client, object, endpoints, methodDefinition)

	// then
	assert.NoError(t, err)
	assert.Equal(t, methodDefinition, methodDef)
	var actualObject environment.Object
	assert.NoError(t, yaml.Unmarshal(body, &actualObject))
	assert.Equal(t, object, actualObject)
	assert.Equal(t, openshift.GetObjectAndMergeName, openshift.GetObjectAndMerge.Name)
}

func TestGetExistingObjectAndWaitTillIsNotTerminating(t *testing.T) {
	// given
	defer gock.OffAll()
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "PATCH", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	terminatingCalls := 0
	gock.New("https://starter.com").
		Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
		SetMatcher(test.SpyOnCalls(&terminatingCalls)).
		Reply(200).
		BodyString(terminatingPVC)
	boundCalls := 0
	gock.New("https://starter.com").
		Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
		SetMatcher(test.SpyOnCalls(&boundCalls)).
		Reply(200).
		BodyString(boundPVC)

	// when
	methodDef, body, err := openshift.GetObjectAndMerge.Call(client, object, endpoints, methodDefinition)

	// then
	assert.NoError(t, err)
	assert.Equal(t, methodDefinition, methodDef)
	var actualObject environment.Object
	assert.NoError(t, yaml.Unmarshal(body, &actualObject))
	assert.Equal(t, object, actualObject)
	assert.Equal(t, openshift.GetObjectAndMergeName, openshift.GetObjectAndMerge.Name)
	assert.Equal(t, 1, terminatingCalls)
	assert.Equal(t, 1, boundCalls)
}

func TestGetMissingObjectAndMerge(t *testing.T) {
	// given
	defer gock.OffAll()
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "PATCH", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	gock.New("https://starter.com").
		Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
		Reply(404)

	// when
	methodDef, body, err := openshift.GetObjectAndMerge.Call(client, object, endpoints, methodDefinition)

	// then
	assert.NoError(t, err)
	postMethodDef, err := endpoints.GetMethodDefinition("POST", object)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%+v", *postMethodDef), fmt.Sprintf("%+v", *methodDef))
	var actualObject environment.Object
	assert.NoError(t, yaml.Unmarshal(body, &actualObject))
	assert.Equal(t, object, actualObject)
}

func TestWhenNoConflictThenJustCheckResponseCode(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	t.Run("original response is 200 and error is nil, so no error is returned", func(t *testing.T) {
		// given
		defer gock.OffAll()
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("original response is 404 and error is nil, so an error is returned", func(t *testing.T) {
		// given
		defer gock.OffAll()
		url, err := url.Parse("https://starter.com/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home")
		require.NoError(t, err)
		result := openshift.NewResult(&http.Response{
			StatusCode: http.StatusNotFound,
			Request: &http.Request{
				Method: http.MethodPost,
				URL:    url,
			},
		}, []byte{}, nil)

		// when
		err = openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 404 for the POST request"))
	})

	t.Run("original response nil and error is not nil, so the same error is returned", func(t *testing.T) {
		// given
		defer gock.OffAll()
		expErr := fmt.Errorf("unexpected format")
		result := openshift.NewResult(nil, []byte{}, expErr)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.Equal(t, expErr, err)
	})
	assert.Equal(t, openshift.WhenConflictThenDeleteAndRedoName, openshift.WhenConflictThenDeleteAndRedo.Name)
}

func TestWhenConflictThenDeleteAndRedoAction(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	t.Run("both delete and redo post is successful", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Delete("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)
		gock.New("https://starter.com").
			Post("/api/v1/namespaces/john-jenkins/persistentvolumeclaims").
			SetMatcher(test.ExpectRequest(test.HasBodyContainingObject(object))).
			Reply(200)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when delete fails, then it returns an error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Delete("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err,
			test.HasMessageContaining("delete request failed while removing an object because of a conflict"),
			test.HasMessageContaining("server responded with status: 404 for the DELETE request"))
	})

	t.Run("when there is a second conflict while redoing the action, then it return an error and stops redoing", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Delete("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)
		gock.New("https://starter.com").
			Post("/api/v1/namespaces/john-jenkins/persistentvolumeclaims").
			SetMatcher(test.ExpectRequest(test.HasBodyContainingObject(object))).
			Reply(409)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err,
			test.HasMessageContaining("redoing an action POST failed after the object was successfully removed because of a previous conflict"),
			test.HasMessageContaining("server responded with status: 409 for the POST request"))
	})
}

func TestIgnoreWhenDoesNotExist(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "DELETE", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	t.Run("when there is 404, then it ignores it even if there is an error", func(t *testing.T) {
		// given
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusNotFound}, []byte{}, fmt.Errorf("not found"))

		// when
		err := openshift.IgnoreWhenDoesNotExistOrConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when there is 409, then it ignores it even if there is an error", func(t *testing.T) {
		// given
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, fmt.Errorf("conflict"))

		// when
		err := openshift.IgnoreWhenDoesNotExistOrConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when code is 200 but an error is not nil, then it returns the error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").Times(0)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, fmt.Errorf("wrong request"))

		// when
		err := openshift.IgnoreWhenDoesNotExistOrConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessage("wrong request"))
	})

	t.Run("when there status code is 500, then it returns the an appropriate error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").Times(0)
		url, err := url.Parse("https://starter.com/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home")
		require.NoError(t, err)
		result := openshift.NewResult(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: http.MethodDelete,
				URL:    url,
			},
		}, []byte{}, nil)

		// when
		err = openshift.IgnoreWhenDoesNotExistOrConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 500 for the DELETE request"))
	})

	t.Run("when the status code is 200 and no error then it returns nil", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").Times(0)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.IgnoreWhenDoesNotExistOrConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	assert.Equal(t, openshift.IgnoreWhenDoesNotExistName, openshift.IgnoreWhenDoesNotExistOrConflicts.Name)
}

func TestGetObject(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST", environment.ValKindPersistenceVolumeClaim, pvcToSet)

	t.Run("when returns 200, then it reads the object an checks status. everything is good, then return nil", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction", "status": {"phase":"Active"}}`)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when returns 200, then it reads the object an checks status. when is missing then retries until is present", func(t *testing.T) {
		// given
		defer gock.OffAll()
		counter := 0
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Times(3).
			SetMatcher(test.SpyOnCalls(&counter)).
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction"`)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction", "status": {"phase":"Active"}}`)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
		assert.Equal(t, 3, counter)
	})

	t.Run("when returns 200, but with invalid Body. then retries until everything is fine", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction""`)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction", "status": {"phase":"Active"}}`)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when returns 404, then retries until everything is fine", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction", "status": {"phase":"Active"}}`)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when always returns 404 then after 50 attempts it returns error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Times(50).
			Reply(404)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessageContaining("unable to finish the action POST on a object"),
			test.HasMessageContaining("as there were 50 of unsuccessful retries to get the created objects from the cluster https://starter.com"))
	})

	t.Run("when there status code is 404, then it returns the an appropriate error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").Times(0)
		url, err := url.Parse("https://starter.com/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home")
		require.NoError(t, err)
		result := openshift.NewResult(&http.Response{
			StatusCode: http.StatusNotFound,
			Request: &http.Request{
				Method: http.MethodPost,
				URL:    url,
			},
		}, []byte{}, nil)

		// when
		err = openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 404 for the POST request"))
	})

	t.Run("when there is an error in the result, then returns it", func(t *testing.T) {
		// given
		defer gock.OffAll()
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, fmt.Errorf("error"))

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessage("error"))
	})

	assert.Equal(t, openshift.GetObjectName, openshift.GetObject.Name)
}

func TestFailIfAlreadyExists(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST", environment.ValKindProjectRequest, projectRequestJenkins)

	t.Run("when returns 200, then it returns error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/oapi/v1/projects/john-jenkins").
			SetMatcher(test.ExpectRequest(test.HasBearerWithSub("master-token"))).
			Reply(200).
			BodyString(``)

		// when
		methodDef, body, err := openshift.FailIfAlreadyExists.Call(client, object, endpoints, methodDefinition)

		// then
		test.AssertError(t, err, test.HasMessageContaining("already exists"))
		assert.Nil(t, methodDef)
		assert.Nil(t, body)
	})

	t.Run("when returns 404, then it should return original method and body", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/oapi/v1/projects/john-jenkins").
			SetMatcher(test.ExpectRequest(test.HasBearerWithSub("master-token"))).
			Reply(404).
			BodyString(``)

		// when
		actualMethodDef, body, err := openshift.FailIfAlreadyExists.Call(client, object, endpoints, methodDefinition)

		// then
		require.NoError(t, err)
		assert.Equal(t, methodDefinition, actualMethodDef)
		assert.Contains(t, string(body), "name: john-jenkins")
	})

	t.Run("when returns 403, then it should return original method and body", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/oapi/v1/projects/john-jenkins").
			SetMatcher(test.ExpectRequest(test.HasBearerWithSub("master-token"))).
			Reply(403).
			BodyString(``)

		// when
		actualMethodDef, body, err := openshift.FailIfAlreadyExists.Call(client, object, endpoints, methodDefinition)

		// then
		require.NoError(t, err)
		assert.Equal(t, methodDefinition, actualMethodDef)
		assert.Contains(t, string(body), "name: john-jenkins")
	})
}

func TestFailIfAlreadyExistsForUserNamespaceShouldUseMasterToken(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST", environment.ValKindProjectRequest, projectRequestUser)

	t.Run("when returns 200, then it returns error", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/oapi/v1/projects/john").
			SetMatcher(test.ExpectRequest(test.HasBearerWithSub("master-token"))).
			Reply(200).
			BodyString(``)

		// when
		methodDef, body, err := openshift.FailIfAlreadyExists.Call(client, object, endpoints, methodDefinition)

		// then
		test.AssertError(t, err, test.HasMessageContaining("already exists"))
		assert.Nil(t, methodDef)
		assert.Nil(t, body)
	})

	t.Run("when returns 404, then it should return original method and body", func(t *testing.T) {
		// given
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/oapi/v1/projects/john").
			SetMatcher(test.ExpectRequest(test.HasBearerWithSub("master-token"))).
			Reply(404).
			BodyString(``)

		// when
		actualMethodDef, body, err := openshift.FailIfAlreadyExists.Call(client, object, endpoints, methodDefinition)

		// then
		require.NoError(t, err)
		assert.Equal(t, methodDefinition, actualMethodDef)
		assert.Contains(t, string(body), "name: john")
	})
}

func TestWaitUntilIsGone(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "DELETE", environment.ValKindPersistenceVolumeClaim, pvcToSet)
	result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

	t.Run("wait until is in terminating state", func(t *testing.T) {
		defer gock.OffAll()
		terminatingCalls := 0
		boundCalls := 0
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			SetMatcher(test.SpyOnCalls(&boundCalls)).
			Times(2).
			Reply(200).
			BodyString(boundPVC)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			SetMatcher(test.SpyOnCalls(&terminatingCalls)).
			Reply(200).
			BodyString(terminatingPVC)

		// when
		err := openshift.TryToWaitUntilIsGone.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
		assert.Equal(t, openshift.TryToWaitUntilIsGoneName, openshift.TryToWaitUntilIsGone.Name)
		assert.Equal(t, 1, terminatingCalls)
		assert.Equal(t, 2, boundCalls)
	})

	t.Run("wait until is it returns 404", func(t *testing.T) {
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Times(2).
			Reply(200).
			BodyString(boundPVC)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(404)

		// when
		err := openshift.TryToWaitUntilIsGone.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
		assert.Equal(t, openshift.TryToWaitUntilIsGoneName, openshift.TryToWaitUntilIsGone.Name)
	})

	t.Run("wait until is it returns 403", func(t *testing.T) {
		defer gock.OffAll()
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(200).
			BodyString(boundPVC)
		gock.New("https://starter.com").
			Get("/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home").
			Reply(403)

		// when
		err := openshift.TryToWaitUntilIsGone.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("if gets result with 500, then returns an error", func(t *testing.T) {
		// given
		url, err := url.Parse("https://starter.com/api/v1/namespaces/john-jenkins/persistentvolumeclaims/jenkins-home")
		require.NoError(t, err)
		failingResult := openshift.NewResult(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Request: &http.Request{
				Method: http.MethodPost,
				URL:    url,
			},
		}, []byte{}, nil)

		// when
		err = openshift.TryToWaitUntilIsGone.Call(client, object, endpoints, methodDefinition, failingResult)

		// then
		test.AssertError(t, err,
			test.HasMessageContaining("server responded with status: 500 for the POST request"))
	})
}

func getClientObjectEndpointAndMethod(t *testing.T, method, kind, response string) (*openshift.Client, environment.Object, *openshift.ObjectEndpoints, *openshift.MethodDefinition) {
	client := openshift.NewClient(nil, "https://starter.com", tokenProducer)
	var object environment.Objects
	require.NoError(t, yaml.Unmarshal([]byte(response), &object))
	bindingEndpoints := openshift.AllObjectEndpoints[kind]
	methodDefinition, err := bindingEndpoints.GetMethodDefinition(method, object[0])
	assert.NoError(t, err)
	return client, object[0], bindingEndpoints, methodDefinition
}
