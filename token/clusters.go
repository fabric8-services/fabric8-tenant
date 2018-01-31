package token

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/auth"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

type ClusterResolver func(ctx context.Context, target string) (Cluster, error)

type Cluster struct {
	APIURL     string
	ConsoleURL string
	MetricsURL string
	AppDNS     string

	User  string
	Token string
}

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

type ClusterClient interface {
	Get(context.Context) ([]*Cluster, error)
}

// NewAuthClusterClient creates a Resolver that rely on the Auth service to retrieve tokens
func NewAuthClusterClient(config AuthClientConfig, serviceToken string, token Resolver, decode Decode) ClusterClient {
	return &clusterClient{config: config, serviceToken: serviceToken, token: token, decode: decode}
}

type clusterClient struct {
	config       AuthClientConfig
	serviceToken string
	token        Resolver
	decode       Decode
}

func (c *clusterClient) Get(ctx context.Context) ([]*Cluster, error) {
	authclient, err := CreateClient(c.config)
	if err != nil {
		return nil, err
	}
	authclient.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: c.serviceToken,
					Type:  "Bearer"}}})

	res, err := authclient.ShowClusters(ctx, auth.ShowClustersPath())
	if err != nil {
		return nil, errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := validateError(authclient, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", c.config.GetAuthURL())
	}

	clusters, err := authclient.DecodeClusterList(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", c.config.GetAuthURL())
	}

	var cls []*Cluster
	for _, cluster := range clusters.Data {

		cuser, ctoken, err := c.token(ctx, cluster.APIURL, c.serviceToken, c.decode)
		if err != nil {
			return nil, errors.Wrapf(err, "Unable to resolve token for cluster %v", cluster.APIURL)
		}
		cls = append(cls, &Cluster{
			APIURL:     cluster.APIURL,
			AppDNS:     cluster.AppDNS,
			ConsoleURL: cluster.ConsoleURL,
			MetricsURL: cluster.MetricsURL,
			User:       cuser,
			Token:      ctoken,
		})
	}

	return cls, nil
}
