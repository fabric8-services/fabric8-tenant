package update

import (
	"database/sql/driver"
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"time"
)

const TenantsUpdateTableName = "tenants_update"

type Status string

const (
	Finished   Status = "finished"
	Updating   Status = "updating"
	Failed     Status = "failed"
	Killed     Status = "killed"
	Incomplete Status = "incomplete"
)

// Value - Implementation of valuer for database/sql
func (s Status) Value() (driver.Value, error) {
	return string(s), nil
}

// Scan - Implement the database/sql scanner interface
func (s *Status) Scan(value interface{}) error {
	if value == nil {
		*s = Status("")
		return nil
	}
	if bv, err := driver.String.ConvertValue(value); err == nil {
		// if this is a bool type
		if v, ok := bv.(string); ok {
			// set the value of the pointer yne to YesNoEnum(v)
			*s = Status(v)
			return nil
		}
	}
	// otherwise, return an error
	return errors.New("failed to scan status")
}

func (s Status) String() string {
	return string(s)
}

type TenantsUpdate struct {
	LastVersionFabric8TenantUserFile          string
	LastVersionFabric8TenantCheMtFile         string
	LastVersionFabric8TenantCheQuotasFile     string
	LastVersionFabric8TenantJenkinsFile       string
	LastVersionFabric8TenantJenkinsQuotasFile string
	LastVersionFabric8TenantDeployFile        string
	Status                                    Status
	FailedCount                               int
	LastTimeUpdated                           time.Time
	CanContinue                               bool
}

type Repository interface {
	GetTenantsUpdate() (*TenantsUpdate, error)
	SaveTenantsUpdate(tenantUpdate *TenantsUpdate) error
	UpdateStatus(status Status) error
	UpdateLastTimeUpdated() error
	PrepareForUpdating() error
	IncrementFailedCount() error
	CanContinue() (bool, error)
	Stop() error
}

type GormRepository struct {
	tx *gorm.DB
}

func NewRepository(tx *gorm.DB) *GormRepository {
	return &GormRepository{
		tx: tx,
	}
}

func (r *GormRepository) GetTenantsUpdate() (*TenantsUpdate, error) {
	var tenantsUpdate TenantsUpdate
	err := r.tx.Table(TenantsUpdateTableName).Find(&tenantsUpdate).Error
	if err != nil {
		return &tenantsUpdate, errors.Wrapf(err, "failed to get TenantsUpdate entity from the table %s", TenantsUpdateTableName)
	}
	return &tenantsUpdate, nil
}

func (r *GormRepository) SaveTenantsUpdate(tenantUpdate *TenantsUpdate) error {
	err := r.tx.Table(TenantsUpdateTableName).Updates(tenantUpdate).Error
	if err != nil {
		return errors.Wrapf(err, "failed to update TenantsUpdate entity to values %+v", tenantUpdate)
	}
	return nil
}

func (r *GormRepository) IncrementFailedCount() error {
	query := fmt.Sprintf("UPDATE %s SET failed_count = failed_count + 1", TenantsUpdateTableName)
	if err := r.tx.Exec(query).Error; err != nil {
		return errors.Wrapf(err, "failed to increment failed_count in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) UpdateStatus(status Status) error {
	err := r.tx.Table(TenantsUpdateTableName).UpdateColumn("status", status).Error
	if err != nil {
		return errors.Wrapf(err, "failed to update status in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) PrepareForUpdating() error {
	err := r.tx.Table(TenantsUpdateTableName).
		UpdateColumn("status", Updating).
		UpdateColumn("failed_count", 0).
		UpdateColumn("last_time_updated", time.Now()).
		UpdateColumn("can_continue", true).Error
	if err != nil {
		return errors.Wrapf(err, "failed to update status in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) UpdateLastTimeUpdated() error {
	err := r.tx.Table(TenantsUpdateTableName).UpdateColumn("last_time_updated", time.Now()).Error
	if err != nil {
		return errors.Wrapf(err, "failed to set last_time_updated to NOW() in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) CanContinue() (bool, error) {
	var tenantsUpdate TenantsUpdate
	err := r.tx.Table(TenantsUpdateTableName).Select("can_continue").Find(&tenantsUpdate).Error
	if err != nil {
		return false, errors.Wrapf(err, "failed to get can_continue column of TenantsUpdate entity from the table %s", TenantsUpdateTableName)
	}
	return tenantsUpdate.CanContinue, nil
}

func (r *GormRepository) Stop() error {
	err := r.tx.Table(TenantsUpdateTableName).UpdateColumn("can_continue", false).Error
	if err != nil {
		return errors.Wrapf(err, "failed to set can_continue to false in %s table", TenantsUpdateTableName)
	}
	return nil
}
