package token_test

import (
	"context"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/token"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/stretchr/testify/assert"
)

func TestServiceAccount(t *testing.T) {

	serviceName := "test-service"

	t.Run("Is Service Account", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.True(t, token.IsServiceAccount(ctx))
		})

		t.Run("Missing name", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.False(t, token.IsServiceAccount(ctx))
		})

		t.Run("Missing token", func(t *testing.T) {
			// given
			ctx := context.Background()
			// then
			assert.False(t, token.IsServiceAccount(ctx))
		})

		t.Run("Nil token", func(t *testing.T) {
			// given
			ctx := goajwt.WithJWT(context.Background(), nil)
			// then
			assert.False(t, token.IsServiceAccount(ctx))
		})

		t.Run("Wrong data type", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			claims["service_accountname"] = 100
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.False(t, token.IsServiceAccount(ctx))
		})
	})

	t.Run("Is Specific Service Account", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.True(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})

		t.Run("Missing name", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.False(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Nil token", func(t *testing.T) {
			// given
			ctx := goajwt.WithJWT(context.Background(), nil)
			// then
			assert.False(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Wrong data type", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			claims["service_accountname"] = 100
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.False(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Missing token", func(t *testing.T) {
			// given
			ctx := context.Background()
			// then
			assert.False(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Wrong name", func(t *testing.T) {
			// given
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName + "_asdsa"
			ctx := goajwt.WithJWT(context.Background(), jwt.NewWithClaims(jwt.SigningMethodRS512, claims))
			// then
			assert.False(t, token.IsSpecificServiceAccount(ctx, serviceName))
		})
	})
}
