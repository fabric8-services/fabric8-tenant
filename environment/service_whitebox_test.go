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
