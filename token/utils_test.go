package token

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func Test_validateError(t *testing.T) {

	config, err := configuration.GetData()
	if err != nil {
		t.Fatalf("could not retrieve configuration: %v", err)
	}

	authclient, err := CreateClient(config)
	if err != nil {
		t.Fatalf("%v", err)
	}

	tests := []struct {
		name    string
		res     *http.Response
		wantErr bool
	}{
		{
			name: "status ok",
			res: &http.Response{
				StatusCode: http.StatusOK,
			},
		},
		{
			name: "unmarshalling should fail",
			res: &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte("foo"))),
			},
			wantErr: true,
		},
		{
			name: "error response should be parsed rightly",
			res: &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body: ioutil.NopCloser(bytes.NewReader([]byte(`
				{
					"errors": [
						{
								"code": "jwt_security_error",
								"detail": "JWT validation failed: token contains an invalid number of segments",
								"id": "BEO45Wxi",
								"status": "401",
								"title": "Unauthorized"
						}
					]
				}`))),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateError(authclient, tt.res); (err != nil) != tt.wantErr {
				t.Errorf("validateError() error = %v, wantErr %v", err, tt.wantErr)
			} else if err != nil && tt.wantErr {
				t.Logf("validateError() failed with error = %v", err)
			}
		})
	}
}
