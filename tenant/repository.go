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
	CreateTenant(tenant *Tenant) error
	SaveTenant(tenant *Tenant) error
	LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error)
	NewTenantRepository(tenantID uuid.UUID) Repository
	NamespaceExists(nsName string) (bool, error)
	ExistsWithNsBaseName(nsBaseName string) (bool, error)
	GetTenantsToUpdate(typeWithVersion map[environment.Type]string, count int, commit string, masterURL string) ([]*Tenant, error)
	GetNumberOfOutdatedTenants(typeWithVersion map[environment.Type]string, commit string, masterURL string) (int, error)
}

func NewDBService(db *gorm.DB) Service {
	return &DBService{db: db}
}

type DBService struct {
	db *gorm.DB
}

func (s *DBService) SaveTenant(tenant *Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	return s.db.Save(tenant).Error
}

func (s *DBService) CreateTenant(tenant *Tenant) error {
	if tenant.Profile == "" {
		tenant.Profile = "free"
	}
	return s.db.Create(tenant).Error
}

func (s *DBService) ExistsWithNsBaseName(nsBaseName string) (bool, error) {
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

func (s *DBService) NamespaceExists(nsName string) (bool, error) {
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

func (s *DBService) LookupTenantByClusterAndNamespace(masterURL, namespace string) (*Tenant, error) {
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

func (s *DBService) GetTenantsToUpdate(typeWithVersion map[environment.Type]string, count int, commit string, masterURL string) ([]*Tenant, error) {
	var tenants []*Tenant
	err := s.newGetOutdatedTenantsQuery(typeWithVersion, commit, masterURL).Limit(count).Scan(&tenants).Error

	return tenants, err
}

func (s *DBService) GetNumberOfOutdatedTenants(typeWithVersion map[environment.Type]string, commit string, masterURL string) (int, error) {
	var count int
	err := s.newGetOutdatedTenantsQuery(typeWithVersion, commit, masterURL).Count(&count).Error

	return count, err
}

func (s *DBService) newGetOutdatedTenantsQuery(typeWithVersion map[environment.Type]string, commit string, masterURL string) *gorm.DB {
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
	return s.db.Table(Tenant{}.TableName()).
		Joins("INNER JOIN ? n ON tenants.id = n.tenant_id", nsSubQuery.SubQuery())
}

func (s *DBService) NewTenantRepository(tenantID uuid.UUID) Repository {
	return &DBTenantRepository{DBService: s, tenantID: tenantID}
}

func NewTenantRepository(db *gorm.DB, tenantID uuid.UUID) Repository {
	return &DBTenantRepository{DBService: &DBService{db: db}, tenantID: tenantID}
}

type Repository interface {
	Service
	Exists() bool
	GetTenant() (*Tenant, error)
	NewNamespace(envType environment.Type, nsName, masterURL string, state NamespaceState) *Namespace
	GetNamespaces() ([]*Namespace, error)
	SaveNamespace(namespace *Namespace) error
	CreateNamespace(namespace *Namespace) (*Namespace, error)
	DeleteNamespace(namespace *Namespace) error
	DeleteNamespaces() error
	DeleteTenant() error
}

type DBTenantRepository struct {
	*DBService
	tenantID uuid.UUID
}

func (r *DBTenantRepository) NewNamespace(envType environment.Type, nsName, masterURL string, state NamespaceState) *Namespace {
	return &Namespace{
		TenantID:  r.tenantID,
		Name:      nsName,
		Type:      envType,
		State:     state,
		MasterURL: masterURL,
	}
}

func (r *DBTenantRepository) Exists() bool {
	var t Tenant
	err := r.db.Table(t.TableName()).Where("id = ?", r.tenantID).Find(&t).Error
	if err != nil {
		return false
	}
	return true
}

func (r *DBTenantRepository) GetTenant() (*Tenant, error) {
	var t Tenant
	err := r.db.Table(t.TableName()).Where("id = ?", r.tenantID).Find(&t).Error
	if err == gorm.ErrRecordNotFound {
		// no match
		return nil, errors.NewNotFoundError("tenant", r.tenantID.String())
	} else if err != nil {
		return nil, errs.Wrapf(err, "unable to lookup tenant by id")
	}
	return &t, nil
}

func (r *DBTenantRepository) GetNamespaces() ([]*Namespace, error) {
	var t []*Namespace
	err := r.db.Table(Namespace{}.TableName()).Where("tenant_id = ?", r.tenantID).Find(&t).Error
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *DBTenantRepository) SaveNamespace(namespace *Namespace) error {
	if namespace.TenantID == uuid.Nil {
		namespace.TenantID = r.tenantID
	}
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	return r.db.Save(namespace).Error
}

func (r *DBTenantRepository) CreateNamespace(namespace *Namespace) (*Namespace, error) {
	if namespace.TenantID == uuid.Nil {
		namespace.TenantID = r.tenantID
	}
	if namespace.ID == uuid.Nil {
		namespace.ID = uuid.NewV4()
	}
	length := len(namespace.Name)
	lockid := length + int([]rune(namespace.Name)[length-1])
	created := false
	err := dbsupport.Transaction(r.db, dbsupport.Lock(lockid, 10, func(tx *gorm.DB) error {
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

func (r *DBTenantRepository) DeleteNamespace(namespace *Namespace) error {
	return r.db.Unscoped().Delete(namespace).Error
}

func (r *DBTenantRepository) DeleteNamespaces() error {
	if r.tenantID == uuid.Nil {
		return nil
	}
	return r.db.Unscoped().Delete(&Namespace{}, "tenant_id = ?", r.tenantID).Error
}

func (r *DBTenantRepository) DeleteTenant() error {
	if r.tenantID == uuid.Nil {
		return nil
	}
	return r.db.Unscoped().Delete(&Tenant{ID: r.tenantID}).Error
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
