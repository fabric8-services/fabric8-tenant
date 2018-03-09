package cluster_test

import (
	"context"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
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
	// given
	r, err := recorder.New("../test/data/cluster/resolve_cluster", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	authURL := "http://authservice"
	resolveToken := token.NewResolve(authURL, configuration.WithRoundTripper(r.Transport))
	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// given
		clusterService := cluster.NewService(
			authURL,
			saToken.Raw,
			resolveToken,
			token.NewGPGDecypter("foo"),
			configuration.WithRoundTripper(r.Transport),
		)
		// when
		clusters, err := clusterService.GetClusters(context.Background())
		// then
		require.NoError(t, err)
		require.Len(t, clusters, 1)
		assert.Equal(t, "http://api.cluster1/", clusters[0].APIURL)
		assert.Equal(t, "foo", clusters[0].AppDNS)
		assert.Equal(t, "http://console.cluster1/console/", clusters[0].ConsoleURL)
		assert.Equal(t, "http://metrics.cluster1/", clusters[0].MetricsURL)
		assert.Equal(t, "http://logging.cluster1/", clusters[0].LoggingURL)
		assert.Equal(t, saToken.Raw, clusters[0].Token) // see decode_test.go for decoded value of data in yaml file
		assert.Equal(t, "tenant_service", clusters[0].User)

	})
}
