package token

import (
	"context"
	"fmt"

	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/pkg/errors"
)

// Resolver resolves a Token for a given user/service
type Resolver func(ctx context.Context, target, token string, decode Decode) (accessToken string, err error)

// Manager is an interface to split Cluster token lookup vs User token lookup.
// Primarly to 'hide' Service token from normal Controller usage
type Manager interface {
	Cluster(ctx context.Context, target string) (string, error)
	Tenant(ctx context.Context, target, token string) (string, error)
}

// NewAuthServiceResolver creates a Resolver that rely on the Auth service to retrieve tokens
func NewAuthServiceResolver(config AuthClientConfig) Resolver {
	c := tokenClient{Config: config}
	return c.Get
}

func NewAuthServiceManager(resolver Resolver, serviceToken, passphrase string) Manager {
	return &tokenManager{
		resolver:     resolver,
		serviceToken: serviceToken,
		passphrase:   passphrase,
	}
}

type tokenManager struct {
	resolver     Resolver
	serviceToken string
	passphrase   string
}

func (t *tokenManager) Cluster(ctx context.Context, target string) (string, error) {
	return t.resolver(ctx, target, t.serviceToken, NewGPGDecypter(t.passphrase))
}

func (t *tokenManager) Tenant(ctx context.Context, target, token string) (string, error) {
	return t.resolver(ctx, target, token, PlainTextToken)
}

type tokenClient struct {
	Config AuthClientConfig
}

func (c *tokenClient) Get(ctx context.Context, target, token string, decode Decode) (string, error) {
	// auth can return empty token so validate against that
	if token == "" {
		return "", fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if target == "" {
		return "", fmt.Errorf("auth service returned an empty cluster url")
	}

	authclient, err := CreateClient(c.Config)
	if err != nil {
		return "", err
	}
	res, err := authclient.RetrieveToken(ctx, auth.RetrieveTokenPath(), target, nil)
	if err != nil {
		return "", errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	externalToken, err := authclient.DecodeExternalToken(res)
	validationerror := validateError(authclient, res)

	if validationerror != nil {
		return "", errors.Wrapf(validationerror, "error from server %q", c.Config.GetAuthURL())
	} else if err != nil {
		return "", errors.Wrapf(err, "error from server %q", c.Config.GetAuthURL())
	}

	return decode(externalToken.AccessToken)
}
