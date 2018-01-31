package auth

import (
	"context"
	"fmt"
	"io/ioutil"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

// TokenResolver resolves a Token for a given user/service
type TokenResolver func(ctx context.Context, target, token *string, decode Decode) (username, accessToken *string, err error)

// TenantResolver resolves tenant tokens based on tenants auth
type TenantResolver func(ctx context.Context, target, token *string) (username, accessToken *string, err error)

// NewTokenResolver creates a Resolver that rely on the Auth service to retrieve tokens
func NewTokenResolver(config ClientConfig) TokenResolver {
	c := tokenService{config: config}
	return c.ResolveUserToken
}

type tokenService struct {
	config ClientConfig
}

func (c *tokenService) ResolveUserToken(ctx context.Context, target, token *string, decode Decode) (username, accessToken *string, err error) {
	// auth can return empty token so validate against that
	if token == nil {
		return nil, nil, fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if target == nil {
		return nil, nil, fmt.Errorf("auth service returned an empty cluster url")
	}

	client, err := NewClient(c.config)
	if err != nil {
		return nil, nil, err
	}
	client.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: *token,
					Type:  "Bearer"}}})

	res, err := client.RetrieveToken(ctx, authclient.RetrieveTokenPath(), *target, nil)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := validateError(client, res)
	if validationerror != nil {
		return nil, nil, errors.Wrapf(validationerror, "error from server %q", c.config.GetAuthURL())
	}

	externalToken, err := client.DecodeExternalToken(res)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "error from server %q", c.config.GetAuthURL())
	}
	if externalToken.Username == nil {
		return nil, nil, errors.Wrapf(err, "missing username", c.config.GetAuthURL())
	}

	t, err := decode(externalToken.AccessToken)
	return externalToken.Username, t, err
}
