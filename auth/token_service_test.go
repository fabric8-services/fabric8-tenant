package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func TestClusterTokenClient_Get(t *testing.T) {
	want := "fake_token"
	fake_user := "fake_user"
	output := fmt.Sprintf(`{
			"access_token": "%s",
			"token_type": "bearer",
			"username": "%s"
		}`, want, fake_user)
	accessToken := "fake_accesstoken"
	cluster := "fake_cluster"

	type fields struct {
		ClusterToken string
	}
	type args struct {
		accessToken *string
		cluster     *string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
		URL     string
		status  int
		output  string
		decoder auth.Decode
	}{
		{
			name:    "access token empty",
			wantErr: true,
		},
		{
			name:    "cluster url is empty",
			wantErr: true,
			args:    args{accessToken: &accessToken},
		},
		{
			name:    "misformed URL",
			URL:     "google.com",
			args:    args{accessToken: &accessToken, cluster: &cluster},
			wantErr: true,
		},
		{
			name:    "bad status code",
			args:    args{accessToken: &accessToken, cluster: &cluster},
			wantErr: true,
			status:  http.StatusNotFound,
		},
		{
			name:    "make code fail on parsing output",
			args:    args{accessToken: &accessToken, cluster: &cluster},
			wantErr: true,
			output:  "foobar",
		},
		{
			name:    "valid output",
			args:    args{accessToken: &accessToken, cluster: &cluster},
			wantErr: false,
		},
		{
			name:    "invalid encrypted token",
			args:    args{accessToken: &accessToken, cluster: &cluster},
			wantErr: true,
			decoder: func(data string) (*string, error) { return nil, fmt.Errorf("Could not decrypt") },
		},
	}
	for _, testData := range tests {
		t.Run(testData.name, func(t *testing.T) {
			// given
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// if no status code given in test case, set the default
				if testData.status == 0 {
					testData.status = http.StatusOK
				}
				w.WriteHeader(testData.status)

				// if the output of the server is not set in testcase, set the default
				if testData.output == "" {
					testData.output = output
				}
				w.Write([]byte(testData.output))
			}))
			defer ts.Close()

			// if the URL is not given in test case then set what is given by user
			if testData.URL == "" {
				testData.URL = ts.URL
			}

			// set the URL given by the temporary server
			os.Setenv("F8_AUTH_URL", testData.URL)
			config, err := configuration.GetData()
			require.NoError(t, err)

			resolver := auth.NewTokenResolver(config)

			if testData.decoder == nil {
				testData.decoder = auth.PlainTextToken
			}
			// when
			_, got, err := resolver(context.Background(), testData.args.cluster, testData.args.accessToken, testData.decoder)
			// then
			if testData.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, want, *got)
		})
	}
}
