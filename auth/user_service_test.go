package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	uuid "github.com/satori/go.uuid"
)

func TestUserProfileClient_GetUserCluster(t *testing.T) {
	want := "https://fake-cluster.com"
	token := "random"
	wantOutput := fmt.Sprintf(`{
	  "data": {
		"attributes": {
		  "cluster": "%s"
		}
	  }
	}`, want)

	tests := []struct {
		name    string
		user    uuid.UUID
		want    string
		wantErr bool
		URL     string
		status  int
		output  string
	}{
		{
			name:    "normal input to see if cluster is parsed",
			want:    want,
			wantErr: false,
			user:    uuid.NewV4(),
		},
		{
			name:    "misformed URL",
			URL:     "foo.com",
			user:    uuid.NewV4(),
			wantErr: true,
		},
		{
			name:    "bad status code",
			wantErr: true,
			status:  http.StatusNotFound,
			user:    uuid.NewV4(),
		},
		{
			name:    "make code fail on parsing output",
			wantErr: true,
			output:  "foobar",
			user:    uuid.NewV4(),
		},
	}

	for _, testData := range tests {
		t.Run(testData.name, func(t *testing.T) {
			// given
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, filepath.Join("/api/users/"+testData.user.String()), r.URL.EscapedPath())
				require.True(t, strings.Contains(r.Header.Get("Authorization"), token), "Expected request to contain Authorization header")
				// if no status code given in test case, set the default
				if testData.status == 0 {
					testData.status = http.StatusOK
				}
				w.WriteHeader(testData.status)

				// if the output of the server is not set in testcase, set the default
				if testData.output == "" {
					testData.output = wantOutput
				}
				w.Write([]byte(testData.output))
			}))
			// if the URL is not given in test case then set what is given by user
			if testData.URL == "" {
				testData.URL = ts.URL
			}
			// set the URL given by the temporary server
			os.Setenv("F8_AUTH_URL", testData.URL)
			config, err := configuration.GetData()
			require.NoError(t, err)
			s := auth.NewUserService(config, token)
			// when
			user, err := s.GetUser(context.Background(), testData.user)
			// then
			if testData.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, user)
			assert.Equal(t, testData.want, *user.Cluster)
		})
	}
}
