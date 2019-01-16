package environment

import (
	"context"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

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
	templates := RetrieveMappedTemplates()["che"]
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	err = getCheParams(ctx, templates[0].DefaultParams)

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, templates[0].DefaultParams["JOB_ID"])
	assert.Equal(t, token.Raw, templates[0].DefaultParams["OSIO_TOKEN"])
	assert.Equal(t, sub, templates[0].DefaultParams["IDENTITY_ID"])
	assert.Empty(t, templates[0].DefaultParams["REQUEST_ID"])
}

func TestRetrieveCheMtParamsShouldNotFailIfMissingSub(t *testing.T) {
	// given
	token, err := testsupport.NewToken(
		map[string]interface{}{},
		"../test/private_key.pem",
	)
	require.NoError(t, err)
	templates := RetrieveMappedTemplates()["che"]
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	err = getCheParams(ctx, templates[0].DefaultParams)

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, templates[0].DefaultParams["JOB_ID"])
	assert.Equal(t, token.Raw, templates[0].DefaultParams["OSIO_TOKEN"])
	assert.Empty(t, templates[0].DefaultParams["IDENTITY_ID"])
	assert.Empty(t, templates[0].DefaultParams["REQUEST_ID"])
}

func TestRetrieveCheMtParamsWhenTokenIsMissing(t *testing.T) {
	// given
	templates := RetrieveMappedTemplates()["che"]

	// when
	err := getCheParams(context.Background(), templates[0].DefaultParams)

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, templates[0].DefaultParams["JOB_ID"])
	assert.Empty(t, templates[0].DefaultParams["OSIO_TOKEN"])
	assert.Empty(t, templates[0].DefaultParams["IDENTITY_ID"])
	assert.Empty(t, templates[0].DefaultParams["REQUEST_ID"])
}

var contextInfo = map[string]interface{}{
	"tenantConfig": map[string]interface{}{
		"templatesRepo":     "http://my.own.repo",
		"templatesRepoBlob": "12345",
		"templatesRepoDir":  "my/own/dir",
	},
}

func TestTenantOverride(t *testing.T) {
	internalFeatureLevel := "internal"
	otherFeatureLevel := "production"

	t.Run("override disabled", func(t *testing.T) {

		t.Run("external user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: contextInfo,
				FeatureLevel:       &otherFeatureLevel,
			}

			// when
			service := NewServiceForUserData(user)

			// then
			assertValuesToBeEmpty(t, service)
		})

		t.Run("external user without config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{}

			// when
			service := NewServiceForUserData(user)

			// then
			assertValuesToBeEmpty(t, service)
		})
	})

	t.Run("override enabled", func(t *testing.T) {

		t.Run("internal user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: contextInfo,
				FeatureLevel:       &internalFeatureLevel,
			}

			// when
			service := NewServiceForUserData(user)

			// then
			assert.Equal(t, service.templatesRepo, "http://my.own.repo")
			assert.Equal(t, service.templatesRepoBlob, "12345")
			assert.Equal(t, service.templatesRepoDir, "my/own/dir")
		})

		t.Run("internal user without config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				FeatureLevel: &internalFeatureLevel,
			}

			// when
			service := NewServiceForUserData(user)

			// then
			assertValuesToBeEmpty(t, service)
		})
	})
}

func assertValuesToBeEmpty(t *testing.T, service *Service) {
	assert.Empty(t, service.templatesRepo)
	assert.Empty(t, service.templatesRepoBlob)
	assert.Empty(t, service.templatesRepoDir)
}
