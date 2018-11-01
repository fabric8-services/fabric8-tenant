package gormsupport

import (
	"errors"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	errorscommon "github.com/fabric8-services/fabric8-common/errors"
	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
)

type DBStub struct {
	Tenants    []*tenant.Tenant
	Namespaces []*tenant.Namespace
}

func NewDBServiceStub(tenantRecord *tenant.Tenant, namespaces []*tenant.Namespace) (tenant.Service, *DBStub) {
	dbStub := &DBStub{Tenants: []*tenant.Tenant{tenantRecord}, Namespaces: namespaces}
	return &DBServiceStub{db: dbStub}, dbStub
}

func NewEmptyDBServiceStub() (tenant.Service, *DBStub) {
	dbStub := &DBStub{}
	return &DBServiceStub{db: dbStub}, dbStub
}

type DBServiceStub struct {
	db *DBStub
}

func (s DBServiceStub) Exists(tenantID uuid.UUID) bool {
	for _, tenant := range s.db.Tenants {
		if tenant.ID == tenantID {
			return true
		}
	}
	return false
}

func (s DBServiceStub) GetTenant(tenantID uuid.UUID) (*tenant.Tenant, error) {
	for _, tenant := range s.db.Tenants {
		if tenant.ID == tenantID {
			return tenant, nil
		}
	}
	return nil, errorscommon.NewNotFoundError("tenant", tenantID.String())

}

// todo finish
func (s DBServiceStub) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*tenant.Tenant, error) {
	for _, ns := range s.db.Namespaces {
		if ns.MasterURL == masterURL && ns.Name == namespace {
			for _, tenant := range s.db.Tenants {
				if tenant.ID == ns.TenantID {
					return tenant, nil
				}
			}
		}
	}
	return nil, errorscommon.NewNotFoundError("tenant", "")
}

func (s DBServiceStub) SaveTenant(tenant *tenant.Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}

	for i, tenant := range s.db.Tenants {
		if tenant.ID == tenant.ID {
			s.db.Tenants[i] = tenant
			return nil
		}
	}
	return s.CreateTenant(tenant)
}

func (s DBServiceStub) CreateTenant(tenant *tenant.Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	if s.Exists(tenant.ID) {
		return errors.New("pq: duplicate key value violates unique constraint \"tenants_pkey\"")
	}
	s.db.Tenants = append(s.db.Tenants, tenant)
	return nil
}

func (s DBServiceStub) SaveNamespace(namespace *tenant.Namespace) error {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	for i, ns := range s.db.Namespaces {
		if ns.ID == namespace.ID {
			s.db.Namespaces[i] = namespace
			return nil
		}
	}
	s.db.Namespaces = append(s.db.Namespaces, namespace)
	return nil
}

func (s DBServiceStub) GetNamespaces(tenantID uuid.UUID) ([]*tenant.Namespace, error) {
	var nss []*tenant.Namespace
	for _, ns := range s.db.Namespaces {
		if ns.TenantID == tenantID {
			nss = append(nss, ns)
		}
	}
	return nss, nil
}

func (s DBServiceStub) DeleteNamespace(tenantID uuid.UUID, nsType environment.Type) error {
	for i, ns := range s.db.Namespaces {
		if ns.TenantID == tenantID && ns.Type == nsType {
			s.db.Namespaces = append(s.db.Namespaces[:i], s.db.Namespaces[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (s DBServiceStub) deleteNamespaces(tenantID uuid.UUID) error {
	found := false
	for i, ns := range s.db.Namespaces {
		if ns.TenantID == tenantID {
			s.db.Namespaces = append(s.db.Namespaces[:i], s.db.Namespaces[i+1:]...)
			found = true
		}
	}
	if found {
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (s DBServiceStub) DeleteTenant(tenantID uuid.UUID) error {
	for i, tenant := range s.db.Tenants {
		if tenant.ID == tenantID {
			s.db.Tenants = append(s.db.Tenants[:i], s.db.Tenants[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (s DBServiceStub) NewTenantRepository(tenantID uuid.UUID) tenant.Repository {
	return &tenantRepositoryStub{service: s, tenantID: tenantID}
}

type tenantRepositoryStub struct {
	service  DBServiceStub
	tenantID uuid.UUID
}

func (n tenantRepositoryStub) NewNamespace(nsType environment.Type, nsName string, state tenant.NamespaceState) *tenant.Namespace {
	return &tenant.Namespace{
		TenantID: n.tenantID,
		Name:     nsName,
		Type:     nsType,
		State:    state,
	}
}

func (n tenantRepositoryStub) GetNamespaces() ([]*tenant.Namespace, error) {
	return n.service.GetNamespaces(n.tenantID)
}

func (n tenantRepositoryStub) SaveNamespace(namespace *tenant.Namespace) error {
	namespace.TenantID = n.tenantID
	return n.service.SaveNamespace(namespace)
}

func (n tenantRepositoryStub) DeleteNamespace(namespace *tenant.Namespace) error {
	for i, ns := range n.service.db.Namespaces {
		if ns.ID == namespace.ID {
			n.service.db.Namespaces = append(n.service.db.Namespaces[:i], n.service.db.Namespaces[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (n tenantRepositoryStub) DeleteTenant() error {
	return n.service.DeleteTenant(n.tenantID)
}
