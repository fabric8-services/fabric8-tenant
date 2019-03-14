package environment_test

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"regexp"
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
  name: fabric8-tenant-che
objects:
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    labels:
      test: custom-location
      version: ${COMMIT}
      version-quotas: ${COMMIT_QUOTAS}
    name: ${USER_NAME}-che`

var customLocationQuotas = `apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: LimitRange
  metadata:
    labels:
      app: fabric8-tenant-che-quotas
      provider: fabric8
      version: ${COMMIT_QUOTAS}
    name: resource-limits
    namespace: ${USER_NAME}-che`

func TestGetAllTemplatesForAllTypes(t *testing.T) {
	// given
	service := environment.NewServiceForUserData(testdoubles.NewUserDataWithTenantConfig("", "", ""))
	testdoubles.SetTemplateVersions()
	vars := map[string]string{
		"USER_NAME": "dev",
	}

	for _, ctx := range []context.Context{nil, context.Background()} {
		for _, envType := range environment.DefaultEnvTypes {
			// when
			env, err := service.GetEnvData(ctx, envType)
			require.NoError(t, err)
			objects, err := env.Templates[0].Process(vars)

			// then
			require.NoError(t, err)
			assert.Equal(t, env.EnvType, envType)
			switch envType {
			case "che":
				assert.Len(t, env.Templates, 2)
				assert.Contains(t, env.Templates[0].Filename, envType)
				assert.Contains(t, env.Templates[1].Filename, "quotas")
				assert.Equal(t, "234bcd", environment.GetLabelVersion(objects[0]))
				assert.Equal(t, "zyx098", environment.GetLabel(objects[0], environment.FieldVersionQuotas))

			case "user":
				assert.Len(t, env.Templates, 1)
				assert.Contains(t, env.Templates[0].Filename, envType)
				assert.Equal(t, "345cde", environment.GetLabelVersion(objects[0]))
				assert.Empty(t, environment.GetLabel(objects[0], environment.FieldVersionQuotas))
			}

			for _, template := range env.Templates {
				assert.NotEmpty(t, template.Content)
			}
		}
	}
}

func TestAllTemplatesHaveNecessaryData(t *testing.T) {
	// given
	testdoubles.SetTemplateVersions()
	service := environment.NewServiceForUserData(testdoubles.NewUserDataWithTenantConfig("", "", ""))
	vars := map[string]string{
		"USER_NAME": "dev",
	}

	for _, envType := range environment.DefaultEnvTypes {
		nsName := "dev-" + envType
		if envType == environment.TypeUser {
			nsName = "dev"
		}

		// when
		env, err := service.GetEnvData(context.Background(), envType)
		require.NoError(t, err)
		objects, err := env.Templates[0].Process(vars)
		require.NoError(t, err)

		//then
		for _, obj := range objects {
			assert.Regexp(t, regexp.MustCompile(`[1-7]{3}[a-g]{3}`), environment.GetLabelVersion(obj))
			if environment.GetKind(obj) != environment.ValKindProjectRequest {
				assert.Contains(t, environment.GetNamespace(obj), nsName)
			} else {
				assert.Contains(t, environment.GetName(obj), nsName)
			}
		}
	}
}

// Ignored because it downloads files directly from GitHub
func XTestDownloadFromExistingLocation(t *testing.T) {
	// given
	testdoubles.SetTemplateVersions()
	service := environment.NewServiceForUserData(testdoubles.NewUserDataWithTenantConfig("", "29541ca", ""))
	vars := map[string]string{
		"USER_NAME": "dev",
	}

	for _, envType := range environment.DefaultEnvTypes {
		// when
		env, err := service.GetEnvData(context.Background(), envType)
		require.NoError(t, err)

		for _, template := range env.Templates {
			objects, err := template.Process(vars)
			require.NoError(t, err)

			//then
			for _, obj := range objects {
				assert.Equal(t, "29541ca", environment.GetLabelVersion(obj), template.Filename)
			}
		}
	}
}

func TestDownloadFromGivenBlob(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/987654321/environment/templates/fabric8-tenant-user.yml").
		Reply(200).
		BodyString(defaultLocationTempl)
	testdoubles.SetTemplateVersions()
	service := environment.NewServiceForUserData(testdoubles.NewUserDataWithTenantConfig("", "987654321", ""))

	// when
	envData, err := service.GetEnvData(context.Background(), environment.TypeUser)

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
	}
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, environment.GetLabel(objects[0], "test"), "default-location")
	assert.Equal(t, environment.GetLabelVersion(objects[0]), "987654321")
}

func TestDownloadFromGivenBlobLocatedInCustomLocation(t *testing.T) {
	// given
	defer gock.OffAll()
	gock.New("http://raw.githubusercontent.com").
		Get("my-services/my-tenant/987cba/any/path/fabric8-tenant-che-mt.yml").
		Reply(200).
		BodyString(customLocationTempl)
	gock.New("http://raw.githubusercontent.com").
		Get("my-services/my-tenant/987cba/any/path/fabric8-tenant-che-quotas.yml").
		Reply(200).
		BodyString(customLocationQuotas)
	testdoubles.SetTemplateVersions()
	service := environment.NewServiceForUserData(testdoubles.NewUserDataWithTenantConfig("http://github.com/my-services/my-tenant", "987cba", "any/path"))

	// when
	envData, err := service.GetEnvData(context.Background(), environment.TypeChe)

	// then
	require.NoError(t, err)
	vars := map[string]string{
		"USER_NAME": "dev",
	}
	assert.Len(t, envData.Templates, 2)
	objects, err := envData.Templates[0].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Equal(t, environment.GetLabel(objects[0], "test"), "custom-location")
	assert.Equal(t, environment.GetLabelVersion(objects[0]), "987cba")
	assert.Equal(t, environment.GetLabel(objects[0], environment.FieldVersionQuotas), "987cba")

	objects, err = envData.Templates[1].Process(vars)
	require.NoError(t, err)
	assert.Len(t, objects, 1)
	assert.Empty(t, environment.GetLabel(objects[0], environment.FieldVersionQuotas))
	assert.Equal(t, environment.GetLabelVersion(objects[0]), "987cba")
}

var dnsRegExp = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

func TestConstructCompleteVersion(t *testing.T) {
	// given
	testdoubles.SetTemplateVersions()

	// when
	mappedTemplates := environment.RetrieveMappedTemplates()

	t.Run("check che-mt complete version", func(t *testing.T) {
		// and when
		completeVersion := mappedTemplates["che"].ConstructCompleteVersion()

		// then
		assert.Equal(t, "234bcd_zyx098", completeVersion)
	})
	t.Run("check user complete version", func(t *testing.T) {
		// and when
		completeVersion := mappedTemplates["user"].ConstructCompleteVersion()

		// then
		assert.Equal(t, "345cde", completeVersion)
	})
}

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
	assert.Regexp(t, dnsRegExp, environment.RetrieveUserName(username))
	assert.Equal(t, expected, environment.RetrieveUserName(username))
}
