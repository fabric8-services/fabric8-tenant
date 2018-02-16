package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	goaclient "github.com/goadesign/goa/client"
)

type clientImpl struct {
	client authclient.Client
}

// NewClient returns a new auth client
func NewClient(config ClientConfig, options ...ClientOption) (*authclient.Client, error) {
	u, err := url.Parse(config.GetAuthURL())
	if err != nil {
		return nil, err
	}
	c := doerConfig{
		httpClient: http.DefaultClient,
	}
	// apply options
	for _, opt := range options {
		opt(&c)
	}
	client := authclient.New(newDoer(c))
	client.Host = u.Host
	client.Scheme = u.Scheme
	return client, nil
}

// ClientConfig the client config
type ClientConfig interface {
	GetAuthURL() string
}

// ClientOption a function to customize the auth client
type ClientOption func(*doerConfig)

type doerConfig struct {
	httpClient *http.Client
	token      *string
}

// WithHTTPClient an option to specify the http client to use
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *doerConfig) {
		c.httpClient = httpClient
	}
}

// WithToken an option to specify the token to use
func WithToken(token string) ClientOption {
	return func(c *doerConfig) {
		c.token = &token
	}
}

type doer struct {
	target goaclient.Doer
	token  *string
}

// Doer adds Authorization to all Requests, un related to goa design
func newDoer(config doerConfig) goaclient.Doer {
	return &doer{
		target: goaclient.HTTPClientDoer(config.httpClient),
		token:  config.token,
	}
}

func (d *doer) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if d.token != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *d.token))
	}
	return d.target.Do(ctx, req)
}

// ValidateError function when given client and response checks if the
// response has any errors by also looking at the status code
func ValidateError(c *authclient.Client, res *http.Response) error {
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
