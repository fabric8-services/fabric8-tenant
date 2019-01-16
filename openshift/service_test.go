package openshift_test

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/assertion"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-wit/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
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
		Reply(404)
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
	assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
		HasNumberOfNamespaces(1).
		HasNamespaceOfTypeThat(environment.TypeRun).
		HasName("aslak-run").
		HasState(tenant.Ready)
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
	assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
		HasNumberOfNamespaces(1).
		HasNamespaceOfTypeThat(environment.TypeRun).
		HasName("aslak-run").
		HasState(tenant.Ready)
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
	err := service.WithDeleteMethod(fxt.Namespaces, true, false, true).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
		DoesNotExist().
		HasNoNamespace()
}

func (s *ServiceTestSuite) TestNumberOfCallsToCluster() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()
	testdoubles.SetTemplateVersions()

	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "developer")
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
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), namespaces, 5)
}

func (s *ServiceTestSuite) TestCreateNewNamespacesWithBaseNameEnding2WhenConflictsWithProject() {
	// given
	config, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johndoe")), tf.AddNamespaces())
	deleteCalls := 0

	gock.New("http://api.cluster1").
		Get("/oapi/v1/projects/johndoe-che").
		Reply(200).
		BodyString("{}")
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johndoe2")
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johndoe")
	gock.New("http://api.cluster1").
		Delete("/oapi/v1/projects/.*").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Times(5).
		Reply(200).
		BodyString("{}")

	repo := tenant.NewDBService(s.DB).NewTenantRepository(fxt.Tenants[0].ID)
	service := testdoubles.NewOSService(
		config,
		testdoubles.AddUser("johndoe").WithToken("12345"),
		repo)

	// when
	err := service.WithPostMethod(true).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, deleteCalls)
	assertion.AssertTenant(s.T(), repo).
		HasNsBaseName("johndoe2").
		HasNumberOfNamespaces(5)
}

func (s *ServiceTestSuite) TestCreateNewNamespacesWithBaseNameEnding3WhenConflictsWithProjectAndWith2Exists() {
	// given
	config, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johndoe"), tf.SingleWithName("johndoe2")), tf.AddNamespaces())
	deleteCalls := 0

	gock.New("http://api.cluster1").
		Get("/oapi/v1/projects/johndoe-che").
		Reply(200).
		BodyString("{}")
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johndoe3")
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johndoe")
	gock.New("http://api.cluster1").
		Delete("/oapi/v1/projects/.*").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Times(5).
		Reply(200).
		BodyString("{}")

	repo := tenant.NewDBService(s.DB).NewTenantRepository(fxt.Tenants[0].ID)
	service := testdoubles.NewOSService(
		config,
		testdoubles.AddUser("johndoe").WithToken("12345"),
		repo)

	// when
	err := service.WithPostMethod(true).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 5, deleteCalls)
	assertion.AssertTenant(s.T(), repo).
		HasNsBaseName("johndoe3").
		HasNumberOfNamespaces(5)
}

func (s *ServiceTestSuite) TestCreateNewNamespacesWithNormalBaseNameWhenFailsLimitRangesReturnsConflict() {
	// given
	defer gock.Off()
	config, reset := test.LoadTestConfig(s.T())
	defer reset()
	testdoubles.SetTemplateVersions()

	deleteCalls := 0
	gock.New(test.ClusterURL).
		Post("/api/v1/namespaces/johndoe-jenkins/limitranges").
		Reply(409).
		BodyString("{}")
	gock.New(test.ClusterURL).
		Delete("/api/v1/namespaces/johndoe-jenkins/limitranges/resource-limits").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Times(1).
		Reply(200).
		BodyString("{}")
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johndoe")
	userCreator := testdoubles.AddUser("johndoe").WithToken("12345")

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johndoe")), tf.AddNamespaces()).Tenants[0]
	service := testdoubles.NewOSService(
		config,
		userCreator,
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod(true).ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	assert.Equal(s.T(), 1, deleteCalls)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), namespaces, 5)
}
