package openshift_test

import (
	"testing"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"net/http"
	"net/url"
	"github.com/fabric8-services/fabric8-tenant/test"
	"fmt"
)

var originalRoleBindingRestrictionObject = `
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    labels:
      app: fabric8-tenant-che-mt
      provider: fabric8
      version: 2.0.85
      group: io.fabric8.tenant.packages
    name: dsaas-user-access
    namespace: john-run
  spec:
    userrestriction:
      users:
      - john@redhat.com
`

var modifiedRoleBindingRestrictionObject = `
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    labels:
      app: fabric8-tenant-che-mt
      provider: fabric8
      version: 2.0.85
      group: io.fabric8.tenant.packages
    name: dsaas-user-access
    namespace: john-run
  spec:
    userrestriction:
      users:
      - master-user
      - john@ibm-redhat.com
`

var tokenProducer = func(forceMasterToken bool) string {
	if forceMasterToken {
		return "master-token"
	}
	return "user-token"
}

func TestGetExistingObjectAndMerge(t *testing.T) {
	// given
	defer gock.Off()
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "PATCH")

	gock.New("https://starter.com").
		Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200).
		BodyString(originalRoleBindingRestrictionObject)

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

func TestGetMissingObjectAndMerge(t *testing.T) {
	// given
	defer gock.Off()
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "PATCH")

	gock.New("https://starter.com").
		Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
		Reply(404)

	// when
	methodDef, body, err := openshift.GetObjectAndMerge.Call(client, object, endpoints, methodDefinition)

	// then
	assert.NoError(t, err)
	postMethodDef, err := endpoints.GetMethodDefinition("POST", object)
	assert.NoError(t, err)
	assert.Equal(t, postMethodDef, methodDef)
	var actualObject environment.Object
	assert.NoError(t, yaml.Unmarshal(body, &actualObject))
	assert.Equal(t, object, actualObject)
}

// todo rethink when object is removed

func TestWhenNoConflictThenJustCheckResponseCode(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST")

	t.Run("original response is 200 and error is nil, so no error is returned", func(t *testing.T) {
		// given
		defer gock.Off()
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("original response is 404 and error is nil, so an error is returned", func(t *testing.T) {
		// given
		defer gock.Off()
		url, err := url.Parse("https://starter.com/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access")
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
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 404 for the request POST"))
	})

	t.Run("original response nil and error is not nil, so the same error is returned", func(t *testing.T) {
		// given
		defer gock.Off()
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
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST")

	t.Run("both delete and redo post is successful", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").
			Delete("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(200)
		gock.New("https://starter.com").
			Post("/oapi/v1/namespaces/john-run/rolebindingrestrictions").
			SetMatcher(test.ExpectRequest(test.HasObjectAsBody(object))).
			Reply(200)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when delete fails, then it returns an error", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").
			Delete("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(404)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err,
			test.HasMessageContaining("delete request failed while removing an object because of a conflict"),
			test.HasMessageContaining("server responded with status: 404 for the request DELETE"))
	})

	t.Run("when there is a second conflict while redoing the action, then it return an error and stops redoing", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").
			Delete("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(200)
		gock.New("https://starter.com").
			Post("/oapi/v1/namespaces/john-run/rolebindingrestrictions").
			SetMatcher(test.ExpectRequest(test.HasObjectAsBody(object))).
			Reply(409)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, nil)

		// when
		err := openshift.WhenConflictThenDeleteAndRedo.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err,
			test.HasMessageContaining("redoing an action POST failed after the object was successfully removed because of a previous conflict"),
			test.HasMessageContaining("server responded with status: 409 for the request POST"))
	})
}

func TestIgnoreConflicts(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST")

	t.Run("when there is a conflict, then it ignores it even if there is an error", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").Times(0)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusConflict}, []byte{}, fmt.Errorf("conflict"))

		// when
		err := openshift.IgnoreConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	t.Run("when there is no conflict but an error is not nil, then it returns the error", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").Times(0)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, fmt.Errorf("conflict"))

		// when
		err := openshift.IgnoreConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessage("conflict"))
	})

	t.Run("when there status code is 404, then it returns the an appropriate error", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").Times(0)
		url, err := url.Parse("https://starter.com/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access")
		require.NoError(t, err)
		result := openshift.NewResult(&http.Response{
			StatusCode: http.StatusNotFound,
			Request: &http.Request{
				Method: http.MethodPost,
				URL:    url,
			},
		}, []byte{}, nil)

		// when
		err = openshift.IgnoreConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 404 for the request POST"))
	})

	t.Run("when there is no conflict but and no error it returns nil", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").Times(0)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.IgnoreConflicts.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
	})

	assert.Equal(t, openshift.IgnoreConflictsName, openshift.IgnoreConflicts.Name)
}

func TestGetObject(t *testing.T) {
	// given
	client, object, endpoints, methodDefinition := getClientObjectEndpointAndMethod(t, "POST")

	t.Run("when returns 200, then it reads the object an checks status. everything is good, then return nil", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
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
		defer gock.Off()
		counter := 0
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Times(3).
			SetMatcher(test.SpyOnCalls(&counter)).
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction"`)
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction", "status": {"phase":"Active"}}`)
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, nil)

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		assert.NoError(t, err)
		assert.Equal(t, 3, counter)
	})

	t.Run("when returns 200, but with invalid body. then retries until everything is fine", func(t *testing.T) {
		// given
		defer gock.Off()
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(200).
			BodyString(`{"kind": "RoleBindingRestriction""`)
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
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
		defer gock.Off()
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
			Reply(404)
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
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
		defer gock.Off()
		gock.New("https://starter.com").
			Get("/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access").
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
		defer gock.Off()
		gock.New("https://starter.com").Times(0)
		url, err := url.Parse("https://starter.com/oapi/v1/namespaces/john-run/rolebindingrestrictions/dsaas-user-access")
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
		test.AssertError(t, err, test.HasMessageContaining("server responded with status: 404 for the request POST"))
	})

	t.Run("when there is an error in the result, then returns it", func(t *testing.T) {
		// given
		defer gock.Off()
		result := openshift.NewResult(&http.Response{StatusCode: http.StatusOK}, []byte{}, fmt.Errorf("error"))

		// when
		err := openshift.GetObject.Call(client, object, endpoints, methodDefinition, result)

		// then
		test.AssertError(t, err, test.HasMessage("error"))
	})

	assert.Equal(t, openshift.GetObjectName, openshift.GetObject.Name)
}

func getClientObjectEndpointAndMethod(t *testing.T, method string) (*openshift.Client, environment.Object, *openshift.ObjectEndpoints, *openshift.MethodDefinition) {
	client := openshift.NewClient(nil, "https://starter.com", tokenProducer)
	var object environment.Objects
	require.NoError(t, yaml.Unmarshal([]byte(modifiedRoleBindingRestrictionObject), &object))
	bindingEndpoints := openshift.AllObjectEndpoints["RoleBindingRestriction"]
	methodDefinition, err := bindingEndpoints.GetMethodDefinition(method, object[0])
	assert.NoError(t, err)
	return client, object[0], bindingEndpoints, methodDefinition
}
