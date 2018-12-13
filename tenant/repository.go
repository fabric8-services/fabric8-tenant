package tenant

import (
	"fmt"

	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-wit/errors"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"strings"
)

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
	DeleteAll(tenantID uuid.UUID) error
	NamespaceExists(nsName string) (bool, error)
	ExistsWithNsBaseName(nsBaseName string) (bool, error)
	GetTenantsToUpdate(typeWithVersion map[string]string, count int, commit string) ([]*Tenant, error)
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

func (s DBService) ExistsWithNsBaseName(nsBaseName string) (bool, error) {
	var t Tenant
	err := s.db.Table(t.TableName()).Where("ns_base_name = ?", nsBaseName).Find(&t).Error
	if err != nil {
		if gorm.ErrRecordNotFound == err {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

func (s DBService) NamespaceExists(nsName string) (bool, error) {
	var ns Namespace
	err := s.db.Table(Namespace{}.TableName()).Where("name = ?", nsName).Find(&ns).Error
	if err != nil {
		if gorm.ErrRecordNotFound == err {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s DBService) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error) {
	// select t.id from tenant t, namespaces n where t.id = n.tenant_id and n.master_url = ? and n.name = ?
	query := fmt.Sprintf("select t.* from %[1]s t, %[2]s n where t.id = n.tenant_id and n.master_url = ? and n.name = ?", Tenant{}.TableName(), Namespace{}.TableName())
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
	err := s.db.Table(Namespace{}.TableName()).Where("tenant_id = ?", tenantID).Find(&t).Error
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s DBService) GetTenantsToUpdate(typeWithVersion map[string]string, count int, commit string) ([]*Tenant, error) {
	var tenants []*Tenant
	nsSubQuery := s.db.Table(Namespace{}.TableName()).Select("tenant_id").
		Where("state != 'failed' OR (state = 'failed' AND updated_by != ?)", commit)

	var conditions []string
	var params []interface{}
	for envType, version := range typeWithVersion {
		conditions = append(conditions, "(namespaces.type = ? AND namespaces.version != ?)")
		params = append(params, envType, version)
	}
	nsSubQuery = nsSubQuery.Where(strings.Join(conditions, " OR "), params...).Group("tenant_id")

	err := s.db.Table(Tenant{}.TableName()).
		Joins("INNER JOIN ? n ON tenants.id = n.tenant_id", nsSubQuery.SubQuery()).Limit(count).
		Scan(&tenants).Error

	return tenants, err
}

func (s DBService) DeleteAll(tenantID uuid.UUID) error {
	err := s.deleteNamespaces(tenantID)
	err = s.deleteTenant(tenantID)
	return err
}

func (s DBService) deleteNamespaces(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return nil
	}
	return s.db.Unscoped().Delete(&Namespace{}, "tenant_id = ?", tenantID).Error
}

func (s DBService) deleteTenant(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return nil
	}
	return s.db.Unscoped().Delete(&Tenant{ID: tenantID}).Error
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

func (s NilService) SaveTenant(tenant *Tenant) error {
	return nil
}

func (s NilService) SaveNamespace(namespace *Namespace) error {
	return nil
}

func (s NilService) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error) {
	return nil, nil
}

func (s NilService) DeleteAll(tenantID uuid.UUID) error {
	return nil
}

func ConstructNsBaseName(repo Service, username string) (string, error) {
	return constructNsBaseName(repo, username, 1)
}

func constructNsBaseName(repo Service, username string, number int) (string, error) {
	nsBaseName := username
	if number > 1 {
		nsBaseName += fmt.Sprintf("%d", number)
	}
	exists, err := repo.ExistsWithNsBaseName(nsBaseName)
	if err != nil {
		return "", errs.Wrapf(err, "getting already existing tenants with the NsBaseName %s failed: ", nsBaseName)
	}
	if exists {
		number++
		return constructNsBaseName(repo, username, number)
	}
	for _, nsType := range environment.DefaultEnvTypes {
		nsName := nsBaseName
		if nsType != "user" {
			nsName += "-" + nsType
		}
		exists, err := repo.NamespaceExists(nsName)
		if err != nil {
			return "", errs.Wrapf(err, "getting already existing namespaces with the name %s failed: ", nsName)
		}
		if exists {
			number++
			return constructNsBaseName(repo, username, number)
		}
	}
	return nsBaseName, nil
}
