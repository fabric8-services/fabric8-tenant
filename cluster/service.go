package cluster

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/token"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

// Cluster a cluster
type Cluster struct {
	APIURL     string
	ConsoleURL string
	MetricsURL string
	LoggingURL string
	AppDNS     string

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
}

// NewService creates a Resolver that rely on the Auth service to retrieve tokens
func NewService(authURL string, serviceToken string, resolveToken token.Resolve, decode token.Decode, options ...configuration.HTTPClientOption) Service {
	return &clusterService{authURL: authURL, serviceToken: serviceToken, resolveToken: resolveToken, decode: decode, clientOptions: options}
}

type clusterService struct {
	authURL       string
	clientOptions []configuration.HTTPClientOption
	serviceToken  string
	resolveToken  token.Resolve
	decode        token.Decode
}

func (s *clusterService) GetClusters(ctx context.Context) ([]*Cluster, error) {
	client, err := auth.NewClient(s.authURL, s.serviceToken, s.clientOptions...)
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

	validationerror := auth.ValidateResponse(ctx, client, res)
	if validationerror != nil {
		return nil, errors.Wrapf(validationerror, "error from server %q", s.authURL)
	}

	clusters, err := client.DecodeClusterList(res)
	if err != nil {
		return nil, errors.Wrapf(err, "error from server %q", s.authURL)
	}

	var cls []*Cluster
	for _, cluster := range clusters.Data {
		// resolve/obtain the cluster token
		clusterUser, clusterToken, err := s.resolveToken(ctx, cluster.APIURL, s.serviceToken, false, s.decode) // can't use "forcePull=true" to validate the `tenant service account` token since it's encrypted on auth
		if err != nil {
			return nil, errors.Wrapf(err, "Unable to resolve token for cluster %v", cluster.APIURL)
		}
		// verify the token
		_, err = openshift.WhoAmI(ctx, cluster.APIURL, clusterToken, s.clientOptions...)
		if err != nil {
			return nil, errors.Wrapf(err, "token retrieved for cluster %v is invalid", cluster.APIURL)
		}

		cls = append(cls, &Cluster{
			APIURL:     cluster.APIURL,
			AppDNS:     cluster.AppDNS,
			ConsoleURL: cluster.ConsoleURL,
			MetricsURL: cluster.MetricsURL,
			LoggingURL: cluster.LoggingURL,
			User:       clusterUser,
			Token:      clusterToken,
		})
	}
	return cls, nil
}
