package openshift_test

import (
	"github.com/fabric8-services/fabric8-tenant-get-token/openshift"
	"github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"os"
	"testing"
)

var templateHeader = `
---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-tenant-team-environments
    version: 1.0.58
    group: io.fabric8.online.packages
  name: fabric8-tenant-team
objects:
`
var projectRequestObject = `
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: run-Project-Description
      openshift.io/display-name: run-Project-Name
      openshift.io/requester: Aslak-User
    labels:
      provider: fabric8
      project: fabric8-tenant-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: ${USER_NAME}-run
`
var roleBindingRestrictionObject = `
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    labels:
      app: fabric8-tenant-che-mt
      provider: fabric8
      version: 2.0.85
      group: io.fabric8.tenant.packages
    name: dsaas-user-access
    namespace: ${USER_NAME}-run
  spec:
    userrestriction:
      users:
      - ${PROJECT_USER}
`

type ServiceTestSuite struct {
	gormsupport.DBTestSuite
}

func TestService(t *testing.T) {
	os.Setenv(resource.Database, "1")
	suite.Run(t, &ServiceTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *ServiceTestSuite) TestInvokePostAndGetCallsForAllObjects() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject + roleBindingRestrictionObject)
	gock.New("http://api.cluster1/").
		Post("/oapi/v1/projectrequests").
		Reply(200)
	gock.New("http://api.cluster1/").
		Get("/oapi/v1/projects/aslak-run").
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
	gock.New("http://api.cluster1/").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), tf.AddNamespaces()).Tenants[0]
	service := testdoubles.NewOSService(
		config,
		testdoubles.AddUser("aslak").
			WithData(testdoubles.NewUserDataWithTenantConfig("", "12345", "")).
			WithToken("abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod(true).ApplyAll([]environment.Type{environment.TypeRun})

	// then
	require.NoError(s.T(), err)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), namespaces, 1)
	assert.Equal(s.T(), "aslak-run", namespaces[0].Name)
	assert.Equal(s.T(), tenant.Ready.String(), namespaces[0].State.String())
}

func (s *ServiceTestSuite) TestDeleteIfThereIsConflict() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + roleBindingRestrictionObject)
	gock.New("http://api.cluster1/").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(409)
	gock.New("http://api.cluster1/").
		Delete("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200)
	gock.New("http://api.cluster1/").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)
	gock.New("http://api.cluster1/").
		Get("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200).
		BodyString(roleBindingRestrictionObject)

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), tf.AddNamespaces()).Tenants[0]
	service := testdoubles.NewOSService(
		config,
		testdoubles.AddUser("aslak").
			WithData(testdoubles.NewUserDataWithTenantConfig("", "12345", "")).
			WithToken("abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod(true).ApplyAll([]environment.Type{environment.TypeRun})

	// then
	require.NoError(s.T(), err)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	require.Len(s.T(), namespaces, 1)
	assert.Equal(s.T(), "aslak-run", namespaces[0].Name)
	assert.Equal(s.T(), tenant.Ready.String(), namespaces[0].State.String())
}

func (s *ServiceTestSuite) TestDeleteAndGet() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject)
	gock.New("http://api.cluster1/").
		Delete("/oapi/v1/projects/aslak-run").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
		Reply(200)

	//namespaceCreator := Ns("aslak-run", environment.TypeRun)
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), tf.AddNamespaces(environment.TypeRun))
	tnnt := fxt.Tenants[0]
	service := testdoubles.NewOSService(
		config,
		testdoubles.AddUser("aslak").
			WithData(testdoubles.NewUserDataWithTenantConfig("", "12345", "")).
			WithToken("abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithDeleteMethod(fxt.Namespaces, true, false).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	repo := tenant.NewDBService(s.DB)
	namespaces, err := repo.GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	assert.Empty(s.T(), namespaces)
	assert.False(s.T(), repo.Exists(tnnt.ID))
}

func (s *ServiceTestSuite) TestNumberOfCallsToCluster() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()
	testdoubles.SetTemplateVersions()

	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, "http://api.cluster1/")
	userCreator := testdoubles.AddUser("developer").WithToken("12345")

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("developer")), tf.AddNamespaces()).Tenants[0]
	service := testdoubles.NewOSService(
		config,
		userCreator,
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod(true).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	// the expected number is number of all objects + 11 get calls to verify that objects are created + 1 to removed admin role binding
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), namespaces, 5)
}

func (s *ServiceTestSuite) TestCreateNewNamespacesWithBaseNameEnding2WhenConflictWithProject() {
	// given
	data, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johndoe")), tf.AddDefaultNamespaces().State(tenant.Provisioning))
	johndoeCalls := 0
	projectRequestCalls := 0
	deleteCalls := 0
	johndoe2Calls := 2

	gock.New("http://api.cluster1").
		Post("/api/v1/projectrequests").
		Reply(409).
		BodyString("{}")
	gock.New("http://api.cluster1").
		Delete("/apis/project.openshift.io/v1/projects/.*").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Times(5).
		Reply(200).
		BodyString("{}")
	gock.New("http://api.cluster1").
		Path(`.*johndoe2.*`).
		SetMatcher(test.SpyOnCalls(&johndoe2Calls)).
		Persist().
		Reply(200).
		BodyString("{}")
	gock.New("http://api.cluster1").
		Path(`.*johndoe[^2].*`).
		SetMatcher(test.SpyOnCalls(&johndoeCalls)).
		Persist().
		Reply(200).
		BodyString("{}")
	gock.New("http://api.cluster1").
		Post("/oapi/v1/projectrequests").
		SetMatcher(test.SpyOnCalls(&projectRequestCalls)).
		Times(10).
		Reply(200).
		BodyString("{}")

	user := &client.UserDataAttributes{}
	config := NewConfigForUser(data, user, "clusterUser", "clusterToken", "http://api.cluster1/")
	config.HTTPTransport = http.DefaultTransport
	objsNumber := len(tmplObjects(s.T(), data))
	repo := tenant.NewDBService(s.DB)

	// when
	_, err := RawInitTenant(context.Background(), config, fxt.Tenants[0], "12345", repo, true)

	// then
	require.NoError(s.T(), err)
	// the number of calls should be equal to the number of parsed objects plus one call that removes admin role from user's namespace
	assert.Equal(s.T(), objsNumber-10, johndoeCalls)
	assert.Equal(s.T(), 10, projectRequestCalls)
	assert.Equal(s.T(), 5, deleteCalls)
	assert.Equal(s.T(), objsNumber-2, johndoe2Calls)
	updatedTnnt, err := repo.GetTenant(fxt.Tenants[0].ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "johndoe2", updatedTnnt.NsBaseName)
}

func (s *ServiceTestSuite) TestCreateNewNamespacesWithBaseNameEnding3WhenFailsAnd2Exists() {
	// given
	data, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenantsNamed("johndoe", "johndoe2"), tf.AddDefaultNamespaces().State(tenant.Provisioning))

	gock.New("http://api.cluster1").
		Post("/api/v1/namespaces/johndoe-jenkins/persistentvolumeclaims").
		Reply(403).
		BodyString("{}")
	gock.New("http://api.cluster1").
		Persist().
		Reply(200).
		BodyString("{}")

	user := &client.UserDataAttributes{}
	config := openshift.NewConfigForUser(data, user, "clusterUser", "clusterToken", "http://api.cluster1/")
	config.HTTPTransport = http.DefaultTransport
	repo := tenant.NewDBService(s.DB)

	// when
	_, err := openshift.RawInitTenant(context.Background(), config, fxt.Tenants[0], "12345", repo, true)

	// then
	require.NoError(s.T(), err)
	// the number of calls should be equal to the number of parsed objects plus one call that removes admin role from user's namespace
	updatedTnnt, err := repo.GetTenant(fxt.Tenants[0].ID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "johndoe3", updatedTnnt.NsBaseName)
}