package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

// ClusterResolver a cluster resolver
type ClusterResolver func(ctx context.Context, target string) (Cluster, error)

// Cluster a cluster
type Cluster struct {
	APIURL     string
	ConsoleURL string
	MetricsURL string
	AppDNS     string

	User  string
	Token string
}

// NewCachedClusterResolver returns a new ClusterResolved
func NewCachedClusterResolver(clusters []*Cluster) ClusterResolver {
	return func(ctx context.Context, target string) (Cluster, error) {
		for _, cluster := range clusters {
			if cleanURL(target) == cleanURL(cluster.APIURL) {
				return *cluster, nil
			}
		}
		return Cluster{}, fmt.Errorf("unable to resovle cluster")
	}
}

func cleanURL(url string) string {
	if !strings.HasSuffix(url, "/") {
		return url + "/"
	}
	return url
}

// ClusterService the interface for the cluster service
type ClusterService interface {
	GetClusters(context.Context) ([]*Cluster, error)
}

// NewClusterService creates a Resolver that rely on the Auth service to retrieve tokens
func NewClusterService(config ClientConfig, serviceToken string, token TokenResolver, decode Decode) ClusterService {
	return &clusterService{config: config, serviceToken: serviceToken, token: token, decode: decode}
}

type clusterService struct {
	config       ClientConfig
	serviceToken string
	token        TokenResolver
	decode       Decode
}

func (s *clusterService) GetClusters(ctx context.Context) ([]*Cluster, error) {
	client, err := NewClient(s.config)
	if err != nil {
		return nil, err
	}
	client.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: s.serviceToken,
					Type:  "Bearer"}}})

	res, err := client.ShowClusters(ctx, authclient.ShowClustersPath())
	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := validateError(client, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.config.GetAuthURL())
	}

	clusters, err := client.DecodeClusterList(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.config.GetAuthURL())
	}

	var cls []*Cluster
	for _, cluster := range clusters.Data {
		clusterUser, clusterToken, err := s.token(ctx, &cluster.APIURL, &s.serviceToken, s.decode)
		if err != nil {
			return nil, errors.Wrapf(err, "Unable to resolve token for cluster %v", cluster.APIURL)
		}
		cls = append(cls, &Cluster{
			APIURL:     cluster.APIURL,
			AppDNS:     cluster.AppDNS,
			ConsoleURL: cluster.ConsoleURL,
			MetricsURL: cluster.MetricsURL,
			User:       *clusterUser,
			Token:      *clusterToken,
		})
	}
	return cls, nil
}
