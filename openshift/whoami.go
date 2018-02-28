package openshift

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// WhoAmI checks with OSO who owns the current token.
// returns the username
func WhoAmI(ctx context.Context, config Config) (string, error) {
	whoamiURL := config.MasterURL + "/apis/user.openshift.io/v1/users/~"
	statusCode, body, err := execute(ctx, config.CreateHTTPClient(), "GET", whoamiURL, config.Token)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	if statusCode != http.StatusOK {
		return "", errors.Errorf("unexpected response code: \n%v\n%v", statusCode, string(body))
	}

	var u user
	err = yaml.Unmarshal(body, &u)
	if err != nil {
		return "", err
	}
	return u.Metadata.Name, nil
}

type user struct {
	Metadata struct {
		Name string
	}
}
