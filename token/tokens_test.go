package token

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func TestClusterTokenClient_Get(t *testing.T) {

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
			config, err := configuration.GetData()
			if err != nil {
				t.Fatalf("could not retrieve configuration: %v", err)
			}

			// set the URL given by the temporary server
			config.SetAuthURL(tt.URL)

			c := &ClusterTokenClient{}
			err = c.Get(config)
			got := c.AuthServiceAccountToken
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterTokenClient.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			} else if err != nil && tt.wantErr {
				t.Logf("ClusterTokenClient.Get() failed with = %v", err)
				return
			}
			if got != want {
				t.Errorf("ClusterTokenClient.Get() = %v, want %v", got, want)
			}
		})
	}
}

func Test_validateError(t *testing.T) {
	type args struct {
		status int
		body   []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "status ok",
			args:    args{status: http.StatusOK},
			wantErr: false,
		},
		{
			name:    "unmarshalling should fail",
			args:    args{body: []byte("foobar")},
			wantErr: true,
		},
		{
			name: "return proper error",
			args: args{
				status: http.StatusNotFound,
				body: []byte(`
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
					}`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateError(tt.args.status, tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("validateError() error = %v, wantErr %v", err, tt.wantErr)
			} else if tt.wantErr && err != nil {
				t.Logf("validateError() failed with error = %v", err)
			}
		})
	}
}

func TestOpenShiftTokenClient_Get(t *testing.T) {
	want := "fake_token"
	output := `
		{
			"access_token": "` + want + `",
			"token_type": "bearer"
		}`
	accessToken := "fake_accesstoken"
	cluster := "fake_cluster"

	type fields struct {
		OpenShiftToken string
	}
	type args struct {
		accessToken string
		cluster     string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		URL     string
		status  int
		output  string
	}{
		{
			name:    "access token empty",
			wantErr: true,
		},
		{
			name:    "cluster url is empty",
			wantErr: true,
			args:    args{accessToken: accessToken},
		},
		{
			name:    "misformed URL",
			URL:     "google.com",
			args:    args{accessToken: accessToken, cluster: cluster},
			wantErr: true,
		},
		{
			name:    "bad status code",
			args:    args{accessToken: accessToken, cluster: cluster},
			wantErr: true,
			status:  http.StatusNotFound,
		},
		{
			name:    "make code fail on parsing output",
			args:    args{accessToken: accessToken, cluster: cluster},
			wantErr: true,
			output:  "foobar",
		},
		{
			name:    "valid output",
			args:    args{accessToken: accessToken, cluster: cluster},
			wantErr: false,
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

			config, err := configuration.GetData()
			if err != nil {
				t.Fatalf("could not retrieve configuration: %v", err)
			}

			// set the URL given by the temporary server
			config.SetAuthURL(tt.URL)

			z := &OpenShiftTokenClient{}
			if err := z.Get(config, tt.args.accessToken, tt.args.cluster); (err != nil) != tt.wantErr {
				t.Errorf("OpenShiftTokenClient.Get() error = %v, wantErr %v", err, tt.wantErr)
			} else if err != nil && tt.wantErr {
				t.Logf("OpenShiftTokenClient.Get() failed with = %v", err)
				return
			}
			got := z.OpenShiftToken
			if got != want {
				t.Errorf("OpenShiftTokenClient.Get() = %v, want %v", got, want)
			}
		})
	}
}
