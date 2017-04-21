package tenant

import (
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

type Service interface {
	Exists(tenantID uuid.UUID) bool
	UpdateTenant(tenant *Tenant) error
	UpdateNamespace(namespace *Namespace) error
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

func (s DBService) UpdateTenant(tenant *Tenant) error {
	return s.db.Save(tenant).Error
}

func (s DBService) UpdateNamespace(namespace *Namespace) error {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	return s.db.Save(namespace).Error
}

type NilService struct {
}

func (s NilService) Exists(tenantID uuid.UUID) bool {
	return false
}

func (s NilService) UpdateTenant(tenant *Tenant) error {
	return nil
}

func (s NilService) UpdateNamespace(namespace *Namespace) error {
	return nil
}
