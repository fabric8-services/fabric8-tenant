package cluster

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/pkg/errors"
	"io/ioutil"
	"strings"
	"sync"
	"time"
)

// Cluster a cluster
type Cluster struct {
	APIURL            string
	ConsoleURL        string
	MetricsURL        string
	LoggingURL        string
	AppDNS            string
	CapacityExhausted bool

	User  string
	Token string
}

type GetCluster func(ctx context.Context, target string) (Cluster, error)

// Service the interface for the cluster service
type Service interface {
	GetCluster(ctx context.Context, target string) (Cluster, error)
	GetClusters(ctx context.Context) []Cluster
	Start() error
	Stop()
}

// Stats some stats about the cached data, for verifying during the tests, at first.
type Stats struct {
	CacheHits      int
	CacheMissed    int
	CacheRefreshes int
}

type clusterService struct {
	authService      auth.Service
	clientOptions    []configuration.HTTPClientOption
	cacheRefresher   *time.Ticker
	cacheRefreshLock *sync.RWMutex
	cacheHits        int
	cacheMissed      int
	cacheRefreshes   int
	cachedClusters   []Cluster
}

// NewClusterService creates an instance of service that using the Auth service retrieves information about clusters
func NewClusterService(refreshInt time.Duration, authService auth.Service, options ...configuration.HTTPClientOption) Service {
	// setup a ticker to refresh the cluster cache at regular intervals
	cacheRefresher := time.NewTicker(refreshInt)
	service := &clusterService{
		authService:      authService,
		clientOptions:    options,
		cacheRefresher:   cacheRefresher,
		cacheRefreshLock: &sync.RWMutex{},
	}
	return service
}

func (s *clusterService) Start() error {
	//immediately load the list of clusters before returning
	err := s.refreshCache(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load the list of clusters during service initialization: %s", err)
	}
	go func() {
		for range s.cacheRefresher.C { // while the `cacheRefresh` ticker is running
			err := s.refreshCache(context.Background())
			if err != nil {
				log.Error(nil, map[string]interface{}{
					"err": err,
				}, "failed to load the list of clusters")
			}
		}
	}()
	return nil
}

func (s *clusterService) GetCluster(ctx context.Context, target string) (Cluster, error) {
	for _, cluster := range s.GetClusters(ctx) {
		if cleanURL(target) == cleanURL(cluster.APIURL) {
			return cluster, nil
		}
	}
	return Cluster{}, fmt.Errorf("unable to resolve cluster")
}

func (s *clusterService) GetClusters(ctx context.Context) []Cluster {
	s.cacheRefreshLock.RLock()
	defer func() {
		s.cacheRefreshLock.RUnlock()
		log.Debug(ctx, nil, "read lock released")
	}()
	log.Debug(ctx, nil, "read lock acquired")
	clusters := make([]Cluster, len(s.cachedClusters))
	copy(clusters, s.cachedClusters)
	return clusters

}

func (s *clusterService) Stop() {
	s.cacheRefresher.Stop()
}

func cleanURL(url string) string {
	if !strings.HasSuffix(url, "/") {
		return url + "/"
	}
	return url
}

func (s *clusterService) refreshCache(ctx context.Context) error {
	log.Debug(ctx, nil, "refreshing cached list of clusters...")
	defer log.Debug(ctx, nil, "refreshed cached list of clusters.")
	s.cacheRefreshes = s.cacheRefreshes + 1
	client, err := s.authService.NewSaClient()
	if err != nil {
		return err
	}

	res, err := client.ShowClusters(ctx, authclient.ShowClustersPath())
	if err != nil {
		return errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := auth.ValidateResponse(ctx, client, res)
	if validationerror != nil {
		return errors.Wrapf(validationerror, "error from server %q", s.authService.GetAuthURL())
	}

	clusters, err := client.DecodeClusterList(res)
	if err != nil {
		return errors.Wrapf(err, "error from server %q", s.authService.GetAuthURL())
	}

	var cls []Cluster
	for _, cluster := range clusters.Data {
		// resolve/obtain the cluster token
		clusterUser, clusterToken, err := s.authService.ResolveSaToken(ctx, cluster.APIURL)
		if err != nil {
			return errors.Wrapf(err, "Unable to resolve token for cluster %v", cluster.APIURL)
		}
		// verify the token
		_, err = WhoAmI(ctx, cluster.APIURL, clusterToken, s.clientOptions...)
		if err != nil {
			return errors.Wrapf(err, "token retrieved for cluster %v is invalid", cluster.APIURL)
		}

		cls = append(cls, Cluster{
			APIURL:            cluster.APIURL,
			AppDNS:            cluster.AppDNS,
			ConsoleURL:        cluster.ConsoleURL,
			MetricsURL:        cluster.MetricsURL,
			LoggingURL:        cluster.LoggingURL,
			CapacityExhausted: cluster.CapacityExhausted,

			User:  clusterUser,
			Token: clusterToken,
		})
	}
	// lock to avoid concurrent writes
	s.cacheRefreshLock.Lock()
	defer func() {
		s.cacheRefreshLock.Unlock()
		log.Debug(ctx, nil, "write lock released")
	}()
	log.Debug(ctx, nil, "write lock acquired")
	s.cachedClusters = cls // only replace at the end of this function and within a Write lock scope, i.e., when all retrieved clusters have been processed
	return nil
}
