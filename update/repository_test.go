package update_test

import (
	"testing"

	"database/sql"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	"github.com/fabric8-services/fabric8-tenant/update"
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
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateStatus(update.Updating)
		})
		require.NoError(s.T(), err)

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			repo := update.NewRepository(tx)
			status, err := repo.GetStatus()
			if err != nil {
				return err
			}
			assert.Equal(t, update.Updating, status)
			return repo.UpdateStatus(update.Finished)
		})

		// then
		assert.NoError(t, err)
		var actualStatus update.Status
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			actualStatus, err = update.NewRepository(tx).GetStatus()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, update.Finished, actualStatus)
	})
}

func (s *UpdateRepoTestSuite) TestIncrementAndGetFailedCount() {

	s.T().Run("increment and get failed_count should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).PrepareForUpdating()
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			repo := update.NewRepository(tx)
			for i := 0; i < 10; i++ {
				if repo.IncrementFailedCount() != nil {
					return err
				}
			}
			return nil
		})

		// then
		assert.NoError(t, err)
		var failedCount int
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			failedCount, err = update.NewRepository(tx).GetFailedCount()
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, 10, failedCount)
	})
}

func (s *UpdateRepoTestSuite) TestGetAndSetLastTimeUpdated() {

	s.T().Run("set and get last_time_updated should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateLastTimeUpdated()
		})
		require.NoError(t, err)
		before := time.Now()

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateLastTimeUpdated()
		})

		// then
		assert.NoError(t, err)
		var updatedTime time.Time
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			updatedTime, err = update.NewRepository(tx).GetLastTimeUpdated()
			return err
		})
		assert.True(t, before.Before(updatedTime))
	})
}

func (s *UpdateRepoTestSuite) TestPrepareForUpdating() {

	s.T().Run("set and get last_time_updated should pass", func(t *testing.T) {
		// given
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
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
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).PrepareForUpdating()
		})

		// then
		assert.NoError(t, err)
		var updatedTime time.Time
		var actualStatus update.Status
		var failedCount int
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			if actualStatus, err = update.NewRepository(tx).GetStatus(); err != nil {
				return err
			}
			if updatedTime, err = update.NewRepository(tx).GetLastTimeUpdated(); err != nil {
				return err
			}
			if failedCount, err = update.NewRepository(tx).GetFailedCount(); err != nil {
				return err
			}
			return err
		})
		assert.NoError(t, err)
		assert.Equal(t, update.Updating, actualStatus)
		assert.True(t, before.Before(updatedTime))
		assert.Equal(t, 0, failedCount)
	})
}

func (s *UpdateRepoTestSuite) TestOperationOverVersions() {

	s.T().Run("should say that all versions are different", func(t *testing.T) {
		// given
		testdoubles.SetTemplateVersions()
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateVersionsTo(retrieveMappingWithVersion("000bbb"))
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			for attrName, versionWithTypes := range update.RetrieveAttrNameMapping() {
				// when

				isVersionSame, err := update.NewRepository(tx).IsVersionSame(attrName, versionWithTypes.Version)
				if err != nil {
					return err
				}
				// then
				assert.False(t, isVersionSame)
			}
			return nil
		})

		// then
		assert.NoError(t, err)
	})

	s.T().Run("should say that all versions but one are different", func(t *testing.T) {
		// given
		testdoubles.SetTemplateVersions()
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateVersionsTo(retrieveMappingWithVersion("123abc"))
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			for attrName, versionWithTypes := range update.RetrieveAttrNameMapping() {
				// when

				isVersionSame, err := update.NewRepository(tx).IsVersionSame(attrName, versionWithTypes.Version)
				if err != nil {
					return err
				}
				// then
				if attrName == "last_version_fabric8_tenant_che_file" {
					assert.True(t, isVersionSame)
				} else {
					assert.False(t, isVersionSame)
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
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			return update.NewRepository(tx).UpdateVersionsTo(update.RetrieveAttrNameMapping())
		})
		require.NoError(t, err)

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			for attrName, versionWithTypes := range update.RetrieveAttrNameMapping() {
				// when

				isVersionSame, err := update.NewRepository(tx).IsVersionSame(attrName, versionWithTypes.Version)
				if err != nil {
					return err
				}
				// then
				assert.True(t, isVersionSame)
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
		err := update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			if err := update.NewRepository(tx).PrepareForUpdating(); err != nil {
				return err
			}
			if err := update.NewRepository(tx).UpdateStatus(update.Failed); err != nil {
				return err
			}
			if err := update.NewRepository(tx).IncrementFailedCount(); err != nil {
				return err
			}
			return update.NewRepository(tx).UpdateVersionsTo(retrieveMappingWithVersion("000abc"))
		})
		require.NoError(s.T(), err)
		before := time.Now()

		// when
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
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
			if err := update.NewRepository(tx).UpdateVersionsTo(update.RetrieveAttrNameMapping()); err != nil {
				return err
			}
			return fmt.Errorf("any error")
		})

		// then
		test.AssertError(t, err, test.HasMessage("any error"))
		var updatedTime time.Time
		var actualStatus update.Status
		var failedCount int
		err = update.Transaction(s.DB.DB(), func(tx *sql.Tx) error {
			if actualStatus, err = update.NewRepository(tx).GetStatus(); err != nil {
				return err
			}
			if updatedTime, err = update.NewRepository(tx).GetLastTimeUpdated(); err != nil {
				return err
			}
			if failedCount, err = update.NewRepository(tx).GetFailedCount(); err != nil {
				return err
			}
			for attrName, versionWithTypes := range update.RetrieveAttrNameMapping() {
				// when

				isVersionSame, err := update.NewRepository(tx).IsVersionSame(attrName, versionWithTypes.Version)
				if err != nil {
					return err
				}
				// then
				assert.False(t, isVersionSame)
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, update.Failed, actualStatus)
		assert.True(t, before.After(updatedTime))
		assert.Equal(t, 1, failedCount)
	})
}
