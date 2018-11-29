package stub

import (
	"context"
	"crypto/rsa"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/satori/go.uuid"
)

type ClusterService struct {
	APIURL string
	User   string
	Token  string
}

func (s *ClusterService) Start() error {
	return nil
}

func (s *ClusterService) GetCluster(ctx context.Context, target string) (cluster.Cluster, error) {
	return cluster.Cluster{
		APIURL: s.APIURL,
		User:   s.User,
		Token:  s.Token,
	}, nil
}

func (s *ClusterService) GetClusters(ctx context.Context) []cluster.Cluster {
	cl, _ := s.GetCluster(ctx, "")
	return []cluster.Cluster{cl}

}

func (s *ClusterService) Stop() {
}

type AuthService struct {
	TenantID           uuid.UUID
	OpenShiftUsername  string
	OpenShiftUserToken string
	ClusterURL         string
}

func (s *AuthService) GetUser(ctx context.Context) (*auth.User, error) {
	return &auth.User{
		ID:                 s.TenantID,
		OpenShiftUsername:  s.OpenShiftUsername,
		OpenShiftUserToken: s.OpenShiftUserToken,
		UserData: &authclient.UserDataAttributes{
			IdentityID:   ptr.String(s.TenantID.String()),
			Cluster:      ptr.String(s.ClusterURL),
			Username:     ptr.String(s.OpenShiftUsername),
			Email:        ptr.String(s.OpenShiftUsername + "@redhat.com"),
			FeatureLevel: ptr.String("internal"),
		},
	}, nil
}

func (s *AuthService) GetAuthURL() string {
	return ""
}
func (s *AuthService) NewSaClient() (*authclient.Client, error) {
	return &authclient.Client{}, nil
}

func (s *AuthService) ResolveUserToken(ctx context.Context, target, userToken string) (user, accessToken string, err error) {
	return "", "", nil
}

func (s *AuthService) ResolveSaToken(ctx context.Context, target string) (username, accessToken string, err error) {
	return "", "", nil
}

func (s *AuthService) GetPublicKeys() ([]*rsa.PublicKey, error) {
	return nil, nil
}
