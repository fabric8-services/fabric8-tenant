package token

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

type userData struct {
	Data struct {
		Attributes struct {
			Cluster string `json:"cluster,omitempty"`
		}
	}
}

type UserProfileService interface {
	GetUserCluster(userID string) (string, error)
}

type UserController struct {
	Config       *configuration.Data
	ClusterToken ClusterTokenService
}

func (uc *UserController) GetUserCluster(userID string) (string, error) {
	u, err := url.Parse(uc.Config.GetAuthURL())
	if err != nil {
		return "", errors.Wrapf(err, "error parsing auth url")
	}
	u.Path = filepath.Join("/api/users", userID)

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", errors.Wrapf(err, "error creating request object")
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

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
		return "", errors.Wrapf(err, "error from server %q", uc.Config.GetAuthURL())
	}

	var response userData
	err = json.Unmarshal(body, &response)
	if err != nil {
		return "", errors.Wrapf(err, "error unmarshalling the response")
	}

	return response.Data.Attributes.Cluster, nil
}