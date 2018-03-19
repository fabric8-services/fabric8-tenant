package openshift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/errors"
	"github.com/fabric8-services/fabric8-wit/log"
	errs "github.com/pkg/errors"
)

type request struct {
	method      string
	url         string
	body        string
	bearerToken string
}

// executeRequest executes/submits a request to the given URL using the given HTTP method and authorization token.
// returns the response body or an error if the response status is not "200 OK"
func executeRequest(ctx context.Context, request request, clientOptions ...configuration.HTTPClientOption) ([]byte, error) {
	req, err := http.NewRequest(request.method, request.url, nil)
	if err != nil {
		return nil, errs.Wrapf(err, "unable to initialize request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+request.bearerToken)
	if request.body != "" {
		req.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(request.body)))
	}
	client := http.DefaultClient
	for _, applyOption := range clientOptions {
		applyOption(client)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errs.Wrapf(err, "unable to execute request")
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, errs.Wrapf(err, "unable to read response body")
	}
	respBody := buf.Bytes()
	if resp.StatusCode >= 400 { // client (4xx) or server (5xx) error
		es := ErrorStatus{}
		err := json.Unmarshal(respBody, &es)
		if err != nil {
			return nil, errs.Wrapf(err, "unable to read error response with status %d: %s", resp.StatusCode, string(respBody))
		}
		log.Error(ctx, map[string]interface{}{
			"http_request":       fmt.Sprintf("%s %s", request.method, request.url),
			"http_error_code":    es.Code,
			"http_error_message": es.Message},
			"operation failed")
		switch es.Code {
		case http.StatusNotFound:
			return nil, errors.NewOpenShiftObjectNotFoundError(es.Message)
		case http.StatusConflict:
			return nil, errors.NewOpenShiftObjectConflictError(es.Message)
		default:
			return nil, errs.Errorf("%s", es.Message)
		}
	}
	return respBody, nil
}

// ErrorStatus the response status for a request that failed
type ErrorStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}
