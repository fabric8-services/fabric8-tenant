package openshift_test

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"testing"
)

type InitTenantTestSuite struct {
	gormsupport.DBTestSuite
}

func TestInitTenant(t *testing.T) {
	suite.Run(t, &InitTenantTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

var emptyCallback = func(statusCode int, method string, request, response map[interface{}]interface{}, versionMapping map[environment.Type]string) (string, map[interface{}]interface{}) {
	return "", nil
}

func (s *InitTenantTestSuite) TestNumberOfCallsToCluster() {
	// given
	data, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	calls := 0
	gock.New("http://my.cluster").
		SetMatcher(test.SpyOnCalls(&calls)).
		Times(78).
		Persist().
		Reply(200).
		BodyString("{}")

	user := &client.UserDataAttributes{}
	config := openshift.NewConfigForUser(data, user, "clusterUser", "clusterToken", "http://my.cluster")
	config.HTTPTransport = http.DefaultTransport
	objectsInTemplates := tmplObjects(s.T(), data)

	tnnt := &tenant.Tenant{ID: uuid.NewV4(), OSUsername: "developer", NsBaseName: "developer"}

	// when
	_, err := openshift.RawInitTenant(context.Background(), config, tnnt, "12345", tenant.NewDBService(s.DB), true)

	// then
	require.NoError(s.T(), err)
	// the number of calls should be equal to the number of parsed objects plus one call that removes admin role from user's namespace
	assert.Equal(s.T(), len(objectsInTemplates)+1, calls)
}

func (s *InitTenantTestSuite) TestCreateNewNamespacesWithBaseNameEnding2WhenFails() {
	// given
	data, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenantsNamed("johndoe"), true, tf.AddDefaultNamespaces().State(tenant.Provisioning))
	johndoeCalls := 0
	projectRequestCalls := 0
	deleteCalls := 0
	johndoe2Calls := 2

	gock.New("http://api.cluster1").
		Post("/api/v1/namespaces/johndoe-jenkins/persistentvolumeclaims").
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
	config := openshift.NewConfigForUser(data, user, "clusterUser", "clusterToken", "http://api.cluster1/")
	config.HTTPTransport = http.DefaultTransport
	objsNumber := len(tmplObjects(s.T(), data))
	repo := tenant.NewDBService(s.DB)

	// when
	_, err := openshift.RawInitTenant(context.Background(), config, fxt.Tenants[0], "12345", repo, true)

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

func (s *InitTenantTestSuite) TestCreateNewNamespacesWithBaseNameEnding3WhenFailsAnd2Exists() {
	// given
	data, reset := test.LoadTestConfig(s.T())
	defer func() {
		gock.OffAll()
		reset()
	}()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenantsNamed("johndoe", "johndoe2"), true, tf.AddDefaultNamespaces().State(tenant.Provisioning))

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
