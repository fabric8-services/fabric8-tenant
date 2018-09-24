package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/fabric8-services/fabric8-auth/errors"

	"github.com/fabric8-services/fabric8-common/log"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	goaclient "github.com/goadesign/goa/client"
)

// newClient returns a new auth client
func newClient(authURL, token string, options ...configuration.HTTPClientOption) (*authclient.Client, error) {
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
	log.Debug(nil, map[string]interface{}{"host": client.Host, "scheme": client.Scheme}, "initializing auth client")
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
func ValidateResponse(ctx context.Context, c *authclient.Client, res *http.Response) error {
	// 2xx and 3xx response are not considered as errors
	if res.StatusCode < 400 {
		return nil
	}
	errs, err := c.DecodeJSONAPIErrors(res)
	if err != nil {
		return err
	}
	if len(errs.Errors) > 0 {
		if res.StatusCode == http.StatusNotFound {
			// take the first JSON-API error and convert it into a NotFoundError
			if errs.Errors[0].ID != nil {
				return errors.NewNotFoundError("users", *errs.Errors[0].ID)
			}
		}
		var output string
		for _, error := range errs.Errors {
			output += fmt.Sprintf("%s: %s %s, %s\n", *error.Title, *error.Status, *error.Code, error.Detail)
		}
		return errors.NewInternalError(ctx, fmt.Errorf("%s", output))
	}
	return errors.NewInternalError(ctx, fmt.Errorf("unknown error: %d", res.StatusCode))
}
