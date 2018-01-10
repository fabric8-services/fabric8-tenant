package token

import (
	"context"
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
	accessToken := "fake_accesstoken"
	cluster := "fake_cluster"

	type fields struct {
		ClusterToken string
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

			c := &ClusterTokenClient{}
			c.Config = config
			c.AccessToken = tt.args.accessToken
			got, err := c.Get(context.Background(), tt.args.cluster)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterTokenClient.Get() error = %v, wantErr %v", err, tt.wantErr)
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
