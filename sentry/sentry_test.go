package sentry

import (
	"context"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/sentry"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/test"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInitializeSentryLoggerAndSendRecord(t *testing.T) {
	// given
	reset := test.SetEnvironments(test.Env("F8_SENTRY_DSN", "https://abcdef123:abcde123@sentry.instance.server.io/1"))
	defer reset()
	config, err := configuration.NewData()
	require.NoError(t, err)

	// given
	claims := jwt.MapClaims{}
	claims["sub"] = uuid.NewV4().String()
	claims["preferred_username"] = "test-user"
	claims["email"] = "test@acme.com"

	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	ctx := goajwt.WithJWT(context.Background(), token)
	testError := errors.New("test error")

	t.Run("use directly sentry method to send a record", func(t *testing.T) {
		// when
		haltSentry, err := InitializeLogger(config, "123abc")
		defer haltSentry()
		sentry.Sentry().CaptureError(ctx, testError)
		// then
		require.NoError(t, err)
	})

	t.Run("use log error wrapper to send a record", func(t *testing.T) {
		// given
		fields := map[string]interface{}{
			"namespace": "developer-che",
		}

		// when
		haltSentry, err := InitializeLogger(config, "123abc")
		defer haltSentry()
		LogError(ctx, fields, testError, "test message")

		// then
		require.NoError(t, err)
		assert.Len(t, fields, 2)
		assert.Equal(t, fields["namespace"], "developer-che")
		assert.Equal(t, fields["err"], testError)
	})
}

func TestExtractUserInfo(t *testing.T) {

	t.Run("valid token", func(t *testing.T) {
		// given
		id := uuid.NewV4().String()
		claims := jwt.MapClaims{}
		claims["sub"] = id
		claims["preferred_username"] = "test-user"
		claims["email"] = "test@acme.com"
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		ctx := goajwt.WithJWT(context.Background(), token)

		// when
		user, err := extractUserInfo()(ctx)

		// then
		require.NoError(t, err)
		assert.Equal(t, id, user.ID)
		assert.Equal(t, "test-user", user.Username)
		assert.Equal(t, "test@acme.com", user.Email)
	})

	t.Run("token with missing user information", func(t *testing.T) {
		// given
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, jwt.MapClaims{})
		ctx := goajwt.WithJWT(context.Background(), token)

		// when
		user, err := extractUserInfo()(ctx)

		// then
		require.NoError(t, err)
		assert.Equal(t, uuid.UUID{}.String(), user.ID)
		assert.Empty(t, "", user.Username)
		assert.Empty(t, "", user.Email)
	})

	t.Run("context without token", func(t *testing.T) {
		// when
		_, err := extractUserInfo()(context.Background())

		// then
		require.Error(t, err)
		assert.Equal(t, err.Error(), "no token found in context")
	})
}
