package cluster

import (
	"context"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-wit/log"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
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

func cleanURL(url string) string {
	if !strings.HasSuffix(url, "/") {
		return url + "/"
	}
	return url
}

// Service the interface for the cluster service
type Service interface {
	GetClusters(context.Context) ([]*Cluster, error)
	Stats() Stats
	Stop()
}

// Stats some stats about the cached data, for verifying during the tests, at first.
type Stats struct {
	CacheHits      int
	CacheMissed    int
	CacheRefreshes int
}

// NewService creates a Resolver that rely on the Auth service to retrieve tokens
func NewService(authURL string, clustersRefreshDelay time.Duration, serviceToken string, resolveToken token.Resolve, decode token.Decode, options ...configuration.HTTPClientOption) Service {
	// setup a ticker to refresh the cluster cache at regular intervals
	cacheRefresher := time.NewTicker(clustersRefreshDelay)
	s := &clusterService{
		authURL:          authURL,
		serviceToken:     serviceToken,
		resolveToken:     resolveToken,
		decode:           decode,
		cacheRefresher:   cacheRefresher,
		cacheRefreshLock: &sync.Mutex{},
		clientOptions:    options}
	go func() {
		for range cacheRefresher.C { // while the `cacheRefresh` ticker is running
			s.cacheRefreshLock.Lock()
			defer s.cacheRefreshLock.Unlock()
			s.refreshCache(context.Background())
		}
	}()

	return s
}

type clusterService struct {
	authURL          string
	serviceToken     string
	resolveToken     token.Resolve
	cacheRefresher   *time.Ticker
	cacheRefreshLock *sync.Mutex
	cacheHits        int
	cacheMissed      int
	cacheRefreshes   int
	cachedClusters   []*Cluster
	decode           token.Decode
	clientOptions    []configuration.HTTPClientOption
}

func (s *clusterService) GetClusters(ctx context.Context) ([]*Cluster, error) {
	// force fetch if nothing was loaded in the cache
	if len(s.cachedClusters) == 0 {
		log.Debug(ctx, nil, "cache not loaded. Attempting to obtain a lock...")
		// lock to avoid concurrent fetches
		s.cacheRefreshLock.Lock()
		log.Debug(ctx, nil, "lock acquired")
		defer s.cacheRefreshLock.Unlock()
		// once lock is freed, check if fetching is still needed
		// (in case another request already triggered the refresh while this one acquired the lock)
		if len(s.cachedClusters) == 0 {
			s.cacheMissed++
			err := s.refreshCache(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			log.Debug(ctx, nil, "ah, cache is now loaded!")
			s.cacheHits++ // cache was hit since loaded by another rountine in the mean time
		}
	} else {
		log.Debug(ctx, nil, "cache is already loaded!")
		s.cacheHits++ // cache was hit immediately
	}
	return s.cachedClusters, nil
}

func (s *clusterService) Stop() {
	s.cacheRefresher.Stop()
}

func (s *clusterService) refreshCache(ctx context.Context) error {
	log.Info(ctx, nil, "refreshing cached list of clusters...")
	s.cacheRefreshes = s.cacheRefreshes + 1
	client, err := auth.NewClient(s.authURL, s.serviceToken, s.clientOptions...)
	if err != nil {
		return err
	}
	client.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: s.serviceToken,
					Type:  "Bearer"}}})

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
		return errors.Wrapf(validationerror, "error from server %q", s.authURL)
	}

	clusters, err := client.DecodeClusterList(res)
	if err != nil {
		return errors.Wrapf(err, "error from server %q", s.authURL)
	}

	var cls []*Cluster
	for _, cluster := range clusters.Data {
		// resolve/obtain the cluster token
		clusterUser, clusterToken, err := s.resolveToken(ctx, cluster.APIURL, s.serviceToken, false, s.decode) // can't use "forcePull=true" to validate the `tenant service account` token since it's encrypted on auth
		if err != nil {
			return errors.Wrapf(err, "Unable to resolve token for cluster %v", cluster.APIURL)
		}
		// verify the token
		_, err = openshift.WhoAmI(ctx, cluster.APIURL, clusterToken, s.clientOptions...)
		if err != nil {
			return errors.Wrapf(err, "token retrieved for cluster %v is invalid", cluster.APIURL)
		}

		cls = append(cls, &Cluster{
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
	s.cachedClusters = cls // only replace at the end, i.e., when all retrieved clusters have been processed
	return nil
}

func (s *clusterService) Stats() Stats {
	return Stats{
		CacheHits:      s.cacheHits,
		CacheMissed:    s.cacheMissed,
		CacheRefreshes: s.cacheRefreshes,
	}
}
