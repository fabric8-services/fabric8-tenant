package token

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/pkg/errors"
)

// ServiceAccountTokenService the interface for the Service Account service
type ServiceAccountTokenService interface {
	GetOAuthToken(ctx context.Context) (*string, error)
}

// ServiceAccountTokenServiceConfig the config for the Service Account service
type ServiceAccountTokenServiceConfig interface {
	GetAuthURL() string
	GetAuthClientID() string
	GetClientSecret() string
	GetAuthGrantType() string
}

// NewServiceAccountTokenService initializes a new ServiceAccountTokenService
func NewServiceAccountTokenService(config ServiceAccountTokenServiceConfig, options ...auth.ClientOption) ServiceAccountTokenService {
	return &serviceAccountTokenService{config: config}
}

type serviceAccountTokenService struct {
	config        ServiceAccountTokenServiceConfig
	clientOptions []auth.ClientOption
}

func (s *serviceAccountTokenService) GetOAuthToken(ctx context.Context) (*string, error) {
	c, err := auth.NewClient(s.config.GetAuthURL(), s.clientOptions...)
	if err != nil {
		return nil, errors.Wrapf(err, "error while initializing the auth client")
	}

	path := authclient.ExchangeTokenPath()
	payload := &authclient.TokenExchange{
		ClientID: s.config.GetAuthClientID(),
		ClientSecret: func() *string {
			sec := s.config.GetClientSecret()
			return &sec
		}(),
		GrantType: s.config.GetAuthGrantType(),
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

	validationerror := auth.ValidateResponse(c, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.config.GetAuthURL())
	}
	token, err := c.DecodeOauthToken(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.config.GetAuthURL())
	}

	if token.AccessToken == nil || *token.AccessToken == "" {
		return nil, fmt.Errorf("received empty token from server %q", s.config.GetAuthURL())
	}

	return token.AccessToken, nil
}
