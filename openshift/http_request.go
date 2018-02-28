package openshift

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httputil"

	"github.com/fabric8-services/fabric8-wit/log"
)

// execute executes the HTTP request on the given method/URL, using the given token
func execute(ctx context.Context, client *http.Client, method, url, token string) (status int, body []byte, err error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// debug only
	if log.IsDebug() {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Warn(ctx, map[string]interface{}{"error": err.Error()}, "failed to dump the request for debug logging")
		} else {
			log.Debug(ctx, map[string]interface{}{"request_dump": string(dump)}, "request dump")
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	body = buf.Bytes()
	status = resp.StatusCode
	return status, body, err
}
