package environment

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"testing"
)

var defaultLocationTempl = `apiVersion: v1
kind: Template
metadata:
  name: fabric8-tenant-${DEPLOY_TYPE}
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    labels:
      test: default-location
      version: ${COMMIT}
    name: ${USER_NAME}-${DEPLOY_TYPE}`

var customLocationTempl = `apiVersion: v1
kind: Template
metadata:
  name: fabric8-tenant-${DEPLOY_TYPE}
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    labels:
      test: custom-location
      version: ${COMMIT}
    name: ${USER_NAME}-${DEPLOY_TYPE}`

func TestGetAllTemplatesForAllTypes(t *testing.T) {
	// given
	service := NewService("", "", "")

	for _, envType := range DefaultEnvTypes {
		// when
		env, err := service.GetEnvData(context.Background(), envType)

		// then
		require.NoError(t, err)
		assert.Equal(t, env.Name, envType)
		if envType == "che" || envType == "jenkins" {
			assert.Len(t, env.Templates, 2)
			assert.Contains(t, env.Templates[0].Filename, envType)
			assert.Contains(t, env.Templates[1].Filename, "quotas")
		} else if envType == "user" {
			assert.Len(t, env.Templates, 1)
			assert.Contains(t, env.Templates[0].Filename, envType)
		} else {
			assert.Len(t, env.Templates, 1)
			assert.Contains(t, env.Templates[0].Filename, "deploy")
		}

		for _, template := range env.Templates {
			assert.NotEmpty(t, template.Content)
		}
	}
}

func TestAllTemplatesHaveNecessaryData(t *testing.T) {
	// given
	service := NewService("", "", "")
	vars := map[string]string{
		"USER_NAME": "dev",
		"COMMIT":    "123",
	}

	for _, envType := range DefaultEnvTypes {
		nsName := "dev-" + envType
		if envType == string(tenant.TypeUser) {
			nsName = "dev"
		}

		// when
		env, err := service.GetEnvData(context.Background(), envType)
		require.NoError(t, err)
		objects, err := env.Templates[0].Process(vars)
		require.NoError(t, err)

		//then
		for _, obj := range objects {
			assert.Equal(t, "123", GetLabelVersion(obj))
			if GetKind(obj) != ValKindProjectRequest {
				assert.Contains(t, GetNamespace(obj), nsName)
			} else {
				assert.Contains(t, GetName(obj), nsName)
			}
		}
	}
}

func TestDownloadFromGivenBlob(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("https://github.com").
		Get("fabric8-services/fabric8-tenant/blob/123abc/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(defaultLocationTempl)

	service := NewService("", "123abc", "")

	// when
	envData, err := service.GetEnvData(context.Background(), "run")

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
		"COMMIT":    "123",
	}
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, GetLabel(objects[0], "test"), "default-location")
}

func TestDownloadFromGivenBlobLocatedInCustomLocation(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("http://my.git.com").
		Get("my-services/my-tenant/blob/123abc/any/path/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(customLocationTempl)

	service := NewService("http://my.git.com/my-services/my-tenant", "123abc", "any/path")

	// when
	envData, err := service.GetEnvData(context.Background(), "run")

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
		"COMMIT":    "123",
	}
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, GetLabel(objects[0], "test"), "custom-location")
}

var dnsRegExp = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

func TestCreateUsername(t *testing.T) {
	assertName(t, "some", "some@email.com")
	assertName(t, "so-me", "so-me@email.com")
	assertName(t, "some", "some")
	assertName(t, "so-me", "so-me")
	assertName(t, "so-me", "so_me")
	assertName(t, "so-me", "so me")
	assertName(t, "so-me", "so me@email.com")
	assertName(t, "so-me", "so.me")
	assertName(t, "so-me", "so?me")
	assertName(t, "so-me", "so:me")
	assertName(t, "some1", "some1")
	assertName(t, "so1me1", "so1me1")
}

func assertName(t *testing.T, expected, username string) {
	assert.Regexp(t, dnsRegExp, RetrieveUserName(username))
	assert.Equal(t, expected, RetrieveUserName(username))
}

func TestRetrieveCheMtParams(t *testing.T) {
	// given
	sub := uuid.NewV4().String()
	token, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": sub,
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	cheMtParams, err := getCheMtParams(ctx)

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, cheMtParams["JOB_ID"])
	assert.Equal(t, token.Raw, cheMtParams["OSIO_TOKEN"])
	assert.Equal(t, sub, cheMtParams["IDENTITY_ID"])
	assert.Empty(t, cheMtParams["REQUEST_ID"])
}

func TestRetrieveCheMtParamsShouldFailIfMissingSub(t *testing.T) {
	// given
	token, err := testsupport.NewToken(
		map[string]interface{}{},
		"../test/private_key.pem",
	)
	require.NoError(t, err)
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	_, err = getCheMtParams(ctx)

	// then
	testsupport.AssertError(t, err, testsupport.HasMessage("missing sub in JWT token"))
}

func TestRetrieveCheMtParamsWhenTokenIsMissing(t *testing.T) {
	// when
	cheMtParams, err := getCheMtParams(context.Background())

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, cheMtParams["JOB_ID"])
	assert.Empty(t, cheMtParams["OSIO_TOKEN"])
	assert.Empty(t, cheMtParams["IDENTITY_ID"])
	assert.Empty(t, cheMtParams["REQUEST_ID"])
}
