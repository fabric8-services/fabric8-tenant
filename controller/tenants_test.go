package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/fabric8-services/fabric8-wit/resource"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TenantControllerTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantController(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TenantControllerTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

var clusterResolver = func(ctx context.Context, target string) (token.Cluster, error) {
	return token.Cluster{
		APIURL:     "https://api.example.com",
		ConsoleURL: "https://console.example.com/console",
		MetricsURL: "https://metrics.example.com",
		AppDNS:     "apps.example.com",
		User:       "service-account",
		Token:      "XX",
	}, nil
}

func (s *TenantControllerTestSuite) TestShowTenants() {

	s.T().Run("OK", func(t *testing.T) {
		// given
		tenantID := uuid.NewV4()
		svc := goa.New("Tenants-service")
		ctrl := NewTenantsController(svc, mockTenantService{ID: tenantID}, clusterResolver)
		// when
		_, tenant := test.ShowTenantsOK(t, createValidSAContext(), svc, ctrl, tenantID)
		// then
		assert.Equal(t, tenantID, *tenant.Data.ID)
		assert.Equal(t, 1, len(tenant.Data.Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		// given
		tenantID := uuid.NewV4()
		svc := goa.New("Tenants-service")
		ctrl := NewTenantsController(svc, mockTenantService{ID: tenantID}, clusterResolver)

		t.Run("Unauhorized - no token", func(t *testing.T) {
			// when/then
			test.ShowTenantsUnauthorized(t, context.Background(), svc, ctrl, tenantID)
		})

		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			// when/then
			test.ShowTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, tenantID)
		})

		t.Run("Not found", func(t *testing.T) {
			// when/then
			test.ShowTenantsNotFound(t, createValidSAContext(), svc, ctrl, uuid.NewV4())
		})
	})
}

func (s *TenantControllerTestSuite) TestSearchTenants() {
	// given
	svc := goa.New("Tenants-service")

	s.T().Run("OK", func(t *testing.T) {
		// given
		ctrl := NewTenantsController(svc, tenant.NewDBService(s.DB), clusterResolver)
		fxt := testfixture.NewTestFixture(t, s.DB, testfixture.Tenants(1), testfixture.Namespaces(1))
		// when
		_, tenant := test.SearchTenantsOK(t, createValidSAContext(), svc, ctrl, fxt.Namespaces[0].MasterURL, fxt.Namespaces[0].Name)
		// then
		require.Len(t, tenant.Data, 1)
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data[0].ID)
		assert.Equal(t, 1, len(tenant.Data[0].Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {
		ctrl := NewTenantsController(svc, mockTenantService{}, clusterResolver)

		t.Run("Unauhorized - no token", func(t *testing.T) {
			test.SearchTenantsUnauthorized(t, context.Background(), svc, ctrl, "foo", "bar")
		})
		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			test.SearchTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, "foo", "bar")
		})
		t.Run("Not found", func(t *testing.T) {
			test.SearchTenantsNotFound(t, createValidSAContext(), svc, ctrl, "foo", "bar")
		})
		t.Run("Internal Server Error", func(t *testing.T) {
			test.SearchTenantsInternalServerError(t, createValidSAContext(), svc, ctrl, "", "")
		})
	})
}

func createValidSAContext() context.Context {
	claims := jwt.MapClaims{}
	claims["service_accountname"] = "fabric8-jenkins-idler"
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

func createInvalidSAContext() context.Context {
	claims := jwt.MapClaims{}
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

type mockTenantService struct {
	ID uuid.UUID
}

func (s mockTenantService) Exists(tenantID uuid.UUID) bool {
	return s.ID == tenantID
}

func (s mockTenantService) GetTenant(tenantID uuid.UUID) (*tenant.Tenant, error) {
	if s.ID != tenantID {
		return nil, errors.NewNotFoundError("tenant", tenantID.String())
	}
	return &tenant.Tenant{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ID:        tenantID,
		Email:     "test@test.org",
	}, nil
}

func (s mockTenantService) GetNamespaces(tenantID uuid.UUID) ([]*tenant.Namespace, error) {
	if s.ID != tenantID {
		return nil, errors.NewNotFoundError("tenant", tenantID.String())
	}
	return []*tenant.Namespace{
		{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			ID:        uuid.NewV4(),
			TenantID:  tenantID,
			Name:      "test-che",
			Type:      "che",
			State:     "created",
			MasterURL: "http://test.org",
			Version:   "1.0",
		},
	}, nil
}

func (s mockTenantService) SaveTenant(tenant *tenant.Tenant) error {
	return nil
}

func (s mockTenantService) SaveNamespace(namespace *tenant.Namespace) error {
	return nil
}

func (s mockTenantService) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*tenant.Tenant, error) {
	// produce InternalServerError
	if masterURL == "" || namespace == "" {
		return nil, fmt.Errorf("mock error")
	}
	return nil, errors.NewNotFoundError("tenant", "")

}
