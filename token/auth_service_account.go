package token

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/fabric8-services/fabric8-tenant/auth"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

type ServiceAccountTokenService interface {
	Get(ctx context.Context) (string, error)
}

type AuthClientConfig interface {
	GetAuthURL() string
}

type ServiceAccountTokenServiceConfig interface {
	AuthClientConfig

	GetAuthClientID() string
	GetClientSecret() string
	GetAuthGrantType() string
}

func NewAuthServiceTokenClient(config ServiceAccountTokenServiceConfig) ServiceAccountTokenService {
	return &serviceAccountTokenClient{config: config}
}

type serviceAccountTokenClient struct {
	config ServiceAccountTokenServiceConfig
}

func (c *serviceAccountTokenClient) Get(ctx context.Context) (string, error) {

	authclient, err := CreateClient(c.config)
	if err != nil {
		return "", err
	}

	path := auth.ExchangeTokenPath()
	payload := &auth.TokenExchange{
		ClientID: c.config.GetAuthClientID(),
		ClientSecret: func() *string {
			sec := c.config.GetClientSecret()
			return &sec
		}(),
		GrantType: c.config.GetAuthGrantType(),
	}
	contentType := "application/x-www-form-urlencoded"

	res, err := authclient.ExchangeToken(ctx, path, payload, contentType)
	if err != nil {
		return "", errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := validateError(authclient, res)
	if validationerror != nil {
		return "", errors.Wrapf(validationerror, "error from server %q", c.config.GetAuthURL())
	}
	token, err := authclient.DecodeOauthToken(res)
	if err != nil {
		return "", errors.Wrapf(err, "error from server %q", c.config.GetAuthURL())
	}

	if token.AccessToken == nil || *token.AccessToken == "" {
		return "", fmt.Errorf("received empty token from server %q", c.config.GetAuthURL())
	}

	return *token.AccessToken, nil

}

func CreateClient(config AuthClientConfig) (*auth.Client, error) {
	u, err := url.Parse(config.GetAuthURL())
	if err != nil {
		return nil, err
	}
	c := auth.New(goaclient.HTTPClientDoer(http.DefaultClient))
	c.Host = u.Host
	c.Scheme = u.Scheme
	return c, nil
}

// validateError function when given client and response checks if the
// response has any errors by also looking at the status code
func validateError(c *auth.Client, res *http.Response) error {

	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("404 Not found")
	} else if res.StatusCode != http.StatusOK {

		goaErr, err := c.DecodeJSONAPIErrors(res)
		if err != nil {
			return err
		}
		if len(goaErr.Errors) != 0 {
			var output string
			for _, error := range goaErr.Errors {
				output += fmt.Sprintf("%s: %s %s, %s\n", *error.Title, *error.Status, *error.Code, error.Detail)
			}
			return fmt.Errorf("%s", output)
		}
	}
	return nil
}
