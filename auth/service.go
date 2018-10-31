package auth

import (
	"context"
	"fmt"
	"io/ioutil"

	"crypto/rsa"
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	commonerrs "github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-wit/rest"
	goaclient "github.com/goadesign/goa/client"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/pkg/errors"
	errs "github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"gopkg.in/square/go-jose.v2"
	"net/http"
)

type Service struct {
	Config        *configuration.Data
	ClientOptions []configuration.HTTPClientOption
	SaToken       string
}

// NewAuthService retrieves SA OAuth token and creates a service instance that is the main point for communication with auth service
func NewAuthService(config *configuration.Data, options ...configuration.HTTPClientOption) (*Service, error) {
	service := &Service{
		Config:        config,
		ClientOptions: options,
	}
	saToken, err := service.getOAuthToken(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch service account token. The cause was: %s", err)
	}
	service.SaToken = *saToken
	return service, nil
}

// User contains user data retrieved from auth service and OS username and user token
type User struct {
	ID                 uuid.UUID
	UserData           *authclient.UserDataAttributes
	OpenShiftUsername  string
	OpenShiftUserToken string
}

// GetUser retrieves user data from auth service related to JWT token stored in the given context.
// It also retrieves OS username and user token for the user's cluster.
func (s *Service) GetUser(ctx context.Context) (*User, error) {
	userToken := goajwt.ContextJWT(ctx)
	if userToken == nil {
		return nil, commonerrs.NewUnauthorizedError("Missing JWT token")
	}
	tenantToken := TenantToken{Token: userToken}

	// fetch the cluster the user belongs to
	userData, err := s.GetAuthUserData(ctx, tenantToken)
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
		return nil, commonerrs.NewUnauthorizedError("could not resolve user token. Caused by: " + err.Error())
	}

	return &User{
		ID:                 tenantToken.Subject(),
		UserData:           userData,
		OpenShiftUsername:  openshiftUsername,
		OpenShiftUserToken: openshiftUserToken,
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
	client, err := s.newClient("") // no need to specify a token in this request
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

	res, err := client.ExchangeToken(ctx, path, payload, contentType)
	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := ValidateResponse(ctx, client, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.Config.GetAuthURL())
	}
	token, err := client.DecodeOauthToken(res)
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

func (s *Service) GetAuthUserData(ctx context.Context, tenantToken TenantToken) (*authclient.UserDataAttributes, error) {
	client, err := s.NewSaClient()
	if err != nil {
		return nil, err
	}

	res, err := client.ShowUsers(ctx, authclient.ShowUsersPath(tenantToken.Subject().String()), nil, nil)

	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := ValidateResponse(ctx, client, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.GetAuthURL())
	}
	user, err := client.DecodeUser(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.GetAuthURL())
	}

	return user.Data.Attributes, nil
}

// GetPublicKeys returns the known public keys used to sign tokens from the auth service
func (s *Service) GetPublicKeys() ([]*rsa.PublicKey, error) {
	client, err := s.newClient("") // no need for a token when calling this endpoint
	if err != nil {
		return nil, errs.Wrapf(err, "unable to retrieve public keys from %s", s.GetAuthURL())
	}
	res, err := client.KeysToken(context.Background(), authclient.KeysTokenPath(), nil)
	if err != nil {
		log.Error(context.Background(), map[string]interface{}{
			"err": err.Error(),
		}, "unable to get public keys from the auth service")
		return nil, errs.Wrap(err, "unable to get public keys from the auth service")
	}
	defer res.Body.Close()
	bodyString := rest.ReadBody(res.Body)
	if res.StatusCode != http.StatusOK {
		log.Error(context.Background(), map[string]interface{}{
			"response_status": res.Status,
		}, "unable to get public keys from the auth service")
		return nil, errors.New("unable to get public keys from the auth service")
	}
	keys, err := unmarshalKeys([]byte(bodyString))
	if err != nil {
		return nil, errs.Wrapf(err, "unable to load keys from auth service")
	}
	log.Info(nil, map[string]interface{}{
		"url":            authclient.KeysTokenPath(),
		"number_of_keys": len(keys),
	}, "Public keys loaded")
	result := make([]*rsa.PublicKey, 0)
	for _, k := range keys {
		result = append(result, k.Key)
	}
	return result, nil
}

// PublicKey a public key loaded from auth service
type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// JSONKeys the JSON structure for unmarshalling the keys
type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

func unmarshalKeys(jsonData []byte) ([]*PublicKey, error) {
	var keys []*PublicKey
	var raw JSONKeys
	err := json.Unmarshal(jsonData, &raw)
	if err != nil {
		return nil, err
	}
	for _, key := range raw.Keys {
		jsonKeyData, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		publicKey, err := unmarshalKey(jsonKeyData)
		if err != nil {
			return nil, err
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

func unmarshalKey(jsonData []byte) (*PublicKey, error) {
	var key *jose.JSONWebKey
	key = &jose.JSONWebKey{}
	err := key.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.Key.(*rsa.PublicKey)
	if !ok {
		return nil, errs.New("Key is not an *rsa.PublicKey")
	}
	log.Info(nil, map[string]interface{}{"key_id": key.KeyID}, "unmarshalled public key")
	return &PublicKey{
			KeyID: key.KeyID,
			Key:   rsaKey},
		nil
}

// TenantToken the token on the tenant
type TenantToken struct {
	Token *jwt.Token
}

// Subject returns the value of the `sub` claim in the token
func (t TenantToken) Subject() uuid.UUID {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		id, err := uuid.FromString(fmt.Sprint(claims["sub"]))
		if err != nil {
			return uuid.UUID{}
		}
		return id
	}
	return uuid.UUID{}
}

// Username returns the value of the `preferred_username` claim in the token
func (t TenantToken) Username() string {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		answer := fmt.Sprint(claims["preferred_username"])
		if len(answer) == 0 {
			answer = fmt.Sprint(claims["username"])
		}
		return answer
	}
	return ""
}

// Email returns the value of the `email` claim in the token
func (t TenantToken) Email() string {
	if claims, ok := t.Token.Claims.(jwt.MapClaims); ok {
		return fmt.Sprint(claims["email"])
	}
	return ""
}
