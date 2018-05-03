package user

import (
	"context"

	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// Service the interface for the User service
type Service interface {
	GetUser(ctx context.Context, id uuid.UUID) (*authclient.UserDataAttributes, error)
}

// NewService creates a new User service
func NewService(authURL string, serviceToken string, options ...configuration.HTTPClientOption) Service {
	return &userService{
		authURL:       authURL,
		serviceToken:  serviceToken,
		clientOptions: options,
	}
}

type userService struct {
	authURL       string
	serviceToken  string
	clientOptions []configuration.HTTPClientOption
}

func (s *userService) GetUser(ctx context.Context, id uuid.UUID) (*authclient.UserDataAttributes, error) {
	c, err := auth.NewClient(s.authURL, s.serviceToken, s.clientOptions...)
	if err != nil {
		return nil, err
	}
	// /api/users not defined as @Secure in design so no invocation of the signer is generated in the client
	c.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: s.serviceToken,
					Type:  "Bearer"}}})

	res, err := c.ShowUsers(ctx, authclient.ShowUsersPath(id.String()), nil, nil)

	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	validationerror := auth.ValidateResponse(ctx, c, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.authURL)
	}
	user, err := c.DecodeUser(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.authURL)
	}

	return user.Data.Attributes, nil
}
