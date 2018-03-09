package openshift

import (
	"bytes"
	"net/http"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

// executeRequest executes/submits a request to the given URL using the given HTTP method and authorization token.
// returns the response body or an error if the response status is not "200 OK"
func executeRequest(url, token string, clientOptions ...configuration.HTTPClientOption) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to initialize request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := http.DefaultClient
	for _, applyOption := range clientOptions {
		applyOption(client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to execute request")
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read response body")
	}
	body := buf.Bytes()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("server responded with a non-OK status: %s", string(body))
	}
	return body, nil
}
