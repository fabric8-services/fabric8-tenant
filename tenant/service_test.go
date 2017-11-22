package tenant_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	"github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-wit/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TenantServiceTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantService(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TenantServiceTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantServiceTestSuite) TestCreateTenant() {

	s.T().Run("ok", func(t *testing.T) {
		// given
		svc := tenant.NewDBService(s.DB)
		tenant := &tenant.Tenant{
			ID:      uuid.NewV4(),
			Email:   "joe@foo.com",
			Profile: "free",
		}
		// when
		err := svc.SaveTenant(tenant)
		// then
		assert.NoError(t, err)
	})

	s.T().Run("ko - missing id", func(t *testing.T) {
		// given
		svc := tenant.NewDBService(s.DB)
		tenant := &tenant.Tenant{
			Email:   "joe@foo.com",
			Profile: "unknown",
		}
		// when
		err := svc.SaveTenant(tenant)
		// then
		assert.Error(t, err)
	})

	s.T().Run("ko - invalid profile", func(t *testing.T) {
		// given
		svc := tenant.NewDBService(s.DB)
		tenant := &tenant.Tenant{
			ID:      uuid.NewV4(),
			Email:   "joe@foo.com",
			Profile: "unknown",
		}
		// when
		err := svc.SaveTenant(tenant)
		// then
		assert.Error(t, err)
	})
}

func (s *TenantServiceTestSuite) TestUpdateTenant() {
	s.T().Run("ok", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(s.T(), s.DB, testfixture.Tenants(1))
		svc := tenant.NewDBService(s.DB)
		tenant := fxt.Tenants[0]
		// when
		tenant.Email = "joe@bar.com"
		err := svc.SaveTenant(tenant)
		// then
		assert.NoError(t, err)
	})

	s.T().Run("ko - invalid profile", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(s.T(), s.DB, testfixture.Tenants(1))
		svc := tenant.NewDBService(s.DB)
		tenant := fxt.Tenants[0]
		// when
		tenant.Profile = "unknown"
		err := svc.SaveTenant(tenant)
		// then
		assert.Error(t, err)
	})
}

func (s *TenantServiceTestSuite) TestLookupTenantByNamespace() {
	s.T().Run("ok", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(s.T(), s.DB, testfixture.Tenants(1), testfixture.Namespaces(1))
		svc := tenant.NewDBService(s.DB)
		ns := fxt.Namespaces[0]
		// when
		result, err := svc.LookupTenantByClusterAndNamespace(ns.MasterURL, ns.Name)
		// then
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, fxt.Tenants[0].ID, result.ID)
	})

	s.T().Run("not found", func(t *testing.T) {
		// given
		svc := tenant.NewDBService(s.DB)
		// when
		result, err := svc.LookupTenantByClusterAndNamespace("foo", "bar")
		// then
		require.Error(t, err)
		require.IsType(t, errors.NotFoundError{}, err)
		require.Nil(t, result)
	})

}
