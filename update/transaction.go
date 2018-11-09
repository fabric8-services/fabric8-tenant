package update

import (
	"database/sql"
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/pkg/errors"
)

const TenantsUpdateAdvisoryLockID = 4242

func Transaction(db *sql.DB, lockAndDo func(tx *sql.Tx) error) error {
	var err error

	if db == nil {
		return fmt.Errorf("Database handle is nil\n")
	}

	var tx *sql.Tx

	tx, err = db.Begin()
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}

	err = lockAndDo(tx)

	if err != nil {
		oldErr := err
		log.Info(nil, map[string]interface{}{
			"err": err,
		}, "Rolling back transaction due to: %v", err)

		if err = tx.Rollback(); err != nil {
			log.Error(nil, map[string]interface{}{
				"err": err,
			}, "error while rolling back transaction: ", err)
			return errors.Wrap(err, "error while rolling back transaction")
		}
		return oldErr
	}

	if err = tx.Commit(); err != nil {
		log.Error(nil, map[string]interface{}{
			"err": err,
		}, "error during transaction commit: %v", err)
		return errors.Wrap(err, "error during transaction commit")
	}

	if err != nil {
		log.Error(nil, map[string]interface{}{
			"err": err,
		}, "tenants update failed with error: %v", err)
		return errors.Wrap(err, "tenants update failed with error")
	}

	return nil
}

type lockAndDo func(tx *sql.Tx) error

func lock(do func(repo Repository) error) lockAndDo {
	return func(tx *sql.Tx) error {
		if _, err := tx.Exec("SELECT pg_advisory_xact_lock($1)", TenantsUpdateAdvisoryLockID); err != nil {
			return errors.Wrap(err, "failed to acquire lock")
		}

		return do(NewRepository(tx))
	}
}
