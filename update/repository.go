package update

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/pkg/errors"
	"strings"
	"time"
)

const TenantsUpdateTableName = "tenants_update"

type Status string

const (
	Finished Status = "finished"
	Updating Status = "updating"
	Failed   Status = "failed"
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

type Repository interface {
	GetStatus() (Status, error)
	UpdateStatus(status Status) error
	GetLastTimeUpdated() (time.Time, error)
	UpdateLastTimeUpdated() error
	PrepareForUpdating() error
	GetFailedCount() (int, error)
	IncrementFailedCount() error
	IsVersionSame(fileVersionAttrName string, expectedVersion string) (bool, error)
	UpdateVersionsTo(attrNameMapping map[string]*VersionWithTypes) error
}

type GormRepository struct {
	tx *sql.Tx
}

func NewRepository(tx *sql.Tx) *GormRepository {
	return &GormRepository{
		tx: tx,
	}
}

func (r *GormRepository) GetStatus() (Status, error) {
	query := fmt.Sprintf("SELECT status FROM %s", TenantsUpdateTableName)
	row := r.tx.QueryRow(query)
	var status Status
	if err := row.Scan(&status); err != nil {
		return status, errors.Wrapf(err, "failed to scan status in table %s", TenantsUpdateTableName)
	}

	return status, nil
}

func (r *GormRepository) GetLastTimeUpdated() (time.Time, error) {
	query := fmt.Sprintf("SELECT last_time_updated FROM %s", TenantsUpdateTableName)
	row := r.tx.QueryRow(query)

	var lastTimeUpdated time.Time
	if err := row.Scan(&lastTimeUpdated); err != nil {
		return lastTimeUpdated, errors.Wrapf(err, "failed to scan last_time_updated in table %s", TenantsUpdateTableName)
	}

	return lastTimeUpdated, nil
}

func (r *GormRepository) IsVersionSame(fileVersionAttrName string, expectedVersion string) (bool, error) {
	row := r.tx.QueryRow(fmt.Sprintf("SELECT EXISTS (SELECT * FROM %s tu WHERE tu.%s='%s')", TenantsUpdateTableName, fileVersionAttrName, expectedVersion))

	var isSame bool
	if err := row.Scan(&isSame); err != nil {
		return isSame, errors.Wrapf(err, "failed to scan bool value while checking if %s is same as %s in table %s",
			fileVersionAttrName, expectedVersion, TenantsUpdateTableName)
	}

	return isSame, nil
}

func (r *GormRepository) UpdateVersionsTo(attrNameMapping map[string]*VersionWithTypes) error {
	query := fmt.Sprintf("UPDATE %s SET ", TenantsUpdateTableName)
	var values []string
	for attr, versionWithTypes := range attrNameMapping {
		values = append(values, fmt.Sprintf("%s = '%s'", attr, versionWithTypes.Version))
	}

	finalQuery := query + strings.Join(values, ", ")
	if _, err := r.tx.Exec(finalQuery); err != nil {
		return errors.Wrapf(err, "failed to update status in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) GetFailedCount() (int, error) {
	query := fmt.Sprintf("SELECT failed_count FROM %s", TenantsUpdateTableName)
	row := r.tx.QueryRow(query)
	var count int
	if err := row.Scan(&count); err != nil {
		return count, errors.Wrapf(err, "failed to scan failed_count in table %s", TenantsUpdateTableName)
	}

	return count, nil
}

func (r *GormRepository) IncrementFailedCount() error {
	query := fmt.Sprintf("UPDATE %s SET failed_count = failed_count + 1", TenantsUpdateTableName)
	if _, err := r.tx.Exec(query); err != nil {
		return errors.Wrapf(err, "failed to increment failed_count in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) UpdateStatus(status Status) error {
	query := fmt.Sprintf("UPDATE %s SET status = '%s'", TenantsUpdateTableName, status)
	if _, err := r.tx.Exec(query); err != nil {
		return errors.Wrapf(err, "failed to update status in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) PrepareForUpdating() error {
	query := fmt.Sprintf("UPDATE %s SET status = '%s', failed_count = 0, last_time_updated = NOW()", TenantsUpdateTableName, Updating)
	_, err := r.tx.Exec(query)
	if err != nil {
		return errors.Wrapf(err, "failed to update status in %s table", TenantsUpdateTableName)
	}
	return nil
}

func (r *GormRepository) UpdateLastTimeUpdated() error {
	query := fmt.Sprintf("UPDATE %s SET last_time_updated = NOW()", TenantsUpdateTableName)
	_, err := r.tx.Exec(query)
	if err != nil {
		return errors.Wrapf(err, "failed to set last_time_updated to NOW() in %s table", TenantsUpdateTableName)
	}
	return nil
}
