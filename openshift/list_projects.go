package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// ListProjects returns the name of the projects of the user identified by the given token
func ListProjects(ctx context.Context, clusterURL string, token string, clientOptions ...configuration.HTTPClientOption) ([]string, error) {
	respBody, err := executeRequest(ctx,
		request{
			method:      "GET",
			url:         fmt.Sprintf("%s/oapi/v1/projects", strings.TrimSuffix(clusterURL, "/")),
			bearerToken: token},
		clientOptions...)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve the user's projects from the API endpoint")
	}
	var prjcts Projects
	err = yaml.Unmarshal(respBody, &prjcts)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve the user's projects from the API endpoint")
	}
	prjNames := make([]string, len(prjcts.Items))
	for i, p := range prjcts.Items {
		prjNames[i] = p.Metadata.Name
	}
	log.Debug(ctx, map[string]interface{}{"project_names": prjNames}, "retrieved projects on tenant cluster")
	return prjNames, nil
}

// Projects the user's projects on the cluster
type Projects struct {
	Items []Project
}

// Project a user's project on the cluster
type Project struct {
	Metadata struct {
		Name string
	}
}
