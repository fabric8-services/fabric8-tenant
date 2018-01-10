package token

import (
	"context"

	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type UserProfileService interface {
	GetUserCluster(ctx context.Context, userID string) (string, error)
}

type UserProfileClient struct {
	Config *configuration.Data
}

func (uc *UserProfileClient) GetUserCluster(ctx context.Context, userID string) (string, error) {

	authclient, err := CreateClient(uc.Config)
	if err != nil {
		return "", err
	}

	path := auth.ShowUsersPath(userID)
	res, err := authclient.ShowUsers(ctx, path, nil, nil)

	if err != nil {
		return "", errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	user, err := authclient.DecodeUser(res)
	validationerror := validateError(authclient, res)

	if validationerror != nil {
		return "", errors.Wrapf(validationerror, "error from server %q", uc.Config.GetAuthURL())
	} else if err != nil {
		return "", errors.Wrapf(err, "error from server %q", uc.Config.GetAuthURL())
	}

	return *user.Data.Attributes.Cluster, nil
}
