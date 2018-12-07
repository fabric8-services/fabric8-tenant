package environment

import (
	"context"
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

func TestRetrieveCheMtParamsShouldFailIfMissingSub(t *testing.T) {
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
	testsupport.AssertError(t, err, testsupport.HasMessage("missing sub in JWT token"))
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
