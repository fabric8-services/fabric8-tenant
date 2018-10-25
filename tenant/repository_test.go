package tenant_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
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
	if ready, reason := resource.IsReady(resource.Database); !ready {
		t.Skip(reason)
	}
	suite.Run(t, &TenantServiceTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantServiceTestSuite) TestCreateTenant() {

	s.T().Run("ko - duplicate", func(t *testing.T) {
		// given
		svc := tenant.NewDBService(s.DB)
		tenant := &tenant.Tenant{
			ID:      uuid.NewV4(),
			Email:   "joe@foo.com",
			Profile: "free",
		}
		// when
		err := svc.CreateTenant(tenant)
		assert.NoError(t, err)
		err = svc.CreateTenant(tenant)
		// then
		assert.Error(t, err)
	})
}

func (s *TenantServiceTestSuite) TestSaveTenant() {

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
			Profile: "free",
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
		fxt := tf.NewTestFixtureWithDB(t, s.DB, tf.Tenants(1))
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
		fxt := tf.NewTestFixtureWithDB(t, s.DB, tf.Tenants(1))
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
		fxt := tf.NewTestFixtureWithDB(t, s.DB, tf.Tenants(1), tf.Namespaces(1))
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

func (s *TenantServiceTestSuite) TestDelete() {
	s.T().Run("all info", func(t *testing.T) {
		// given
		fxt := tf.NewTestFixtureWithDB(t, s.DB, tf.Tenants(2), tf.Namespaces(10, func(fxt *tf.TestFixture, idx int) error {
			if idx < 5 {
				fxt.Namespaces[idx].TenantID = fxt.Tenants[0].ID
			} else {
				fxt.Namespaces[idx].TenantID = fxt.Tenants[1].ID
			}
			return nil
		}))
		svc := tenant.NewDBService(s.DB)
		tenant1 := fxt.Tenants[0]
		tenant2 := fxt.Tenants[1]
		// when
		namespaces, err := svc.GetNamespaces(tenant1.ID)
		require.NoError(t, err)
		for _, ns := range namespaces {
			svc.NewTenantRepository(tenant1.ID).DeleteNamespace(ns)
		}
		svc.DeleteTenant(tenant1.ID)
		// then
		// should be deleted
		ten1, _ := svc.GetTenant(tenant1.ID)
		require.Nil(t, ten1)
		ns1, _ := svc.GetNamespaces(tenant1.ID)
		require.Len(t, ns1, 0)

		// should not be deleted
		ten2, err := svc.GetTenant(tenant2.ID)
		require.NotNil(t, ten2)
		require.NoError(t, err)
		ns2, err := svc.GetNamespaces(tenant2.ID)
		require.NoError(t, err)
		require.Len(t, ns2, 5)
	})
}
