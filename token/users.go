package token

import (
	"context"
	"net/http"

	"github.com/fabric8-services/fabric8-tenant/auth"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

type UserService interface {
	Get(ctx context.Context, id uuid.UUID) (*auth.UserDataAttributes, error)
}

func NewAuthUserServiceClient(config AuthClientConfig, serviceToken string) UserService {
	return &userProfileClient{config: config, serviceToken: serviceToken}
}

func (uc *userProfileClient) Get(ctx context.Context, id uuid.UUID) (*auth.UserDataAttributes, error) {

	authclient, err := CreateCustomClient(uc.config, AuthDoer(goaclient.HTTPClientDoer(http.DefaultClient), uc.serviceToken))
	if err != nil {
		return nil, err
	}
	// /api/users not defined as @Secure in design so no invocation of the signer is generated in the client
	authclient.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: uc.serviceToken,
					Type:  "Bearer"}}})

	res, err := authclient.ShowUsers(ctx, auth.ShowUsersPath(id.String()), nil, nil)

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

type userProfileClient struct {
	config       AuthClientConfig
	serviceToken string
}

type authType struct {
	target goaclient.Doer
	token  string
}

func (a *authType) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return a.target.Do(ctx, req)
}

// AuthDoer adds Authorization to all Requests, un related to goa design
func AuthDoer(doer goaclient.Doer, token string) goaclient.Doer {
	return &authType{target: doer, token: token}
}
