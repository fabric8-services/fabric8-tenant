package dbsupport

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func Transaction(db *gorm.DB, lockAndDo LockAndDo) error {
	var err error

	if db == nil {
		return fmt.Errorf("Database handle is nil\n")
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if tx.Error != nil {
		return errors.Wrap(err, "failed to start transaction")
	}

	err = lockAndDo(tx)

	if err != nil {
		oldErr := err
		log.Info(nil, map[string]interface{}{
			"err": err,
		}, "Rolling back transaction due to: %v", err)

		if err = tx.Rollback().Error; err != nil {
			log.Error(nil, map[string]interface{}{
				"err": err,
			}, "error while rolling back transaction: %v", err)
			return errors.Wrap(err, "error while rolling back transaction")
		}
		return oldErr
	}

	if err = tx.Commit().Error; err != nil {
		log.Error(nil, map[string]interface{}{
			"err": err,
		}, "error during transaction commit: %v", err)
		return errors.Wrap(err, "error during transaction commit")
	}

	return nil
}

type LockAndDo func(tx *gorm.DB) error
