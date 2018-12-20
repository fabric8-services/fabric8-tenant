package testupdate

import (
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Tx(t *testing.T, DB *gorm.DB, do func(repo update.Repository) error) {
	tx := DB.Begin()
	require.NoError(t, tx.Error)
	repo := update.NewRepository(tx)
	if err := do(repo); err != nil {
		require.NoError(t, tx.Rollback().Error)
		assert.NoError(t, err)
	}
	require.NoError(t, tx.Commit().Error)
}

func AssertStatusAndAllVersionAreUpToDate(t *testing.T, db *gorm.DB, st update.Status, filterEnvType update.FilterEnvType) {
	var err error
	var tenantsUpdate *update.TenantsUpdate
	err = update.Transaction(db, func(tx *gorm.DB) error {
		tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
		return err
	})
	assert.Equal(t, string(st), string(tenantsUpdate.Status))
	for _, versionManager := range update.RetrieveVersionManagers() {
		isOk := true
		for _, envType := range versionManager.EnvTypes {
			isOk = isOk && filterEnvType.IsOk(envType)
		}
		if isOk {
			assert.True(t, versionManager.IsVersionUpToDate(tenantsUpdate))
		} else {
			assert.False(t, versionManager.IsVersionUpToDate(tenantsUpdate))
		}
	}
}

func UpdateVersionsTo(repo update.Repository, version string) error {
	tenantsUpdate, err := repo.GetTenantsUpdate()
	if err != nil {
		return err
	}
	for _, versionManager := range update.RetrieveVersionManagers() {
		if version != "" {
			versionManager.Version = version
		}
		versionManager.SetCurrentVersion(tenantsUpdate)
	}
	return repo.SaveTenantsUpdate(tenantsUpdate)
}
