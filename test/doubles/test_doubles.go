package testdoubles

import (
	"context"
	vcrrecorder "github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/utils"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func NewAuthService(t *testing.T, cassetteFile, authURL string, options ...recorder.Option) (*auth.Service, func()) {
	authService, _, cleanup := NewAuthServiceWithRecorder(t, cassetteFile, authURL, options...)
	return authService, cleanup
}

func NewAuthServiceWithRecorder(t *testing.T, cassetteFile, authURL string, options ...recorder.Option) (*auth.Service, *vcrrecorder.Recorder, func()) {
	var clientOptions []configuration.HTTPClientOption
	var r *vcrrecorder.Recorder
	var err error
	if cassetteFile != "" {
		r, err = recorder.New(cassetteFile, options...)
		require.NoError(t, err)
		clientOptions = append(clientOptions, configuration.WithRoundTripper(r))
	}
	resetBack := test.SetEnvironments(test.Env("F8_AUTH_URL", authURL))
	config, err := configuration.GetData()
	require.NoError(t, err)

	authService := &auth.Service{
		Config:        config,
		ClientOptions: clientOptions,
	}
	return authService, r, func() {
		if r != nil {
			err := r.Stop()
			require.NoError(t, err)
		}
		resetBack()
	}
}

func NewUserDataWithTenantConfig(templatesRepo, templatesRepoBlob, templatesRepoDir string) *authclient.UserDataAttributes {
	return &authclient.UserDataAttributes{
		ContextInformation: map[string]interface{}{
			"tenantConfig": map[string]interface{}{
				"templatesRepo":     templatesRepo,
				"templatesRepoBlob": templatesRepoBlob,
				"templatesRepoDir":  templatesRepoDir,
			}},
		FeatureLevel: utils.String("internal"),
	}
}

func SetTemplateVersions() {
	environment.VersionFabric8TenantCheFile = "123abc"
	environment.VersionFabric8TenantCheMtFile = "234bcd"
	environment.VersionFabric8TenantCheQuotasFile = "zyx098"
	environment.VersionFabric8TenantUserFile = "345cde"
	environment.VersionFabric8TenantDeployFile = "456def"
	environment.VersionFabric8TenantJenkinsFile = "567efg"
	environment.VersionFabric8TenantJenkinsQuotasFile = "yxw987"
}

type TenantCreator func() (*tenant.Tenant, []*tenant.Namespace)

type NamespaceCreator func(tenantId uuid.UUID) *tenant.Namespace

func Ns(nsName string, envType environment.Type) NamespaceCreator {
	return func(tenantId uuid.UUID) *tenant.Namespace {
		return &tenant.Namespace{
			ID:       uuid.NewV4(),
			TenantID: tenantId,
			Name:     nsName,
			Type:     envType,
		}
	}
}

func WithTenant(id uuid.UUID, nsCreators ...NamespaceCreator) TenantCreator {
	return func() (*tenant.Tenant, []*tenant.Namespace) {
		var nss []*tenant.Namespace
		for _, createNs := range nsCreators {
			nss = append(nss, createNs(id))
		}

		return &tenant.Tenant{ID: id}, nss
	}
}

type UserCreator func() *auth.User

func WithUser(data *authclient.UserDataAttributes, osUsername, osUserToken string) UserCreator {
	return func() *auth.User {
		return &auth.User{
			UserData:           data,
			OpenShiftUserToken: osUserToken,
			OpenShiftUsername:  osUsername,
		}
	}
}

func SingleClusterMapping(url, user, token string) cluster.ForType {
	return func(envType environment.Type) cluster.Cluster {
		return cluster.Cluster{
			APIURL: url,
			User:   user,
			Token:  token,
		}
	}
}

func NewOSService(config *configuration.Data, createTenant TenantCreator, clusterMapping cluster.ForType, createUser UserCreator) (*openshift.ServiceBuilder, *gormsupport.DBStub) {
	user := createUser()
	envService := environment.NewServiceForUserData(user.UserData)
	ctx := openshift.NewServiceContext(context.Background(), config, clusterMapping, user.OpenShiftUsername, user.OpenShiftUserToken)

	tennt, namespaces := createTenant()
	nsRepo, dbStub := gormsupport.NewDBServiceStub(tennt, namespaces)
	return openshift.NewBuilderWithTransport(ctx, nsRepo.NewTenantRepository(tennt.ID), http.DefaultTransport, envService), dbStub
}
