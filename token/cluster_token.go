package token

import (
	"context"
	"fmt"

	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type ClusterTokenService interface {
	Get(ctx context.Context, cluster string) (string, error)
}

type ClusterTokenClient struct {
	Config      *configuration.Data
	AccessToken string
}

func (c *ClusterTokenClient) Get(ctx context.Context, cluster string) (string, error) {
	// auth can return empty token so validate against that
	if c.AccessToken == "" {
		return "", fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if cluster == "" {
		return "", fmt.Errorf("auth service returned an empty cluster url")
	}

	authclient, err := CreateClient(c.Config)
	if err != nil {
		return "", err
	}
	path := auth.RetrieveTokenPath()
	res, err := authclient.RetrieveToken(ctx, path, cluster, nil)
	if err != nil {
		return "", errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	token, err := authclient.DecodeOauthToken(res)
	validationerror := validateError(authclient, res)

	if validationerror != nil {
		return "", errors.Wrapf(validationerror, "error from server %q", c.Config.GetAuthURL())
	} else if err != nil {
		return "", errors.Wrapf(err, "error from server %q", c.Config.GetAuthURL())
	}

	var openShiftToken string
	if *token.AccessToken != "" {
		openShiftToken = *token.AccessToken
	}

	return openShiftToken, nil
}
