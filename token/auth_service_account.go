package token

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type ServiceAccountTokenService interface {
	Get() error
}

type ServiceAccountTokenClient struct {
	Config                  *configuration.Data
	AuthServiceAccountToken string
}

func (c *ServiceAccountTokenClient) Get() error {
	payload := strings.NewReader("grant_type=" + c.Config.GetAuthGrantType() + "&client_id=" +
		c.Config.GetAuthClientID() + "&client_secret=" + c.Config.GetClientSecret())

	req, err := http.NewRequest("POST", c.Config.GetAuthURL()+"/api/token", payload)
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
		return errors.Wrapf(err, "error from server %q", c.Config.GetAuthURL())
	}

	// parse the token from the output
	if c.AuthServiceAccountToken, err = parseToken(body); err != nil {
		return err
	}

	return nil
}
