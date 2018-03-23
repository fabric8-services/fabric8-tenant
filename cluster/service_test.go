package cluster_test

import (
	"context"
	"sync"
	"testing"
	"time"

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

func TestGetClusters(t *testing.T) {
	// given
	r, err := recorder.New("../test/data/cluster/resolve_cluster", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	authURL := "http://authservice"
	resolveToken := token.NewResolve(authURL, configuration.WithRoundTripper(r))
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
			time.Hour, // don't want to interfer with the refresher here
			saToken.Raw,
			resolveToken,
			token.NewGPGDecypter("foo"),
			configuration.WithRoundTripper(r),
		)
		defer clusterService.Stop()
		// when
		clusters, err := clusterService.GetClusters(context.Background())
		// then
		require.NoError(t, err)
		require.Len(t, clusters, 1)
		assert.Equal(t, "https://api.cluster1/", clusters[0].APIURL)
		assert.Equal(t, "foo", clusters[0].AppDNS)
		assert.Equal(t, "http://console.cluster1/console/", clusters[0].ConsoleURL)
		assert.Equal(t, "http://metrics.cluster1/", clusters[0].MetricsURL)
		assert.Equal(t, "http://logging.cluster1/", clusters[0].LoggingURL)
		assert.Equal(t, saToken.Raw, clusters[0].Token) // see decode_test.go for decoded value of data in yaml file
		assert.Equal(t, "tenant_service", clusters[0].User)
	})

	t.Run("cache", func(t *testing.T) {
		t.Run("not loaded", func(t *testing.T) {
			// given
			clusterService := cluster.NewService(
				authURL,
				time.Hour, //
				saToken.Raw,
				resolveToken,
				token.NewGPGDecypter("foo"),
				configuration.WithRoundTripper(r),
			)
			defer clusterService.Stop()
			<-time.After(time.Second) // make sure the cache is not loaded
			// when
			clusters, err := clusterService.GetClusters(context.Background())
			// then
			require.NoError(t, err)
			require.Len(t, clusters, 1)
			stats := clusterService.Stats()
			assert.Equal(t, 1, stats.CacheRefreshes)
			assert.Equal(t, 0, stats.CacheHits)
			assert.Equal(t, 1, stats.CacheMissed)
		})

		t.Run("loaded", func(t *testing.T) {
			// given
			clusterService := cluster.NewService(
				authURL,
				time.Second, //
				saToken.Raw,
				resolveToken,
				token.NewGPGDecypter("foo"),
				configuration.WithRoundTripper(r),
			)
			defer clusterService.Stop()
			<-time.After(time.Duration(2 * int(time.Second))) // make sure the cache is loaded
			// when
			clusters, err := clusterService.GetClusters(context.Background())
			// then
			require.NoError(t, err)
			require.Len(t, clusters, 1)
			stats := clusterService.Stats()
			assert.Equal(t, 1, stats.CacheRefreshes)
			assert.Equal(t, 1, stats.CacheHits)
			assert.Equal(t, 0, stats.CacheMissed)
		})

		t.Run("concurrent access upon start", func(t *testing.T) {
			// given
			clusterService := cluster.NewService(
				authURL,
				time.Hour, // make sure the refresher does not interfer
				saToken.Raw,
				resolveToken,
				token.NewGPGDecypter("foo"),
				configuration.WithRoundTripper(r),
			)
			defer clusterService.Stop()
			// when 5 requests arrive at the same time (more or less)
			wg := sync.WaitGroup{}
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					clusters, err := clusterService.GetClusters(context.Background())
					// then
					require.NoError(t, err)
					require.Len(t, clusters, 1)
				}()
			}
			wg.Wait()
			stats := clusterService.Stats()
			assert.Equal(t, 1, stats.CacheRefreshes)
			assert.Equal(t, 4, stats.CacheHits)
			assert.Equal(t, 1, stats.CacheMissed)
		})
	})
}
