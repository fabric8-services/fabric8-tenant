package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// ListProjects returns the name of the projects of the user identified by the given token
func ListProjects(ctx context.Context, clusterURL string, token string, clientOptions ...configuration.HTTPClientOption) ([]string, error) {
	respBody, err := executeRequest(fmt.Sprintf("%s/oapi/v1/projects", strings.TrimSuffix(clusterURL, "/")), token, clientOptions...)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve the user's projects from the API endpoint")
	}
	var prjcts projects
	err = yaml.Unmarshal(respBody, &prjcts)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve the user's projects from the API endpoint")
	}
	prjNames := make([]string, len(prjcts.Items))
	for i, p := range prjcts.Items {
		prjNames[i] = p.Metadata.Name
	}
	return prjNames, nil
}

type projects struct {
	Items []project
}

type project struct {
	Metadata struct {
		Name string
	}
}
