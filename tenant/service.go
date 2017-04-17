package tenant

import (
	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

type Service struct {
	db *gorm.DB
}

func (s Service) Exists(tenantID uuid.UUID) bool {
	var t Tenant
	err := s.db.Table(t.TableName()).Where("id = ?", tenantID).Find(&t).Error
	if err != nil {
		return false
	}
	return true
}

func (s Service) UpdateTenant(tenant *Tenant) error {
	return s.db.Save(tenant).Error
}

func (s Service) UpdateNamespace(namespace *Namespace) error {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	return s.db.Save(namespace).Error
}
