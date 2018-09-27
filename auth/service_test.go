package auth_test

import (
	"context"
	"testing"

	"github.com/fabric8-services/fabric8-auth/errors"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveUserToken(t *testing.T) {
	// given
	authService, cleanup := testdoubles.NewAuthService(t, "../test/data/token/auth_resolve_target_token", "http://authservice", recorder.WithJWTMatcher)
	defer cleanup()
	tok, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "user_foo",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// when
		username, accessToken, err := authService.ResolveUserToken(context.Background(), "some_valid_openshift_resource", tok.Raw)
		// then
		require.NoError(t, err)
		assert.Equal(t, "user_foo", username)
		assert.Equal(t, "an_openshift_token", accessToken)
	})

	t.Run("invalid resource", func(t *testing.T) {
		// when
		_, _, err := authService.ResolveUserToken(context.Background(), "some_invalid_resource", tok.Raw)
		// then
		testsupport.AssertError(t, err, testsupport.IsOfType(errors.InternalError{}),
			testsupport.HasMessageContaining("error while resolving the token for some_invalid_resource"))
	})

	t.Run("empty access token", func(t *testing.T) {
		// when
		_, _, err := authService.ResolveUserToken(context.Background(), "some_valid_openshift_resource", "")
		// then
		testsupport.AssertError(t, err, testsupport.HasMessage("token must not be empty"))
	})
}

func TestResolveServiceAccountToken(t *testing.T) {
	// given
	reset := testdoubles.SetEnvironments(testdoubles.Env("F8_AUTH_TOKEN_KEY", "foo"))
	defer reset()
	authService, cleanup := testdoubles.NewAuthService(t, "../test/data/token/auth_resolve_target_token", "http://authservice", recorder.WithJWTMatcher)
	defer cleanup()

	tok, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// given
		authService.SaToken = tok.Raw
		// when
		username, accessToken, err := authService.ResolveSaToken(context.Background(), "some_valid_openshift_resource")
		// then
		require.NoError(t, err)
		assert.Equal(t, "tenant_service", username)
		assert.Equal(t, "an_openshift_token", accessToken)
	})

	t.Run("expired token", func(t *testing.T) {
		// given
		expTok, err := testsupport.NewToken(map[string]interface{}{
			"sub": "expired_tenant_service",
		}, "../test/private_key.pem")
		require.NoError(t, err)
		authService.SaToken = expTok.Raw
		// when
		_, _, err = authService.ResolveSaToken(context.Background(), "some_valid_openshift_resource")
		// then
		testsupport.AssertError(t, err,
			testsupport.HasMessageContaining("error while resolving the token for some_valid_openshift_resource"))
	})

	t.Run("invalid resource", func(t *testing.T) {
		// given
		authService.SaToken = tok.Raw
		// when
		_, _, err := authService.ResolveSaToken(context.Background(), "some_invalid_resource")
		// then
		testsupport.AssertError(t, err,
			testsupport.HasMessageContaining("error while resolving the token for some_invalid_resource"))
	})

	t.Run("empty access token", func(t *testing.T) {
		// given
		authService.SaToken = ""
		// when
		_, _, err := authService.ResolveSaToken(context.Background(), "some_valid_openshift_resource")
		// then
		testsupport.AssertError(t, err, testsupport.HasMessage("token must not be empty"))
	})
}

func TestUserProfileClient_GetUserCluster(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		wantErr string
	}{
		{
			name:    "normal input to see if cluster is parsed",
			wantErr: "",
			user:    "normal_user",
		},
		{
			name:    "bad status code",
			wantErr: "Not Found error: 404 not_found",
			user:    "bad_status_code_user",
		},
		{
			name:    "make code fail on parsing output",
			wantErr: "invalid character",
			user:    "wrong_output_user",
		},
	}

	authClientService, cleanup := testdoubles.NewAuthService(t, "../test/data/token/auth_resolve_user", "http://authservice", recorder.WithJWTMatcher)
	defer cleanup()
	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)
	authClientService.SaToken = saToken.Raw

	for _, testData := range tests {
		t.Run(testData.name, func(t *testing.T) {
			// given
			userToken, err := testsupport.NewToken(
				map[string]interface{}{
					"sub": testData.user,
				},
				"../test/private_key.pem",
			)
			require.NoError(t, err)

			// when
			user, err := authClientService.GetUser(goajwt.WithJWT(context.Background(), userToken))

			// then
			if testData.wantErr != "" {
				testsupport.AssertError(t, err,
					testsupport.HasMessageContaining("error from server \"http://authservice\""),
					testsupport.HasMessageContaining(testData.wantErr))
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			assert.Equal(t, "fake-cluster.com", *user.UserData.Cluster)
		})
	}
}

func TestPublicKeys(t *testing.T) {

	t.Run("valid keys", func(t *testing.T) {
		//given
		authService, cleanup := testdoubles.NewAuthService(t, "../test/data/token/auth_get_keys", "http://authservice")
		defer cleanup()
		// when
		result, err := authService.GetPublicKeys()
		// then
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("invalid url", func(t *testing.T) {
		//given
		authService, cleanup := testdoubles.NewAuthService(t, "", "http://google.com")
		defer cleanup()
		// when
		_, err := authService.GetPublicKeys()
		// then
		testsupport.AssertError(t, err,
			testsupport.HasMessageContaining("unable to get public keys from the auth service"))
	})
}

func TestInitializeAuthServiceAndGetSaToken(t *testing.T) {
	// given
	reset := testdoubles.SetEnvironments(testdoubles.Env("F8_AUTH_URL", "http://authservice"))
	defer reset()
	record, err := recorder.New("../test/data/token/auth_resolve_target_token")
	defer func() {
		err := record.Stop()
		require.NoError(t, err)
	}()
	require.NoError(t, err)
	config, err := configuration.GetData()
	require.NoError(t, err)

	// when
	authService, err := auth.NewAuthService(config, configuration.WithRoundTripper(record))

	// then
	require.NoError(t, err)
	expSaToken := "jA0ECQMC5AvXo6Jyrj5g0kcBv6Qp8ZTWCgYD6TESuc2OxSDZ1lic1tmV6g4IcQUBlohjT3gyQX2oTa1bWfNkk8xY6wyPq8CUK3ReOnnDK/yo661f6LXgvA=="
	assert.Equal(t, authService.SaToken, expSaToken)
}
