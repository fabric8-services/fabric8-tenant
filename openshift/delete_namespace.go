package openshift

import (
	"context"
	"fmt"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

// DeleteNamespace deletes a namespace on the target Openshift cluster
func DeleteNamespace(name string, clusterURL string, token string, clientOptions ...configuration.HTTPClientOption) error {
	_, err := executeRequest(context.Background(),
		request{
			method:      "DELETE",
			url:         fmt.Sprintf("%s/oapi/v1/projects/%s", strings.TrimSuffix(clusterURL, "/"), name),
			bearerToken: token},
		clientOptions...)
	if err != nil {
		return errors.Wrapf(err, "failed to delete the namespace '%s' on the cluster with URL '%s", name, clusterURL)
	}

	return nil
}
