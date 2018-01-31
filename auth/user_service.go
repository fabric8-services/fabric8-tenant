package auth

import (
	"context"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// UserService the interface for the User service
type UserService interface {
	GetUser(ctx context.Context, id uuid.UUID) (*authclient.UserDataAttributes, error)
}

// UserServiceConfig the User service config
type UserServiceConfig interface {
	GetAuthURL() string
}

// NewUserService creates a new User service
func NewUserService(config UserServiceConfig, serviceToken string) UserService {
	return &userService{config: config, serviceToken: serviceToken}
}

type userService struct {
	config       UserServiceConfig
	serviceToken string
}

func (s *userService) GetUser(ctx context.Context, id uuid.UUID) (*authclient.UserDataAttributes, error) {
	c, err := NewClient(s.config, WithToken(s.serviceToken))
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

	validationerror := validateError(c, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.config.GetAuthURL())
	}
	user, err := c.DecodeUser(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.config.GetAuthURL())
	}

	return user.Data.Attributes, nil
}
