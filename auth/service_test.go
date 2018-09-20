package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
)

func TestResolveUserToken(t *testing.T) {
	// given
	authService, r, err := testdoubles.NewAuthClientService("../test/data/token/auth_resolve_target_token", "http://authservice", recorder.WithJWTMatcher)
	require.NoError(t, err)
	defer r.Stop()
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
		require.Error(t, err)
	})

	t.Run("empty access token", func(t *testing.T) {
		// when
		_, _, err := authService.ResolveUserToken(context.Background(), "some_valid_openshift_resource", "")
		// then
		require.Error(t, err)
	})
}

func TestResolveServiceAccountToken(t *testing.T) {
	// given
	authService, r, err := testdoubles.NewAuthClientService("../test/data/token/auth_resolve_target_token", "http://authservice", recorder.WithJWTMatcher)
	require.NoError(t, err)
	defer r.Stop()
	authService.Config.Set(configuration.VarAuthTokenKey, "foo")
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
		require.Error(t, err)
	})

	t.Run("invalid resource", func(t *testing.T) {
		// given
		authService.SaToken = tok.Raw
		// when
		_, _, err := authService.ResolveSaToken(context.Background(), "some_invalid_resource")
		// then
		require.Error(t, err)
	})

	t.Run("empty access token", func(t *testing.T) {
		// given
		authService.SaToken = ""
		// when
		_, _, err := authService.ResolveSaToken(context.Background(), "some_valid_openshift_resource")
		// then
		require.Error(t, err)
	})
}

func TestUserProfileClient_GetUserCluster(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		wantErr bool
	}{
		{
			name:    "normal input to see if cluster is parsed",
			wantErr: false,
			user:    "normal_user",
		},
		{
			name:    "bad status code",
			wantErr: true,
			user:    "bad_status_code_user",
		},
		{
			name:    "make code fail on parsing output",
			wantErr: true,
			user:    "wrong_output_user",
		},
	}

	authClientService, _, err := testdoubles.NewAuthClientService("../test/data/token/auth_resolve_user", "http://authservice", recorder.WithJWTMatcher)
	require.NoError(t, err)
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
			user, err := authClientService.NewUser(goajwt.WithJWT(context.Background(), userToken))

			// then
			if testData.wantErr {
				require.Error(t, err)
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
		authService, r, err := testdoubles.NewAuthClientService("../test/data/token/auth_get_keys", "http://authservice")
		require.NoError(t, err)
		defer r.Stop()
		// when
		result, err := authService.GetPublicKeys()
		// then
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("invalid url", func(t *testing.T) {
		//given
		authService, _, err := testdoubles.NewAuthClientService("", "http://google.com")
		assert.NoError(t, err)
		// when
		_, err = authService.GetPublicKeys()
		// then
		assert.Error(t, err)
	})
}
