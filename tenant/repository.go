package tenant

import (
	"fmt"

	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

type NamespaceState string

const (
	Provisioning NamespaceState = "provisioning"
	Updating     NamespaceState = "updating"
	Ready        NamespaceState = "ready"
	Failed       NamespaceState = "failed"
)

func (s NamespaceState) String() string {
	return string(s)
}

type Service interface {
	Exists(tenantID uuid.UUID) bool
	GetTenant(tenantID uuid.UUID) (*Tenant, error)
	LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error)
	GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error)
	// CreateTenant will return err on duplicate insert
	CreateTenant(tenant *Tenant) error
	// SaveTenant will update on dupliate 'insert'
	SaveTenant(tenant *Tenant) error
	SaveNamespace(namespace *Namespace) error
	DeleteTenant(tenantID uuid.UUID) error
	NewTenantRepository(tenantID uuid.UUID) Repository
}

func NewDBService(db *gorm.DB) Service {
	return &DBService{db: db}
}

type DBService struct {
	db *gorm.DB
}

func (s DBService) Exists(tenantID uuid.UUID) bool {
	var t Tenant
	err := s.db.Table(t.TableName()).Where("id = ?", tenantID).Find(&t).Error
	if err != nil {
		return false
	}
	return true
}

func (s DBService) GetTenant(tenantID uuid.UUID) (*Tenant, error) {
	var t Tenant
	err := s.db.Table(t.TableName()).Where("id = ?", tenantID).Find(&t).Error
	if err == gorm.ErrRecordNotFound {
		// no match
		return nil, errors.NewNotFoundError("tenant", tenantID.String())
	} else if err != nil {
		return nil, errs.Wrapf(err, "unable to lookup tenant by id")
	}
	return &t, nil
}

func (s DBService) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error) {
	// select t.id from tenant t, namespaces n where t.id = n.tenant_id and n.master_url = ? and n.name = ?
	query := fmt.Sprintf("select t.* from %[1]s t, %[2]s n where t.id = n.tenant_id and n.master_url = ? and n.name = ?", tenantTableName, namespaceTableName)
	var result Tenant
	err := s.db.Raw(query, masterURL, namespace).Scan(&result).Error
	if err == gorm.ErrRecordNotFound {
		// no match
		return nil, errors.NewNotFoundError("tenant", "")
	} else if err != nil {
		return nil, errs.Wrapf(err, "unable to lookup tenant by namespace")
	}
	return &result, nil
}

func (s DBService) SaveTenant(tenant *Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	return s.db.Save(tenant).Error
}

func (s DBService) CreateTenant(tenant *Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	return s.db.Create(tenant).Error
}

func (s DBService) SaveNamespace(namespace *Namespace) error {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	return s.db.Save(namespace).Error
}

func (s DBService) GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error) {
	var t []*Namespace
	err := s.db.Table(namespaceTableName).Where("tenant_id = ?", tenantID).Find(&t).Error
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s DBService) DeleteNamespace(tenantID uuid.UUID, envType environment.Type) error {
	if tenantID == uuid.Nil {
		return nil
	}
	return s.db.Unscoped().Delete(&Namespace{}, "tenant_id = ? and type = ?", tenantID, envType).Error
}

func (s DBService) deleteNamespaces(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return nil
	}
	return s.db.Unscoped().Delete(&Namespace{}, "tenant_id = ?", tenantID).Error
}

func (s DBService) DeleteTenant(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return nil
	}
	return s.db.Unscoped().Delete(&Tenant{ID: tenantID}).Error
}

func (s DBService) NewTenantRepository(tenantID uuid.UUID) Repository {
	return DBTenantRepository{service: s, tenantID: tenantID}
}

type Repository interface {
	NewNamespace(envType environment.Type, nsName string, state NamespaceState) *Namespace
	GetNamespaces() ([]*Namespace, error)
	SaveNamespace(namespace *Namespace) error
	DeleteNamespace(namespace *Namespace) error
	DeleteTenant() error
}

type DBTenantRepository struct {
	service  DBService
	tenantID uuid.UUID
}

func (n DBTenantRepository) NewNamespace(envType environment.Type, nsName string, state NamespaceState) *Namespace {
	return &Namespace{
		TenantID: n.tenantID,
		Name:     nsName,
		Type:     envType,
		State:    state,
	}
}

func (n DBTenantRepository) GetNamespaces() ([]*Namespace, error) {
	return n.service.GetNamespaces(n.tenantID)
}

func (n DBTenantRepository) SaveNamespace(namespace *Namespace) error {
	namespace.TenantID = n.tenantID
	return n.service.SaveNamespace(namespace)
}

func (n DBTenantRepository) DeleteNamespace(namespace *Namespace) error {
	return n.service.db.Unscoped().Delete(namespace).Error
}

func (n DBTenantRepository) DeleteTenant() error {
	return n.service.DeleteTenant(n.tenantID)
}
