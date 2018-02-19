package cluster_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterCache(t *testing.T) {

	t.Run("cluster - end slash", func(t *testing.T) {
		// given
		target := "A"
		resolve := cluster.NewResolve([]*cluster.Cluster{
			{APIURL: "X"},
			{APIURL: target + "/"},
		})
		// when
		found, err := resolve(context.Background(), target)
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})
	t.Run("cluster - no end slash", func(t *testing.T) {
		// given
		target := "A"
		resolve := cluster.NewResolve([]*cluster.Cluster{
			{APIURL: "X"},
			{APIURL: target},
		})
		// when
		found, err := resolve(context.Background(), target+"/")
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})

	t.Run("both slash", func(t *testing.T) {
		// given
		target := "A"
		resolve := cluster.NewResolve([]*cluster.Cluster{
			{APIURL: "X"},
			{APIURL: target + "/"},
		})
		// when
		found, err := resolve(context.Background(), target+"/")
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})

	t.Run("no slash", func(t *testing.T) {
		// given
		target := "A"
		resolve := cluster.NewResolve([]*cluster.Cluster{
			{APIURL: "X"},
			{APIURL: target + "/"},
		})
		// when
		found, err := resolve(context.Background(), target+"/")
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})
}

func TestResolveCluster(t *testing.T) {
	tests := []struct {
		name   string
		status int
		count  int
		output string
	}{
		{
			name:   "Fully complete",
			status: 200,
			count:  1,
			output: `{"data": [{"api-url": "http://a.com", "app-dns": "b.com"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				if tt.output != "" {
					w.Write([]byte(tt.output))
				}
			}))

			defer ts.Close()

			os.Setenv("F8_AUTH_URL", ts.URL)

			config, err := configuration.GetData()
			if err != nil {
				t.Fatal(err)
			}
			tr := func(ctx context.Context, target, token string, decode token.Decode) (user, accessToken string, err error) {
				return "foo", "bar", nil
			}
			cs := cluster.NewService(config, "aa", tr, token.PlainText)
			clusters, err := cs.GetClusters(context.Background())
			require.NoError(t, err)
			assert.Len(t, clusters, tt.count)
		})
	}
}
