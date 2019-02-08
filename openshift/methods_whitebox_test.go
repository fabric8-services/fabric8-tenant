package openshift

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	"net/http"
	"regexp"
	"testing"
)

var objectToBeParsed = `
- apiVersion: v1
  metadata:
    name: targeting-object-name
    namespace: targeting-object-name
`
var dummyEndpoint = `/{{ index . "metadata" "name"}}`

func TestAllEndpointsToBeConsumedByAllMethods(t *testing.T) {
	//given
	var object environment.Objects
	err := yaml.Unmarshal([]byte(objectToBeParsed), &object)
	require.NoError(t, err)

	// when
	for _, endpoint := range AllObjectEndpoints {
		for _, methodDefinition := range endpoint.Methods {
			// then
			request, err := methodDefinition.requestCreator.createRequestFor("http://starter", object[0], []byte(objectToBeParsed))
			assert.NoError(t, err)
			url := request.URL.String()
			if !regexp.MustCompile(`http:\/\/starter\/(o)?api\/v1\/(projectrequests|projects|namespaces)`).MatchString(url) {
				assert.Regexp(t, regexp.MustCompile(`http:\/\/starter\/.*targeting-object-name\/*`), url)
			}
		}
	}
}

func TestEachMethodSeparately(t *testing.T) {
	//given
	var object environment.Objects
	err := yaml.Unmarshal([]byte(objectToBeParsed), &object)
	require.NoError(t, err)

	t.Run("POST method", func(t *testing.T) {
		// when
		methodDefinition := POST()(dummyEndpoint)

		// then
		assert.Empty(t, methodDefinition.beforeDoCallbacks)
		assert.Empty(t, methodDefinition.afterDoCallbacks)
		assert.Equal(t, http.MethodPost, methodDefinition.action)
		request, err := methodDefinition.requestCreator.createRequestFor("http://starter", object[0], []byte(objectToBeParsed))
		assert.NoError(t, err)
		assert.Equal(t, "http://starter/targeting-object-name", request.URL.String())
		assert.Equal(t, http.MethodPost, request.Method)
	})

	t.Run("PATCH method", func(t *testing.T) {
		// when
		methodDefinition := PATCH()(dummyEndpoint)

		// then
		assert.Len(t, methodDefinition.beforeDoCallbacks, 1)
		assert.Equal(t, methodDefinition.beforeDoCallbacks[0].Name, GetObjectAndMerge.Name)
		assert.Empty(t, methodDefinition.afterDoCallbacks)
		assert.Equal(t, http.MethodPatch, methodDefinition.action)
		request, err := methodDefinition.requestCreator.createRequestFor("http://starter", object[0], []byte(objectToBeParsed))
		assert.NoError(t, err)
		assert.Equal(t, "http://starter/targeting-object-name", request.URL.String())
		assert.Equal(t, "application/strategic-merge-patch+json", request.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPatch, request.Method)
	})

	t.Run("DELETE method", func(t *testing.T) {
		// when
		methodDefinition := DELETE()(dummyEndpoint)

		// then
		assert.Empty(t, methodDefinition.beforeDoCallbacks)
		assert.Len(t, methodDefinition.afterDoCallbacks, 1)
		assert.Equal(t, methodDefinition.afterDoCallbacks[0].Name, IgnoreWhenDoesNotExistOrConflicts.Name)
		assert.Equal(t, http.MethodDelete, methodDefinition.action)
		request, err := methodDefinition.requestCreator.createRequestFor("http://starter", object[0], []byte(objectToBeParsed))
		assert.NoError(t, err)
		assert.Equal(t, "http://starter/targeting-object-name", request.URL.String())
		assert.Equal(t, http.MethodDelete, request.Method)
	})

	t.Run("GET method", func(t *testing.T) {
		// when
		methodDefinition := GET()(dummyEndpoint)

		// then
		assert.Empty(t, methodDefinition.beforeDoCallbacks)
		assert.Empty(t, methodDefinition.afterDoCallbacks)
		assert.Equal(t, http.MethodGet, methodDefinition.action)
		request, err := methodDefinition.requestCreator.createRequestFor("http://starter", object[0], []byte(objectToBeParsed))
		assert.NoError(t, err)
		assert.Equal(t, "http://starter/targeting-object-name", request.URL.String())
		assert.Equal(t, http.MethodGet, request.Method)
	})
}

func TestNeedMasterTokenModifier(t *testing.T) {
	for _, method := range []methodDefCreator{POST(), PATCH(), DELETE(), GET()} {
		t.Run("needMasterToken is false when modifier is not called", func(t *testing.T) {
			// when
			methodDef := method(dummyEndpoint)
			// then
			assert.False(t, methodDef.requestCreator.needMasterToken)
		})

	}
	for _, method := range []methodDefCreator{POST(Require(MasterToken)), PATCH(Require(MasterToken)), DELETE(Require(MasterToken)), GET(Require(MasterToken))} {
		t.Run("needMasterToken is true when modifier is called", func(t *testing.T) {
			// when
			methodDef := method(dummyEndpoint)
			// then
			assert.True(t, methodDef.requestCreator.needMasterToken)
		})
	}
}
