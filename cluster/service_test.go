package cluster_test

import (
	"context"
	"math/rand"
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

func TestResolveCluster(t *testing.T) {

	// given
	r, err := recorder.New("../test/data/cluster/resolve_cluster.fast", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	authURL := "http://fast.authservice"
	resolveToken := token.NewResolve(authURL, configuration.WithRoundTripper(r))
	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)
	clusterService, err := cluster.NewService(
		authURL,
		time.Hour, // make sure the refresher does not interfer
		saToken.Raw,
		resolveToken,
		token.NewGPGDecypter("foo"),
		configuration.WithRoundTripper(r),
	)
	require.NoError(t, err)
	defer clusterService.Stop()

	t.Run("cluster - end slash", func(t *testing.T) {
		// given
		target := "http://api.cluster1"
		resolve := cluster.NewResolve(clusterService)
		// when
		found, err := resolve(context.Background(), target)
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})

	t.Run("cluster - no end slash", func(t *testing.T) {
		// given
		target := "https://api.cluster2"
		resolve := cluster.NewResolve(clusterService)
		// when
		found, err := resolve(context.Background(), target+"/")
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})

	t.Run("both slash", func(t *testing.T) {
		// given
		target := "http://api.cluster1/"
		resolve := cluster.NewResolve(clusterService)
		// when
		found, err := resolve(context.Background(), target)
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})

	t.Run("no slash", func(t *testing.T) {
		// given
		target := "https://api.cluster2"
		resolve := cluster.NewResolve(clusterService)
		// when
		found, err := resolve(context.Background(), target)
		// then
		require.NoError(t, err)
		assert.Contains(t, found.APIURL, target)
	})
}

func TestGetClusters(t *testing.T) {
	// given
	r, err := recorder.New("../test/data/cluster/resolve_cluster.slow", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	authURL := "http://slow.authservice"
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
		clusterService, err := cluster.NewService(
			authURL,
			time.Hour, // don't want to interfer with the refresher here
			saToken.Raw,
			resolveToken,
			token.NewGPGDecypter("foo"),
			configuration.WithRoundTripper(r),
		)
		require.NoError(t, err)
		defer clusterService.Stop()
		// when
		clusters, err := clusterService.GetClusters(context.Background())
		// then
		require.NoError(t, err)
		require.Len(t, clusters, 2)
		assert.Equal(t, "http://api.cluster1/", clusters[0].APIURL)
		assert.Equal(t, "foo", clusters[0].AppDNS)
		assert.Equal(t, "http://console.cluster1/console/", clusters[0].ConsoleURL)
		assert.Equal(t, "http://metrics.cluster1/", clusters[0].MetricsURL)
		assert.Equal(t, "http://logging.cluster1/", clusters[0].LoggingURL)
		assert.Equal(t, saToken.Raw, clusters[0].Token) // see decode_test.go for decoded value of data in yaml file
		assert.Equal(t, "tenant_service", clusters[0].User)
		assert.Equal(t, false, clusters[0].CapacityExhausted)
	})

	t.Run("cache", func(t *testing.T) {

		t.Run("concurrent reads", func(t *testing.T) {
			// given
			clusterService, err := cluster.NewService(
				authURL,
				time.Second, // make sure the refresher does not interfer
				saToken.Raw,
				resolveToken,
				token.NewGPGDecypter("foo"),
				configuration.WithRoundTripper(r),
			)
			require.NoError(t, err)
			defer clusterService.Stop()
			// when 5 requests arrive at the same time (more or less)
			wg := sync.WaitGroup{}
			readersCount := 50
			results := make(chan int, readersCount)
			for i := 0; i < readersCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					// wait a random amount of time
					time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond)
					clusters, err := clusterService.GetClusters(context.Background())
					// then
					require.NoError(t, err)
					results <- len(clusters)
					require.Len(t, clusters, 2)
				}()
			}
			wg.Wait()
			close(results)
			for i := 0; i < readersCount; i++ {
				result := <-results
				assert.Equal(t, 2, result)
			}
		})
	})
}
