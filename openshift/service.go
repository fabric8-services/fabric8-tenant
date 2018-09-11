package openshift

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
)

// Service the Openshift service interface
type Service interface {
	DeleteNamespace(ctx context.Context, config Config, namespace string) error
}

// NewService return a new Openshift service implementation
func NewService(clientOptions ...configuration.HTTPClientOption) Service {
	return openShiftService{
		clientOptions: clientOptions,
	}
}

// openShiftService the OpenShift service implementation
type openShiftService struct {
	clientOptions []configuration.HTTPClientOption
}

func (s openShiftService) DeleteNamespace(ctx context.Context, config Config, namespace string) error {
	opts := ApplyOptions{Config: config}
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/apis/project.openshift.io/v1/projects/%s", strings.TrimSuffix(opts.MasterURL, "/"), namespace), nil)
	if err != nil {
		return errors.Wrapf(err, "unable to delete the namespace from the API endpoint")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	client := http.DefaultClient
	for _, applyOption := range s.clientOptions {
		applyOption(client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "unable to delete the namespace from the API endpoint")
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "unable to delete the namespace from the API endpoint")
	}
	body := buf.Bytes()
	// only report error if the operation did not return a 2xx (OK) or 403 (Forbidden)
	// actually, Openshift checks if the namespace belongs to the user even before checking if it exists...
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		// let's log the error, nonetheless
		log.Warn(ctx, map[string]interface{}{"namespace": namespace, "message": body}, "failed to delete namespace (but it probably does not exist)")
	} else if resp.StatusCode >= 400 { // other errors
		return errors.Errorf("unable to delete the namespace from the API endpoint: status=%v message=%s", resp.StatusCode, string(body))
	}
	log.Info(ctx, map[string]interface{}{"namespace": namespace}, "deleted namespace")
	return nil
}
