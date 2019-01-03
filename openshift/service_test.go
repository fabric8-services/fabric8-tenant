package openshift_test

import (
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	. "github.com/fabric8-services/fabric8-tenant/test"
	. "github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
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
	config, reset := LoadTestConfig(s.T())
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject + roleBindingRestrictionObject)
	gock.New("http://starter.com").
		Post("/oapi/v1/projectrequests").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/projects/aslak-run").
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), true, tf.AddNamespaces()).Tenants[0]
	service := NewOSService(
		config,
		SingleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod().ApplyAll([]environment.Type{environment.TypeRun})

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
	config, reset := LoadTestConfig(s.T())
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + roleBindingRestrictionObject)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(409)
	gock.New("http://starter.com").
		Delete("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200).
		BodyString(roleBindingRestrictionObject)

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), true, tf.AddNamespaces()).Tenants[0]
	service := NewOSService(
		config,
		SingleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod().ApplyAll([]environment.Type{environment.TypeRun})

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
	config, reset := LoadTestConfig(s.T())
	defer reset()

	tok, err := NewToken(
		map[string]interface{}{
			"sub": "clusterUser",
		},
		"../test/private_key.pem",
	)
	require.NoError(s.T(), err)

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject)
	gock.New("http://starter.com").
		Delete("/oapi/v1/projects/aslak-run").
		SetMatcher(ExpectRequest(HasJWTWithSub("clusterUser"))).
		Reply(200)

	//namespaceCreator := Ns("aslak-run", environment.TypeRun)
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("aslak")), true, tf.AddNamespaces(environment.TypeRun))
	tnnt := fxt.Tenants[0]
	service := NewOSService(
		config,
		SingleClusterMapping("http://starter.com", "clusterUser", tok.Raw),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"),
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err = service.WithDeleteMethod(fxt.Namespaces, true).ApplyAll(environment.DefaultEnvTypes)

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
	config, reset := LoadTestConfig(s.T())
	defer reset()
	SetTemplateVersions()

	calls := 0
	MockPostRequestsToOS(&calls, "http://my.cluster")
	clusterMapping := SingleClusterMapping("http://my.cluster", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8")
	userCreator := WithUser(&authclient.UserDataAttributes{}, "developer", "12345")

	tnnt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("developer")), true, tf.AddNamespaces()).Tenants[0]
	service := NewOSService(
		config,
		clusterMapping,
		userCreator,
		tenant.NewDBService(s.DB).NewTenantRepository(tnnt.ID))

	// when
	err := service.WithPostMethod().ApplyAll(environment.DefaultEnvTypes)

	// then
	require.NoError(s.T(), err)
	// the expected number is number of all objects + 11 get calls to verify that objects are created + 1 to removed admin role binding
	assert.Equal(s.T(), ExpectedNumberOfCallsWhenPost(s.T(), config, clusterMapping, userCreator.NewUserInfo("developer")), calls)
	namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
	require.NoError(s.T(), err)
	assert.Len(s.T(), namespaces, 5)
}
