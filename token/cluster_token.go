package token

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type OpenShiftTokenService interface {
	Get(cluster string) (string, error)
}

type OpenShiftTokenClient struct {
	Config      *configuration.Data
	AccessToken string
}

func (c *OpenShiftTokenClient) Get(cluster string) (string, error) {
	// auth can return empty token so validate against that
	if c.AccessToken == "" {
		return "", fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if cluster == "" {
		return "", fmt.Errorf("auth service returned an empty cluster url")
	}

	// a normal query will look like following
	// http://auth-fabric8.192.168.42.181.nip.io/api/token?for=https://api.starter-us-east-2a.openshift.com
	u, err := url.Parse(c.Config.GetAuthURL())
	if err != nil {
		return "", errors.Wrapf(err, "error parsing auth url")
	}
	u.Path = "/api/token"
	q := u.Query()
	q.Set("for", cluster)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", errors.Wrapf(err, "error creating request object")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", "Bearer "+c.AccessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "error while doing the request")
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", errors.Wrapf(err, "error reading response")
	}

	if err := validateError(res.StatusCode, body); err != nil {
		return "", errors.Wrapf(err, "error from server %q", c.Config.GetAuthURL())
	}

	openShiftToken, err := parseToken(body)
	// parse the token from the output
	if err != nil {
		return "", err
	}

	return openShiftToken, nil
}
