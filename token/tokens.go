package token

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type ClusterTokenService interface {
	Get(config *configuration.Data) error
}

type ClusterTokenClient struct {
	AuthServiceAccountToken string
}

func (c *ClusterTokenClient) Get(config *configuration.Data) error {
	payload := strings.NewReader("grant_type=" + config.GetAuthGrantType() + "&client_id=" +
		config.GetAuthClientID() + "&client_secret=" + config.GetClientSecret())

	req, err := http.NewRequest("POST", config.GetAuthURL()+"/api/token", payload)
	if err != nil {
		return errors.Wrapf(err, "error creating request object")
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "error reading response")
	}

	if err := validateError(res.StatusCode, body); err != nil {
		return errors.Wrapf(err, "error from server %q", config.GetAuthURL())
	}

	// parse the token from the output
	if c.AuthServiceAccountToken, err = parseToken(body); err != nil {
		return err
	}

	return nil
}

func validateError(status int, body []byte) error {
	type authEerror struct {
		Code   string `json:"code,omitempty"`
		Detail string `json:"detail,omitempty"`
		Status string `json:"status,omitempty"`
		Title  string `json:"title,omitempty"`
	}

	type errorResponse struct {
		Errors []authEerror `json:"errors,omitempty"`
	}

	if status != http.StatusOK {
		var e errorResponse
		err := json.Unmarshal(body, &e)
		if err != nil {
			return errors.Wrapf(err, "could not unmarshal the response")
		}

		var output string
		for _, error := range e.Errors {
			output += fmt.Sprintf("%s: %s %s, %s\n", error.Title, error.Status, error.Code, error.Detail)
		}
		return fmt.Errorf("%s", output)
	}
	return nil
}

type OpenShiftTokenService interface {
	Get(config *configuration.Data, accessToken string, cluster string) error
}

type OpenShiftTokenClient struct {
	OpenShiftToken string
}

func (z *OpenShiftTokenClient) Get(config *configuration.Data, accessToken string, cluster string) error {
	// auth can return empty token so validate against that
	if accessToken == "" {
		return fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if cluster == "" {
		return fmt.Errorf("cluster URL can't be empty")
	}

	// a normal query will look like following
	// http://auth-fabric8.192.168.42.181.nip.io/api/token?for=https://api.starter-us-east-2a.openshift.com
	u, err := url.Parse(config.GetAuthURL())
	if err != nil {
		return errors.Wrapf(err, "error parsing auth url")
	}
	u.Path = "/api/token"
	q := u.Query()
	q.Set("for", cluster)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "error creating request object")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrapf(err, "error reading response")
	}

	if err := validateError(res.StatusCode, body); err != nil {
		return errors.Wrapf(err, "error from server %q", config.GetAuthURL())
	}

	// parse the token from the output
	if z.OpenShiftToken, err = parseToken(body); err != nil {
		return err
	}

	return nil
}

func parseToken(data []byte) (string, error) {
	// this struct is defined to obtain the accesstoken from the output
	type authAccessToken struct {
		AccessToken string `json:"access_token,omitempty"`
	}

	var r authAccessToken
	err := json.Unmarshal(data, &r)
	if err != nil {
		return "", errors.Wrapf(err, "error unmarshalling the response")
	}
	return strings.TrimSpace(r.AccessToken), nil
}
