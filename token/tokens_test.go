package token

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func TestTokenClient_GetAuthAccessToken(t *testing.T) {

	want := "fake_token"
	output := `
		{
			"access_token": "` + want + `",
			"token_type": "bearer"
		}`

	tests := []struct {
		name    string
		wantErr bool
		URL     string
		status  int
		output  string
	}{
		{
			name:    "valid token response",
			wantErr: false,
		},
		{
			name:    "invalid URL given",
			wantErr: true,
			URL:     "google.com",
		},
		{
			name:    "valid URL but with no access",
			wantErr: true,
			URL:     "https://auth.prod-preview.openshift.io",
		},
		{
			name:    "fail on status code",
			wantErr: true,
			status:  http.StatusNotFound,
		},
		{
			name:    "make code fail on parsing output",
			wantErr: true,
			output:  "foobar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// if no status code given in test case, set the default
				if tt.status == 0 {
					tt.status = http.StatusOK
				}
				w.WriteHeader(tt.status)

				// if the output of the server is not set in testcase, set the default
				if tt.output == "" {
					tt.output = output
				}
				w.Write([]byte(tt.output))
			}))
			defer ts.Close()

			// if the URL is not given in test case then set what is given by user
			if tt.URL == "" {
				tt.URL = ts.URL
			}
			config, _ := configuration.GetData()

			// set the URL given by the temporary server
			config.SetAuthURL(tt.URL)

			c := &TokenClient{}
			got, err := c.GetAuthAccessToken(config)
			if (err != nil) != tt.wantErr {
				t.Errorf("TokenClient.GetAuthAccessToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err != nil && tt.wantErr {
				t.Logf("TokenClient.GetAuthAccessToken() failed with = %v", err)
				return
			}
			if got != want {
				t.Errorf("TokenClient.GetAuthAccessToken() = %v, want %v", got, want)
			}
		})
	}
}
