package migration_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fabric8-services/fabric8-wit/configuration"
	"github.com/fabric8-services/fabric8-wit/gormsupport"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/fabric8-services/fabric8-wit/migration"
	"github.com/fabric8-services/fabric8-wit/resource"
)

const (
	databaseName           = "test"
	initialMigratedVersion = 5
)

var (
	config     *configuration.Registry
	migrations migration.Migrations
	dialect    gorm.Dialect
	sqlDB      *sql.DB
	gormDB     *gorm.DB
)

func setupTest(t *testing.T) {
	var err error
	config, err = configuration.Get()
	require.NoError(t, err, "failed to setup the configuration")

	configurationString := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s connect_timeout=%d",
		config.GetPostgresHost(),
		config.GetPostgresPort(),
		config.GetPostgresUser(),
		config.GetPostgresPassword(),
		config.GetPostgresSSLMode(),
		config.GetPostgresConnectionTimeout(),
	)

	db, err := sql.Open("postgres", configurationString)
	defer db.Close()
	require.NoError(t, err, "cannot connect to database: %s", databaseName)

	_, err = db.Exec("DROP DATABASE " + databaseName)
	if err != nil && !gormsupport.IsInvalidCatalogName(err) {
		require.NoError(t, err, "failed to drop database %s", databaseName)
	}

	_, err = db.Exec("CREATE DATABASE " + databaseName)
	require.NoError(t, err, "failed to create database %s", databaseName)
	migrations = migration.GetMigrations()
}

// migrateToVersion runs the migration of all the scripts to a certain version
func migrateToVersion(t *testing.T, db *sql.DB, m migration.Migrations, version int64) {
	var err error
	for nextVersion := int64(0); nextVersion < version && err == nil; nextVersion++ {
		var tx *sql.Tx
		tx, err = sqlDB.Begin()
		require.NoError(t, err, "failed to start tansaction for version %d", version)
		if err = migration.MigrateToNextVersion(tx, &nextVersion, m, databaseName); err != nil {
			errRollback := tx.Rollback()
			require.NoError(t, errRollback, "failed to roll back transaction for version %d", version)
			require.NoError(t, err, "failed to migrate to version %d", version)
		}

		err = tx.Commit()
		require.NoError(t, err, "error during transaction commit")
	}
}

// runSQLscript loads the given filename from the packaged SQL test files and
// executes it on the given database. Golang text/template module is used
// to handle all the optional arguments passed to the sql test files
func runSQLscript(db *sql.DB, sqlFilename string, args ...string) error {
	var tx *sql.Tx
	tx, err := db.Begin()
	if err != nil {
		return errs.Wrapf(err, "failed to start transaction with file %s", sqlFilename)
	}
	if err := executeSQLTestFile(sqlFilename, args...)(tx); err != nil {
		log.Warn(nil, nil, "failed to execute data insertion using '%s': %s\n", sqlFilename, err)
		errRollback := tx.Rollback()
		if errRollback != nil {
			return errs.Wrapf(err, "error while rolling back transaction for file %s", sqlFilename)
		}
		return errs.Wrapf(err, "failed to execute data insertion using file %s", sqlFilename)
	}
	err = tx.Commit()
	return errs.Wrapf(err, "error during transaction commit for file %s", sqlFilename)
}

func TestMigrations(t *testing.T) {
	resource.Require(t, resource.Database)
	setupTest(t)

	configurationString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		config.GetPostgresHost(),
		config.GetPostgresPort(),
		config.GetPostgresUser(),
		config.GetPostgresPassword(),
		databaseName,
		config.GetPostgresSSLMode(),
		config.GetPostgresConnectionTimeout(),
	)
	var err error
	sqlDB, err = sql.Open("postgres", configurationString)
	defer sqlDB.Close()
	require.NoError(t, err, "cannot connect to DB %s", databaseName)

	gormDB, err = gorm.Open("postgres", configurationString)
	defer gormDB.Close()
	require.NoError(t, err, "cannot connect to DB %s", databaseName)
	dialect = gormDB.Dialect()
	dialect.SetDB(sqlDB)

	// We migrate the new database until initialMigratedVersion
	t.Run("TestMigration4", testMigration4)

	// Tests normal, for the subsequent tests for new migration add entries below
	// without changing the sequence

	// added new column to the tenant, oso_username
	t.Run("TestMigration5", testMigration5AddColumnOSOUsername)
}

func testMigration4(t *testing.T) {
	var err error
	m := migrations[:initialMigratedVersion]
	for nextVersion := int64(0); nextVersion < int64(len(m)) && err == nil; nextVersion++ {
		var tx *sql.Tx
		tx, err = sqlDB.Begin()
		require.NoError(t, err, "failed to start transaction")

		if err = migration.MigrateToNextVersion(tx, &nextVersion, m, databaseName); err != nil {
			t.Errorf("failed to migrate to version %d: %s\n", nextVersion, err)
			errRollback := tx.Rollback()
			require.NoError(t, errRollback, "error while rolling back transaction")
			require.NoError(t, err, "failed to migrate to version after rolling back")
		}

		err = tx.Commit()
		require.NoError(t, err, "error during transaction commit")
	}
}

func testMigration5AddColumnOSOUsername(t *testing.T) {
	version := initialMigratedVersion + 1

	// before migration, should have the table
	require.True(t, gormDB.HasTable("tenants"))
	// before migration, should not have the column
	require.False(t, dialect.HasColumn("teants", "oso_username"))
	migrateToVersion(t, sqlDB, migrations[:version], int64(version))

	// after migration, should have the column
	require.True(t, dialect.HasColumn("teants", "oso_username"))

	// adding data to the table with inclusion of this new column
	assert.Nil(t, runSQLscript(sqlDB, "005-add-data-to-tenants-oso-username.sql"))
}
