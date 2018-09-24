package openshift

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// WhoAmI checks with OSO who owns the current token.
// returns the username
func WhoAmI(ctx context.Context, clusterURL string, token string, clientOptions ...configuration.HTTPClientOption) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/apis/user.openshift.io/v1/users/~", strings.TrimSuffix(clusterURL, "/")), nil)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := http.DefaultClient
	for _, applyOption := range clientOptions {
		applyOption(client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return "", errors.Wrapf(err, "unable to retrieve the username from the `whoami` API endpoint")
	}
	body := buf.Bytes()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("unexpected response code: \n%v\n%v", resp.StatusCode, string(body))
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
