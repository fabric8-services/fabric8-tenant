package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// WhoAmI checks with OSO who owns the current token.
// returns the username
func WhoAmI(ctx context.Context, clusterURL string, token string, clientOptions ...configuration.HTTPClientOption) (string, error) {
	body, err := executeRequest(ctx,
		request{
			method:      "GET",
			url:         fmt.Sprintf("%s/apis/user.openshift.io/v1/users/~", strings.TrimSuffix(clusterURL, "/")),
			bearerToken: token},
		clientOptions...)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	var u user
	err = yaml.Unmarshal(body, &u)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	return u.Metadata.Name, nil
}

type user struct {
	Metadata struct {
		Name string
	}
}
