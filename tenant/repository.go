package tenant

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-tenant/dbsupport"
	"github.com/fabric8-services/fabric8-tenant/environment"
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
	CreateNamespace(namespace *Namespace) (*Namespace, error)
	DeleteTenant(tenantID uuid.UUID) error
	NewTenantRepository(tenantID uuid.UUID) Repository
	NamespaceExists(nsName string) (bool, error)
	ExistsWithNsBaseName(nsBaseName string) (bool, error)
	GetTenantsToUpdate(typeWithVersion map[environment.Type]string, count int, commit string, masterURL string) ([]*Tenant, error)
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

func (s DBService) CreateNamespace(namespace *Namespace) (*Namespace, error) {
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	length := len(namespace.Name)
	lockid := length + int([]rune(namespace.Name)[length-1])
	created := false
	err := dbsupport.Transaction(s.db, dbsupport.Lock(lockid, 10, func(tx *gorm.DB) error {
		var duplicate []*Namespace
		err := tx.Table(namespaceTableName).
			Where("name = ? AND master_url = ? AND tenant_id = ?", namespace.Name, namespace.MasterURL, namespace.TenantID).
			Find(&duplicate).Error
		if err != nil {
			return err
		}
		if len(duplicate) > 0 {
			return nil
		}
		created = true
		return tx.Create(namespace).Error
	}))
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, nil
	}
	return namespace, nil
}

func (s DBService) GetNamespaces(tenantID uuid.UUID) ([]*Namespace, error) {
	var t []*Namespace
	err := s.db.Table(Namespace{}.TableName()).Where("tenant_id = ?", tenantID).Find(&t).Error
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

func (s DBService) GetTenantsToUpdate(typeWithVersion map[environment.Type]string, count int, commit string, masterURL string) ([]*Tenant, error) {
	var tenants []*Tenant
	nsSubQuery := s.db.Table(Namespace{}.TableName()).Select("tenant_id")
	nsSubQuery = nsSubQuery.Where("state != 'failed' OR (state = 'failed' AND updated_by != ?)", commit)
	if masterURL != "" {
		nsSubQuery = nsSubQuery.Where("master_url = ?", masterURL)
	}

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

func (s DBService) DeleteNamespaces(tenantID uuid.UUID) error {
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
	GetTenant() (*Tenant, error)
	NewNamespace(envType environment.Type, nsName, masterURL string, state NamespaceState) *Namespace
	GetNamespaces() ([]*Namespace, error)
	SaveNamespace(namespace *Namespace) error
	CreateNamespace(namespace *Namespace) (*Namespace, error)
	DeleteNamespace(namespace *Namespace) error
	DeleteNamespaces() error
	DeleteTenant() error
	Service() Service
}

type DBTenantRepository struct {
	service  DBService
	tenantID uuid.UUID
}

func (n DBTenantRepository) NewNamespace(envType environment.Type, nsName, masterURL string, state NamespaceState) *Namespace {
	return &Namespace{
		TenantID:  n.tenantID,
		Name:      nsName,
		Type:      envType,
		State:     state,
		MasterURL: masterURL,
	}
}

func (n DBTenantRepository) GetTenant() (*Tenant, error) {
	return n.service.GetTenant(n.tenantID)
}

func (n DBTenantRepository) GetNamespaces() ([]*Namespace, error) {
	return n.service.GetNamespaces(n.tenantID)
}

func (n DBTenantRepository) SaveNamespace(namespace *Namespace) error {
	namespace.TenantID = n.tenantID
	return n.service.SaveNamespace(namespace)
}

func (n DBTenantRepository) CreateNamespace(namespace *Namespace) (*Namespace, error) {
	namespace.TenantID = n.tenantID
	return n.service.CreateNamespace(namespace)
}

func (n DBTenantRepository) DeleteNamespace(namespace *Namespace) error {
	return n.service.db.Unscoped().Delete(namespace).Error
}

func (n DBTenantRepository) DeleteNamespaces() error {
	return n.service.DeleteNamespaces(n.tenantID)
}

func (n DBTenantRepository) DeleteTenant() error {
	return n.service.DeleteTenant(n.tenantID)
}

func (n DBTenantRepository) Service() Service {
	return n.service
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
		if nsType != environment.TypeUser {
			nsName += "-" + nsType.String()
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
