package controller

import (
	"context"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
)

func TestTenants(t *testing.T) {
	tenantID := uuid.NewV4()
	svc := goa.New("Tenants-service")
	ctrl := NewTenantsController(svc, ctrlTestService{ID: tenantID})
	t.Run("OK", func(t *testing.T) {
		_, tenant := test.ShowTenantsOK(t, createValidSAContext(), svc, ctrl, tenantID)
		assert.Equal(t, tenantID, *tenant.Data.ID)
		assert.Equal(t, 1, len(tenant.Data.Attributes.Namespaces))
	})
	t.Run("Unauhorized - no token", func(t *testing.T) {
		test.ShowTenantsUnauthorized(t, context.Background(), svc, ctrl, tenantID)
	})
	t.Run("Unauhorized - no SA token", func(t *testing.T) {
		test.ShowTenantsUnauthorized(t, createInValidSAContext(), svc, ctrl, tenantID)
	})
	t.Run("Not found", func(t *testing.T) {
		test.ShowTenantsNotFound(t, createValidSAContext(), svc, ctrl, uuid.NewV4())
	})
}

func createValidSAContext() context.Context {
	claims := jwt.MapClaims{}
	claims["service_accountname"] = "fabric8-jenkins-idler"
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

func createInValidSAContext() context.Context {
	claims := jwt.MapClaims{}
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

type ctrlTestService struct {
	ID uuid.UUID
}

func (s ctrlTestService) Exists(tenantID uuid.UUID) bool {
	return s.ID == tenantID
}

func (s ctrlTestService) GetTenant(tenantID uuid.UUID) (*tenant.Tenant, error) {
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

func (s ctrlTestService) GetNamespaces(tenantID uuid.UUID) ([]*tenant.Namespace, error) {
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

func (s ctrlTestService) UpdateTenant(tenant *tenant.Tenant) error {
	return nil
}

func (s ctrlTestService) UpdateNamespace(namespace *tenant.Namespace) error {
	return nil
}
