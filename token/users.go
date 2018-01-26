package token

import (
	"context"

	"github.com/fabric8-services/fabric8-tenant/auth"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

type UserService interface {
	CurrentUser(ctx context.Context, token string) (*auth.UserDataAttributes, error)
}

func NewAuthUserServiceClient(config AuthClientConfig) UserService {
	return &userProfileClient{config: config}
}

type userProfileClient struct {
	config AuthClientConfig
}

func (uc *userProfileClient) CurrentUser(ctx context.Context, token string) (*auth.UserDataAttributes, error) {

	authclient, err := CreateClient(uc.config)
	if err != nil {
		return nil, err
	}
	authclient.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: token,
					Type:  "Bearer"}}})

	res, err := authclient.ShowUser(ctx, auth.ShowUserPath(), nil, nil)

	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	validationerror := validateError(authclient, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", uc.config.GetAuthURL())
	}
	user, err := authclient.DecodeUser(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", uc.config.GetAuthURL())
	}

	return user.Data.Attributes, nil
}
