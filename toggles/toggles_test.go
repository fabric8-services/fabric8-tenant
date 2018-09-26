package toggles_test

import (
	"context"
	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"testing"
)

func TestWithContextWhenClaimsAreMissing(t *testing.T) {
	// given
	claims := jwt.MapClaims{}
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	toggles.WithContext(ctx)

	// then should not panic
}

func TestWithContextWithOnlySubClaimPresent(t *testing.T) {
	// given
	claims := jwt.MapClaims{}
	claims["sub"] = uuid.NewV4().String()
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	toggles.WithContext(ctx)

	// then should not panic
}

func TestWithContextWithOnlySessionStateClaimPresent(t *testing.T) {
	// given
	claims := jwt.MapClaims{}
	claims["session_state"] = uuid.NewV4().String()
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	ctx := goajwt.WithJWT(context.Background(), token)

	// when
	toggles.WithContext(ctx)

	// then should not panic
}
