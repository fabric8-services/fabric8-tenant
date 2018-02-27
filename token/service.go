package token

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

type tokenService struct {
	authURL       string
	clientOptions []auth.ClientOption
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

	client, err := auth.NewClient(s.authURL, s.clientOptions...)
	if err != nil {
		return "", "", err
	}
	client.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: token,
					Type:  "Bearer"}}})

	res, err := client.RetrieveToken(ctx, authclient.RetrieveTokenPath(), target, &forcePull)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := auth.ValidateError(client, res)
	if validationerror != nil {
		return "", "", errors.Wrapf(validationerror, "error from server %q", s.authURL)
	}

	externalToken, err := client.DecodeExternalToken(res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error from server %q", s.authURL)
	}
	if externalToken.Username == nil {
		return "", "", errors.Wrapf(err, "missing username", s.authURL)
	}

	t, err := decode(externalToken.AccessToken)
	return *externalToken.Username, t, err
}
