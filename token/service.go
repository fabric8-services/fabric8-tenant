package token

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

type tokenService struct {
	authURL       string
	clientOptions []configuration.HTTPClientOption
}

// ResolveTargetToken resolves the token for a human user or a service account user on the given target environment (can be GitHub, OpenShift Online, etc.)
func (s *tokenService) ResolveTargetToken(ctx context.Context, target, token string, forcePull bool, decode Decode) (username, accessToken string, err error) {
	// auth can return empty token so validate against that
	if token == "" {
		return "", "", fmt.Errorf("token must not be empty")
	}

	// check if the cluster is empty
	if target == "" {
		return "", "", fmt.Errorf("target must not be empty")
	}

	client, err := auth.NewClient(s.authURL, token, s.clientOptions...)
	if err != nil {
		return "", "", err
	}
	res, err := client.RetrieveToken(ctx, authclient.RetrieveTokenPath(), target, &forcePull)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while resolving the token for %s", target)
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	err = auth.ValidateResponse(ctx, client, res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while resolving the token for %s", target)
	}

	externalToken, err := client.DecodeExternalToken(res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while decoding the token for %s", target)
	}
	if len(externalToken.Username) == 0 {
		return "", "", errors.Errorf("zero-length username from %s", s.authURL)
	}

	t, err := decode(externalToken.AccessToken)
	return externalToken.Username, t, err
}

// TenantToken the token on the tenant
type TenantToken struct {
	Token *jwt.Token
}

// Subject returns the value of the `sub` claim in the token
func (t TenantToken) Subject() uuid.UUID {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		id, err := uuid.FromString(fmt.Sprint(claims["sub"]))
		if err != nil {
			return uuid.UUID{}
		}
		return id
	}
	return uuid.UUID{}
}

// Username returns the value of the `preferred_username` claim in the token
func (t TenantToken) Username() string {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		answer := fmt.Sprint(claims["preferred_username"])
		if len(answer) == 0 {
			answer = fmt.Sprint(claims["username"])
		}
		return answer
	}
	return ""
}

// Email returns the value of the `email` claim in the token
func (t TenantToken) Email() string {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		return fmt.Sprint(claims["email"])
	}
	return ""
}
