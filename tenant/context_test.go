package tenant_test

import (
	"context"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/stretchr/testify/assert"
)

func TestServiceAccount(t *testing.T) {

	serviceName := "test-service"

	t.Run("Is Service Account", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.True(t, tenant.IsServiceAccount(ctx))
		})
		t.Run("Missing name", func(t *testing.T) {
			claims := jwt.MapClaims{}
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.False(t, tenant.IsServiceAccount(ctx))
		})
		t.Run("Missing token", func(t *testing.T) {
			ctx := context.Background()

			assert.False(t, tenant.IsServiceAccount(ctx))
		})
		t.Run("Nil token", func(t *testing.T) {
			ctx := goajwt.WithJWT(context.Background(), nil)

			assert.False(t, tenant.IsServiceAccount(ctx))
		})
		t.Run("Wrong data type", func(t *testing.T) {
			claims := jwt.MapClaims{}
			claims["service_accountname"] = 100
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.False(t, tenant.IsServiceAccount(ctx))
		})
	})
	t.Run("Is Specific Service Account", func(t *testing.T) {

		t.Run("Valid", func(t *testing.T) {
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.True(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Missing name", func(t *testing.T) {
			claims := jwt.MapClaims{}
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.False(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Nil token", func(t *testing.T) {
			ctx := goajwt.WithJWT(context.Background(), nil)

			assert.False(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Wrong data type", func(t *testing.T) {
			claims := jwt.MapClaims{}
			claims["service_accountname"] = 100
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.False(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Missing token", func(t *testing.T) {
			ctx := context.Background()

			assert.False(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
		t.Run("Wrong name", func(t *testing.T) {
			claims := jwt.MapClaims{}
			claims["service_accountname"] = serviceName + "_asdsa"
			token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
			ctx := goajwt.WithJWT(context.Background(), token)

			assert.False(t, tenant.IsSpecificServiceAccount(ctx, serviceName))
		})
	})
}
