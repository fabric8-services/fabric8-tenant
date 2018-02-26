package token_test

import (
	"context"
	"net/http"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabric8-services/fabric8-tenant/auth"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/token"
)

func TestResolveUserToken(t *testing.T) {
	// given
	r, err := recorder.New("../test/data/token/auth_resolve_user_token", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	resolveToken := token.NewResolve("http://authservice", auth.WithHTTPClient(&http.Client{Transport: r.Transport}))
	tok, err := createToken("user_foo")
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// when
		username, accessToken, err := resolveToken(context.Background(), "some_valid_openshift_resource", tok.Raw, token.PlainText)
		// then
		require.NoError(t, err)
		assert.Equal(t, "user_foo", username)
		assert.Equal(t, "an_access_token", accessToken)
	})

	t.Run("invalid resource", func(t *testing.T) {
		// when
		_, _, err := resolveToken(context.Background(), "some_invalid_resource", tok.Raw, token.PlainText)
		// then
		require.Error(t, err)
	})

	t.Run("empty access token", func(t *testing.T) {
		// when
		_, _, err := resolveToken(context.Background(), "some_valid_openshift_resource", "", token.PlainText)
		// then
		require.Error(t, err)
	})
}

func createToken(sub string) (*jwt.Token, error) {
	claims := jwt.MapClaims{}
	claims["sub"] = sub
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	// use the test private key to sign the token
	key, err := testsupport.PrivateKey("../test/private_key.pem")
	if err != nil {
		return nil, err
	}
	signed, err := token.SignedString(key)
	if err != nil {
		return nil, err
	}
	token.Raw = signed
	return token, nil
}
