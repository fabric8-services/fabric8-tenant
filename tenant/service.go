package tenant

import (
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

type Service interface {
	Exists(tenantID uuid.UUID) bool
	GetTenant(tenantID uuid.UUID) (*Tenant, error)
	GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error)
	CreateOrUpdateTenant(tenant *Tenant) error
	CreateOrUpdateNamespace(namespace *Namespace) error
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
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s DBService) CreateOrUpdateTenant(tenant *Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	return s.db.Save(tenant).Error
}

func (s DBService) CreateOrUpdateNamespace(namespace *Namespace) error {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	return s.db.Save(namespace).Error
}

func (s DBService) GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error) {
	var t []*Namespace
	err := s.db.Table(Namespace{}.TableName()).Where("tenant_id = ?", tenantID).Find(&t).Error
	if err != nil {
		return nil, err
	}
	return t, nil
}

type NilService struct {
}

func (s NilService) Exists(tenantID uuid.UUID) bool {
	return false
}

func (s NilService) GetTenant(tenantID uuid.UUID) (*Tenant, error) {
	return nil, nil
}

func (s NilService) GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error) {
	return nil, nil
}

func (s NilService) UpdateTenant(tenant *Tenant) error {
	return nil
}

func (s NilService) UpdateNamespace(namespace *Namespace) error {
	return nil
}
