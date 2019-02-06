package testdoubles

import (
	"context"
	"fmt"
	vcrrecorder "github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/auth"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"strings"
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
		FeatureLevel: ptr.String(auth.InternalFeatureLevel),
	}
}

func SetTemplateVersions() {
	environment.VersionFabric8TenantCheMtFile = "234bcd"
	environment.VersionFabric8TenantCheQuotasFile = "zyx098"
	environment.VersionFabric8TenantUserFile = "345cde"
	environment.VersionFabric8TenantDeployFile = "456def"
	environment.VersionFabric8TenantJenkinsFile = "567efg"
	environment.VersionFabric8TenantJenkinsQuotasFile = "yxw987"
}

type UserModifier func(user *auth.User)

func AddUser(osUsername string) UserModifier {
	return func(user *auth.User) {
		user.OpenShiftUsername = osUsername
	}
}

func (m UserModifier) WithData(data *authclient.UserDataAttributes) UserModifier {
	return func(user *auth.User) {
		m(user)
		user.UserData = data
	}
}

func (m UserModifier) WithToken(osUserToken string) UserModifier {
	return func(user *auth.User) {
		m(user)
		user.OpenShiftUserToken = osUserToken
	}
}

var defaultClusterToken, _ = test.NewToken(
	map[string]interface{}{
		"sub": "devtools-sre",
	},
	"../test/private_key.pem",
)
var DefaultClusterMapping = SingleClusterMapping("http://api.cluster1/", "clusterUser", defaultClusterToken.Raw)

func SingleClusterMapping(url, user, token string) cluster.ForType {
	return func(envType environment.Type) cluster.Cluster {
		return cluster.Cluster{
			APIURL: url,
			User:   user,
			Token:  token,
		}
	}
}

func (m UserModifier) NewUserInfo(nsBaseName string) UserInfo {
	user := &auth.User{ID: uuid.NewV4()}
	m(user)
	return UserInfo{
		OsUsername:  user.OpenShiftUsername,
		OsUserToken: user.OpenShiftUserToken,
		NsBaseName:  nsBaseName,
	}
}

func NewOSService(config *configuration.Data, modifier UserModifier, repository tenant.Repository) *openshift.ServiceBuilder {
	user := &auth.User{ID: uuid.NewV4()}
	modifier(user)
	envService := environment.NewServiceForUserData(user.UserData)
	ctx := openshift.NewServiceContext(context.Background(), config, DefaultClusterMapping, user.OpenShiftUsername,
		environment.RetrieveUserName(user.OpenShiftUsername), openshift.TokenResolverForUser(user))

	return openshift.NewBuilderWithTransport(ctx, repository, http.DefaultTransport, envService)
}

type UserInfo struct {
	OsUsername  string
	OsUserToken string
	NsBaseName  string
}

var DefaultUserInfo = UserInfo{
	OsUsername:  "developer",
	OsUserToken: "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8",
	NsBaseName:  "developer",
}

func SingleTemplatesObjects(t *testing.T, config *configuration.Data, envType environment.Type, clusterMapping cluster.ForType, userInfo UserInfo) environment.Objects {
	envService := environment.NewService()

	ctx := openshift.NewServiceContext(
		context.Background(), config, clusterMapping, userInfo.OsUsername, userInfo.NsBaseName, func(cluster cluster.Cluster) string {
			return userInfo.OsUserToken
		})

	nsTypeService := openshift.NewEnvironmentTypeService(envType, ctx, envService)
	_, objects, err := nsTypeService.GetEnvDataAndObjects(func(objects environment.Object) bool {
		return true
	})
	require.NoError(t, err)
	return objects
}

func SingleTemplatesObjectsWithDefaults(t *testing.T, config *configuration.Data, envType environment.Type) environment.Objects {
	return SingleTemplatesObjects(t, config, envType, DefaultClusterMapping, DefaultUserInfo)
}

func RetrieveObjects(t *testing.T, config *configuration.Data, clusterMapping cluster.ForType, userInfo UserInfo, envTypes ...environment.Type) environment.Objects {
	var objs environment.Objects
	for _, envType := range envTypes {
		objects := SingleTemplatesObjects(t, config, envType, clusterMapping, userInfo)
		objs = append(objs, objects...)
	}
	return objs
}

func AllDefaultObjects(t *testing.T, config *configuration.Data) environment.Objects {
	return RetrieveObjects(t, config, DefaultClusterMapping, DefaultUserInfo, environment.DefaultEnvTypes...)
}

func MockPostRequestsToOS(calls *int, cluster string, envs []environment.Type, nsBaseName string) {
	cluster = test.Normalize(cluster)
	for _, env := range envs {
		namespaceName := nsBaseName
		if env != environment.TypeUser {
			namespaceName = nsBaseName + "-" + env.String()
		} else {
			gock.New(cluster).
				Delete(fmt.Sprintf("/oapi/v1/namespaces/%s/rolebindings/%s", nsBaseName, "admin")).
				SetMatcher(test.SpyOnCalls(calls)).
				Reply(200).
				BodyString(`{"status": {"phase":"Active"}}`)
		}

		gock.New(cluster).
			Get("/oapi/v1/projects/" + namespaceName).
			SetMatcher(test.SpyOnCalls(calls)).
			Reply(404)

		basePath := fmt.Sprintf(".*(%s|projectrequests).*", namespaceName)
		gock.New(cluster).
			Post(basePath).
			SetMatcher(test.SpyOnCalls(calls)).
			Persist().
			Reply(200).
			BodyString("{}")

		gock.New(cluster).
			Get(basePath).
			SetMatcher(test.SpyOnCalls(calls)).
			Persist().
			Reply(200).
			BodyString(`{"status": {"phase":"Active"}}`)
	}
}

func MockPatchRequestsToOS(calls *int, cluster string) {
	cluster = test.Normalize(cluster)
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
	listOfKinds := ""
	for _, kind := range openshift.AllKindsToClean {
		listOfKinds += fmt.Sprintf("%ss|", strings.ToLower(kind))
	}
	cluster = test.Normalize(cluster)
	gock.New(cluster).
		Delete(fmt.Sprintf(`.*\/(%s)(\/|$).*`, listOfKinds[:len(listOfKinds)-1])).
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString("{}")
	gock.New(cluster).
		Get(`.*\/(persistentvolumeclaims)\/.*`).
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(404)
	gock.New(cluster).
		Get(`\/api\/v1\/namespaces\/[^\/].+\/services`).
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString(`{"items": []}`)
}

func MockRemoveRequestsToOS(calls *int, cluster string) {
	cluster = test.Normalize(cluster)
	gock.New(cluster).
		Delete(`.*\/(namespaces|projects)\/.*`).
		SetMatcher(test.SpyOnCalls(calls)).
		Persist().
		Reply(200).
		BodyString("{}")
}

func ExpectedNumberOfCallsWhenPost(t *testing.T, config *configuration.Data) int {
	objectsInTemplates := AllDefaultObjects(t, config)
	return len(objectsInTemplates) + NumberOfGetChecks(objectsInTemplates) + 1 + 5
}

func ExpectedNumberOfCallsWhenClean(t *testing.T, config *configuration.Data, envTypes ...environment.Type) int {
	objectsInTemplates := RetrieveObjects(t, config, DefaultClusterMapping, DefaultUserInfo, envTypes...)
	pvcNumber := CountObjectsThat(objectsInTemplates, isOfKind(environment.ValKindPersistentVolumeClaim))
	cleanAllOps := (len(openshift.AllKindsToClean)) * len(envTypes)
	return NumberOfObjectsToClean(objectsInTemplates) + pvcNumber + cleanAllOps
}

func ExpectedNumberOfCallsWhenPatch(t *testing.T, config *configuration.Data, envTypes ...environment.Type) int {
	numberOfObjects := 0
	for _, envType := range envTypes {
		numberOfObjects += len(SingleTemplatesObjectsWithDefaults(t, config, envType))
	}
	return 2 * (numberOfObjects - len(envTypes))
}

func NumberOfGetChecks(objects environment.Objects) int {
	return CountObjectsThat(
		objects,
		isOfKind(environment.ValKindNamespace, environment.ValKindProjectRequest, environment.ValKindProject, environment.ValKindResourceQuota))
}

func NumberOfObjectsToClean(objects environment.Objects) int {
	return CountObjectsThat(objects, isOfKind(openshift.AllKindsToClean...))
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
	environment.VersionFabric8TenantCheMtFile = version
	environment.VersionFabric8TenantCheQuotasFile = version
	environment.VersionFabric8TenantUserFile = version
	environment.VersionFabric8TenantDeployFile = version
	environment.VersionFabric8TenantJenkinsFile = version
	environment.VersionFabric8TenantJenkinsQuotasFile = version
}

func GetMappedVersions(envTypes ...environment.Type) map[environment.Type]string {
	mappedTemplates := environment.RetrieveMappedTemplates()
	typesWithVersion := map[environment.Type]string{}
	for _, envType := range envTypes {
		typesWithVersion[envType] = mappedTemplates[envType].ConstructCompleteVersion()
	}
	return typesWithVersion
}

func MockCommunicationWithAuth(cluster string, otherClusters ...string) {
	clusterList := ""
	var clusters []string
	clusters = append(otherClusters, cluster)
	for _, cl := range clusters {
		gock.New("http://authservice").
			Get("/api/token").
			Persist().
			MatchParam("for", cl+"/").
			MatchParam("force_pull", "false").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
			Reply(200).
			BodyString(`{ 
			"access_token": "jA0ECQMCVXRaahUCbbtg0sDRAe2Yy9f/is3vsRXD2xDjZtSOBcQG/IvvzFA40TbMmTyo3csGKsEs+xr3TOBzHX/oIRLpO74d0mDHy+c6e72eRitmKNssb7pTyx9fD+v1FqJ/PTGFtWVp9XjbtXybkoCQHjYtt7i4di2tfm6rSHCuKB3FA/4a59sN542R3fxS488PKhCLPfq1RbHVi4mg47dsrOlVrJITpNsEH1RTL8w6+pX6FjossE+qB3QwZwopPeNOMUn1vF2O6BfhVO80RyLLHr8EEigBhpxTb6we+IFYztToPJXjNS4LYEVz74zAjyrkqXBNrND09jSCo0oQOtUtuzuv76lJQVe0tLwjM7AwFHHDgQvUykdnHg8jyJtI5OYWypmHpnyHay4ocMRO/hHcx7a+Lbz9Uj40cdtl3+XRUOoJt01OcgK7sKqwG4UCoRzh/RN/vYDEgH8CBrnZ67qG+0cKxdqayPJXrX3gtukXcDnHntiSRCCbryrYlAoTb1ypghdUCWgRWVEsSXDG/lgNW3DMEEnZ+HV23l9fGGudfPY=",
			"token_type": "bearer",
			"username": "devtools-sre"
    }`)

		gock.New(cl).
			Get("/apis/user.openshift.io/v1/users/~").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Persist().
			Reply(200).
			BodyString(`{
     "kind":"User",
     "apiVersion":"user.openshift.io/v1",
     "metadata":{
       "name":"devtools-sre",
       "selfLink":"/apis/user.openshift.io/v1/users/tenant_service",
       "uid":"bcdd0b29-123d-11e8-a8bc-b69930b94f5c",
       "resourceVersion":"814",
       "creationTimestamp":"2018-02-15T10:48:20Z"
     },
     "identities":[],
     "groups":[]
   }`)

		clusterList += fmt.Sprintf(`
        {
          "name": "cluster_name",
          "api-url": "%s/",
          "console-url": "%s/console/",
          "metrics-url": "%s/",
          "logging-url": "%s/",
          "app-dns": "foo",
          "capacity-exhausted": false
        },`, cl, cl, cl, cl)

	}

	gock.New("http://authservice").
		Get("/api/clusters/").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
		Persist().
		Reply(200).
		BodyString(fmt.Sprintf(`{
      "data":[
        %s
      ]
    }`, clusterList[:len(clusterList)-1]))
}
