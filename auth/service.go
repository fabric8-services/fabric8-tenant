package auth

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/dgrijalva/jwt-go"
	commonconf "github.com/fabric8-services/fabric8-common/configuration"
	commonerrs "github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	goaclient "github.com/goadesign/goa/client"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/pkg/errors"
)

type Service struct {
	Config        *configuration.Data
	ClientOptions []commonconf.HTTPClientOption
	SaToken       string
}

// NewAuthService retrieves SA OAuth token and creates a service instance that is the main point for communication with auth service
func NewAuthService(config *configuration.Data, options ...commonconf.HTTPClientOption) (*Service, error) {
	c := &Service{
		Config:        config,
		ClientOptions: options,
	}
	saToken, err := c.getOAuthToken(context.Background())
	if err != nil {
		return nil, err
	}
	c.SaToken = *saToken
	return c, nil
}

// User contains user data retrieved from auth service and OS username and user token
type User struct {
	UserData           *authclient.UserDataAttributes
	OpenshiftUsername  string
	OpenshiftUserToken string
}

// NewUser retrieves user data from auth service related to JWT token stored in the given context.
// It also retrieves OS username and user token for the user's cluster.
func (s *Service) NewUser(ctx context.Context) (*User, error) {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return nil, commonerrs.NewUnauthorizedError("Missing JWT token")
	}

	// fetch the cluster the user belongs to
	userData, err := s.GetAuthUserData(ctx, userToken)
	if err != nil {
		return nil, err
	}

	if userData.Cluster == nil {
		log.Error(ctx, nil, "no cluster defined for tenant")
		return nil, commonerrs.NewInternalError(ctx, fmt.Errorf("unable to provision to undefined cluster"))
	}

	// fetch the users cluster token
	openshiftUsername, openshiftUserToken, err := s.ResolveUserToken(ctx, *userData.Cluster, userToken.Raw)
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err":         err,
			"cluster_url": *userData.Cluster,
		}, "unable to fetch tenant token from auth")
		return nil, commonerrs.NewUnauthorizedError("Could not resolve user token")
	}

	return &User{
		UserData:           userData,
		OpenshiftUsername:  openshiftUsername,
		OpenshiftUserToken: openshiftUserToken,
	}, nil
}

// GetAuthURL returns URL of auth service
func (s *Service) GetAuthURL() string {
	return s.Config.GetAuthURL()
}

// NewSaClient creates an instance of auth client with SA token
func (s *Service) NewSaClient() (*authclient.Client, error) {
	return s.newClient(s.SaToken)
}

func (s *Service) newClient(token string) (*authclient.Client, error) {
	client, err := newClient(s.Config.GetAuthURL(), token, s.ClientOptions...)
	if err != nil {
		return nil, err
	}
	if token != "" {
		client.SetJWTSigner(
			&goaclient.JWTSigner{
				TokenSource: &goaclient.StaticTokenSource{
					StaticToken: &goaclient.StaticToken{
						Value: token,
						Type:  "Bearer"}}})
	}
	return client, nil
}

func (s *Service) getOAuthToken(ctx context.Context) (*string, error) {
	c, err := s.newClient("") // no need to specify a token in this request
	if err != nil {
		return nil, errors.Wrapf(err, "error while initializing the auth client")
	}

	path := authclient.ExchangeTokenPath()
	payload := &authclient.TokenExchange{
		ClientID: s.Config.GetAuthClientID(),
		ClientSecret: func() *string {
			sec := s.Config.GetClientSecret()
			return &sec
		}(),
		GrantType: s.Config.GetAuthGrantType(),
	}
	contentType := "application/x-www-form-urlencoded"

	res, err := c.ExchangeToken(ctx, path, payload, contentType)
	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := ValidateResponse(ctx, c, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.Config.GetAuthURL())
	}
	token, err := c.DecodeOauthToken(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.Config.GetAuthURL())
	}

	if token.AccessToken == nil || *token.AccessToken == "" {
		return nil, fmt.Errorf("received empty token from server %q", s.Config.GetAuthURL())
	}

	return token.AccessToken, nil
}

// ResolveUserToken resolves the token for a human user (can be GitHub, OpenShift Online, etc.)
func (s *Service) ResolveUserToken(ctx context.Context, target, userToken string) (user, accessToken string, err error) {
	return s.ResolveTargetToken(ctx, target, userToken, false, PlainText)
}

// ResolveSaToken resolves the token for a service account user on the given target environment (can be GitHub, OpenShift Online, etc.)
func (s *Service) ResolveSaToken(ctx context.Context, target string) (username, accessToken string, err error) {
	// can't use "forcePull=true" to validate the `tenant service account` token since it's encrypted on auth
	return s.ResolveTargetToken(ctx, target, s.SaToken, false, NewGPGDecypter(s.Config.GetTokenKey()))
}

// ResolveTargetToken resolves the token for a human user or a service account user on the given target environment (can be GitHub, OpenShift Online, etc.)
func (s *Service) ResolveTargetToken(ctx context.Context, target, token string, forcePull bool, decode Decode) (username, accessToken string, err error) {
	// auth can return empty token so validate against that
	if token == "" {
		return "", "", fmt.Errorf("token must not be empty")
	}

	// check if the cluster is empty
	if target == "" {
		return "", "", fmt.Errorf("target must not be empty")
	}

	client, err := s.newClient(token)
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

	err = ValidateResponse(ctx, client, res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while resolving the token for %s", target)
	}

	externalToken, err := client.DecodeExternalToken(res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while decoding the token for %s", target)
	}
	if len(externalToken.Username) == 0 {
		return "", "", errors.Errorf("zero-length username from %s", s.Config.GetAuthURL())
	}

	t, err := decode(externalToken.AccessToken)
	return externalToken.Username, t, err
}

func (s *Service) GetAuthUserData(ctx context.Context, userToken *jwt.Token) (*authclient.UserDataAttributes, error) {
	c, err := s.NewSaClient()
	if err != nil {
		return nil, err
	}

	res, err := c.ShowUsers(ctx, authclient.ShowUsersPath(subject(userToken)), nil, nil)

	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	validationerror := ValidateResponse(ctx, c, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.GetAuthURL())
	}
	user, err := c.DecodeUser(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.GetAuthURL())
	}

	return user.Data.Attributes, nil
}

func subject(token *jwt.Token) string {
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims["sub"].(string)
	}
	return ""
}
