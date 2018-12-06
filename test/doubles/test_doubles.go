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
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-tenant/utils"
	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"testing"
)

func NewAuthService(t *testing.T, cassetteFile, authURL, saToken string, options ...recorder.Option) (auth.Service, func()) {
	authService, _, cleanup := NewAuthServiceWithRecorder(t, cassetteFile, authURL, saToken, options...)
	return authService, cleanup
}

func NewAuthServiceWithRecorder(t *testing.T, cassetteFile, authURL, saToken string, options ...recorder.Option) (auth.Service, *vcrrecorder.Recorder, func()) {
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

	authService := auth.NewAuthServiceWithToken(config, saToken, clientOptions...)

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

func (c UserCreator) NewUserInfo(nsBaseName string) UserInfo {
	user := c()
	return UserInfo{
		OsUsername:  user.OpenShiftUsername,
		OsUserToken: user.OpenShiftUserToken,
		NsBaseName:  nsBaseName,
	}
}

func NewOSService(
	t *testing.T, config *configuration.Data, createTenant TenantCreator, clusterMapping cluster.ForType, createUser UserCreator, db *gorm.DB) *openshift.ServiceBuilder {
	user := createUser()
	envService := environment.NewServiceForUserData(user.UserData)
	ctx := openshift.NewServiceContext(
		context.Background(), config, clusterMapping, user.OpenShiftUsername, user.OpenShiftUserToken, environment.RetrieveUserName(user.OpenShiftUsername))

	tennt, namespaces := createTenant()
	tf.NewTestFixture(t, db, tf.Tenants(1, func(fxt *tf.TestFixture, idx int) error {
		fxt.Tenants[0] = tennt
		return nil
	}), tf.Namespaces(len(namespaces), func(fxt *tf.TestFixture, idx int) error {
		fxt.Namespaces[idx] = namespaces[idx]
		return nil
	}))
	return openshift.NewBuilderWithTransport(ctx, tenant.NewDBService(db).NewTenantRepository(tennt.ID), http.DefaultTransport, envService)
}

type UserInfo struct {
	OsUsername  string
	OsUserToken string
	NsBaseName  string
}

func SingleTemplatesObjects(t *testing.T, config *configuration.Data, envType environment.Type, clusterMapping cluster.ForType, userInfo UserInfo) environment.Objects {
	envService := environment.NewService()

	ctx := openshift.NewServiceContext(
		context.Background(), config, clusterMapping, userInfo.OsUsername, userInfo.OsUserToken, userInfo.NsBaseName)

	nsTypeService := openshift.NewEnvironmentTypeService(envType, ctx, envService)
	_, objects, err := nsTypeService.GetEnvDataAndObjects(func(objects environment.Object) bool {
		return true
	})
	require.NoError(t, err)
	return objects
}

func AllTemplatesObjects(t *testing.T, config *configuration.Data, clusterMapping cluster.ForType, userInfo UserInfo) environment.Objects {
	var objs environment.Objects
	for _, envType := range environment.DefaultEnvTypes {
		objects := SingleTemplatesObjects(t, config, envType, clusterMapping, userInfo)
		objs = append(objs, objects...)
	}
	return objs
}

func MockPostRequestsToOS(calls *int, cluster string) {
	gock.New(cluster).
		Post("").
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString("{}")

	gock.New(cluster).
		Get("").
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)

	gock.New(cluster).
		Delete("").
		SetMatcher(test.SpyOnCalls(calls)).
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
}

func MockPatchRequestsToOS(calls *int, cluster string) {
	gock.New(cluster).
		Path("").
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString("{}")

	gock.New(cluster).
		Get("").
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
}

func MockCleanRequestsToOS(calls *int, cluster string) {
	gock.New(cluster).
		Delete("").
		SetMatcher(test.ExpectRequest(
			test.HasUrlMatching(`.*\/(persistentvolumeclaims|configmaps)\/.*`),
			test.SpyOnCallsMatchFunc(calls))).
		Persist().
		Reply(200).
		BodyString("{}")
}

func MockRemoveRequestsToOS(calls *int, cluster string) {
	gock.New(cluster).
		Delete("").
		SetMatcher(test.ExpectRequest(
			test.HasUrlMatching(`.*\/(namespaces|projects)\/.*`),
			test.SpyOnCallsMatchFunc(calls))).
		Persist().
		Reply(200).
		BodyString("{}")
}

func ExpectedNumberOfCallsWhenPost(t *testing.T, config *configuration.Data, clusterMapping cluster.ForType, userInfo UserInfo) int {
	objectsInTemplates := AllTemplatesObjects(t, config, clusterMapping, userInfo)
	return len(objectsInTemplates) + NumberOfGetChecks(objectsInTemplates) + 1
}

func NumberOfGetChecks(objects environment.Objects) int {
	return CountObjectsThat(
		objects,
		isOfKind(environment.ValKindNamespace, environment.ValKindProjectRequest, environment.ValKindProject, environment.ValKindResourceQuota))
}

func NumberOfObjectsToClean(objects environment.Objects) int {
	return CountObjectsThat(objects, isOfKind(environment.ValKindPersistenceVolumeClaim, environment.ValKindConfigMap))
}

func NumberOfObjectsToRemove(objects environment.Objects) int {
	return CountObjectsThat(objects, isOfKind(environment.ValKindNamespace, environment.ValKindProjectRequest, environment.ValKindProject))
}

func CountObjectsThat(objects environment.Objects, is func(map[interface{}]interface{}) bool) int {
	count := 0
	for _, obj := range objects {
		if is(obj) {
			count++
		}
	}
	return count
}

func isOfKind(kinds ...string) func(map[interface{}]interface{}) bool {
	return func(vs map[interface{}]interface{}) bool {
		kind := environment.GetKind(vs)
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
}

func SetTemplateSameVersion(version string) {
	environment.VersionFabric8TenantCheFile = version
	environment.VersionFabric8TenantCheMtFile = version
	environment.VersionFabric8TenantCheQuotasFile = version
	environment.VersionFabric8TenantUserFile = version
	environment.VersionFabric8TenantDeployFile = version
	environment.VersionFabric8TenantJenkinsFile = version
	environment.VersionFabric8TenantJenkinsQuotasFile = version
}
