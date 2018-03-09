package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	goaclient "github.com/goadesign/goa/client"
)

type clientImpl struct {
	client authclient.Client
}

// NewClient returns a new auth client
func NewClient(authURL, token string, options ...configuration.HTTPClientOption) (*authclient.Client, error) {
	u, err := url.Parse(authURL)
	if err != nil {
		return nil, err
	}
	httpClient := http.DefaultClient
	// apply options
	for _, opt := range options {
		opt(httpClient)
	}
	client := authclient.New(&doer{
		target: goaclient.HTTPClientDoer(httpClient),
		token:  token,
	})
	client.Host = u.Host
	client.Scheme = u.Scheme
	// log.Debug(nil, map[string]interface{}{"host": client.Host, "scheme": client.Scheme}, "initializing auth client")
	return client, nil
}

type doer struct {
	target goaclient.Doer
	token  string
}

func (d *doer) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if d.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.token))
	}
	return d.target.Do(ctx, req)
}

// ValidateResponse function when given client and response checks if the
// response has any errors by also looking at the status code
func ValidateResponse(c *authclient.Client, res *http.Response) error {
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found")
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
