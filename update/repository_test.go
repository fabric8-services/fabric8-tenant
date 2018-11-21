package update_test

import (
	"testing"

	"fmt"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"time"
)

type UpdateRepoTestSuite struct {
	gormsupport.DBTestSuite
}

func TestUpdateRepository(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &UpdateRepoTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *UpdateRepoTestSuite) TestUpdateAndGetStatus() {
	s.T().Run("set and get status should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			return update.NewRepository(tx).UpdateStatus(update.Updating)
		})
		require.NoError(s.T(), err)

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			repo := update.NewRepository(tx)
			tenantsUpdate, err := repo.GetTenantsUpdate()
			if err != nil {
				return err
			}
			assert.Equal(t, string(update.Updating), string(tenantsUpdate.Status))
			return repo.UpdateStatus(update.Finished)
		})

		// then
		assert.NoError(t, err)
		var tenantsUpdate *update.TenantsUpdate
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, string(update.Finished), string(tenantsUpdate.Status))
	})
}

func (s *UpdateRepoTestSuite) TestIncrementAndGetFailedCount() {

	s.T().Run("increment and get failed_count should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			return update.NewRepository(tx).PrepareForUpdating()
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			repo := update.NewRepository(tx)
			for i := 0; i < 10; i++ {
				if err := repo.IncrementFailedCount(); err != nil {
					return err
				}
			}
			return nil
		})

		// then
		assert.NoError(t, err)
		var tenantsUpdate *update.TenantsUpdate
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, 10, tenantsUpdate.FailedCount)
	})
}

func (s *UpdateRepoTestSuite) TestGetAndSetLastTimeUpdated() {

	s.T().Run("set and get last_time_updated should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			return update.NewRepository(tx).UpdateLastTimeUpdated()
		})
		require.NoError(t, err)
		before := time.Now()

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			return update.NewRepository(tx).UpdateLastTimeUpdated()
		})

		// then
		assert.NoError(t, err)
		var tenantsUpdate *update.TenantsUpdate
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		assert.NoError(t, err)
		assert.True(t, before.Before(tenantsUpdate.LastTimeUpdated))
	})
}

func (s *UpdateRepoTestSuite) TestPrepareForUpdating() {

	s.T().Run("set and get last_time_updated should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			repo := update.NewRepository(tx)
			if err := repo.UpdateStatus(update.Updating); err != nil {
				return err
			}
			if err := repo.UpdateLastTimeUpdated(); err != nil {
				return err
			}
			return repo.IncrementFailedCount()
		})
		require.NoError(t, err)
		before := time.Now()

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			return update.NewRepository(tx).PrepareForUpdating()
		})

		// then
		assert.NoError(t, err)
		var tenantsUpdate *update.TenantsUpdate
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, string(update.Updating), string(tenantsUpdate.Status))
		assert.True(t, before.Before(tenantsUpdate.LastTimeUpdated))
		assert.Equal(t, 0, tenantsUpdate.FailedCount)
	})
}

func (s *UpdateRepoTestSuite) TestOperationOverVersions() {

	s.T().Run("should say that all versions are different", func(t *testing.T) {
		// given
		testdoubles.SetTemplateVersions()
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			err := updateVersionsTo(update.NewRepository(tx), "000bbb")
			require.NoError(t, err)
			return err
		})
		require.NoError(t, err)

		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			// when
			tenantsUpdate, err := update.NewRepository(tx).GetTenantsUpdate()
			if err != nil {
				return err
			}
			// then
			for _, versionManager := range update.RetrieveVersionManagers() {
				assert.False(t, versionManager.IsVersionUpToDate(tenantsUpdate))
			}
			return nil
		})
		assert.NoError(t, err)
	})

	s.T().Run("should say that all versions but one are different", func(t *testing.T) {
		// given
		testdoubles.SetTemplateVersions()
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			return updateVersionsTo(update.NewRepository(tx), "123abc")
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err := update.NewRepository(tx).GetTenantsUpdate()
			if err != nil {
				return err
			}
			for _, versionManager := range update.RetrieveVersionManagers() {
				// then
				if versionManager.IsVersionUpToDate(tenantsUpdate) {
					assert.Len(t, versionManager.EnvTypes, 1)
					assert.Equal(t, "che", string(versionManager.EnvTypes[0]))
				}
			}
			return nil
		})

		// then
		assert.NoError(t, err)
	})

	s.T().Run("should say that all versions are same", func(t *testing.T) {
		// given
		testdoubles.SetTemplateVersions()
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			return updateVersionsTo(update.NewRepository(tx), "")
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err := update.NewRepository(tx).GetTenantsUpdate()
			if err != nil {
				return err
			}
			for _, versionManager := range update.RetrieveVersionManagers() {
				// then
				assert.True(t, versionManager.IsVersionUpToDate(tenantsUpdate))
			}
			return nil
		})

		// then
		assert.NoError(t, err)
	})
}

func (s *UpdateRepoTestSuite) TestRollBack() {

	s.T().Run("when an error is returned none of the operation in transaction should be committed", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			repo := update.NewRepository(tx)
			if err := repo.PrepareForUpdating(); err != nil {
				return err
			}
			if err := repo.UpdateStatus(update.Failed); err != nil {
				return err
			}
			if err := repo.IncrementFailedCount(); err != nil {
				return err
			}
			return updateVersionsTo(repo, "000abc")
		})
		require.NoError(s.T(), err)
		before := time.Now()

		// when
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			if err := update.NewRepository(tx).PrepareForUpdating(); err != nil {
				return err
			}
			if err := update.NewRepository(tx).UpdateStatus(update.Finished); err != nil {
				return err
			}
			if err := update.NewRepository(tx).IncrementFailedCount(); err != nil {
				return err
			}
			if err := update.NewRepository(tx).IncrementFailedCount(); err != nil {
				return err
			}
			if err := updateVersionsTo(update.NewRepository(tx), ""); err != nil {
				return err
			}
			return fmt.Errorf("any error")
		})

		// then
		test.AssertError(t, err, test.HasMessage("any error"))

		var tenantsUpdate *update.TenantsUpdate
		err = update.Transaction(s.DB, func(tx *gorm.DB) error {
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, string(update.Failed), string(tenantsUpdate.Status))
		assert.True(t, before.After(tenantsUpdate.LastTimeUpdated))
		assert.Equal(t, 1, tenantsUpdate.FailedCount)
		for _, versionManager := range update.RetrieveVersionManagers() {
			assert.False(t, versionManager.IsVersionUpToDate(tenantsUpdate))
		}
	})
}
