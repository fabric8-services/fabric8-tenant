package gormsupport

import (
	"os"

	"github.com/fabric8-services/fabric8-common/log"
	config "github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/migration"
	"github.com/fabric8-services/fabric8-tenant/test/resource"

	"context"

	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq" // need to import postgres driver
	"github.com/stretchr/testify/suite"
)

var _ suite.SetupAllSuite = &DBTestSuite{}
var _ suite.TearDownAllSuite = &DBTestSuite{}

// NewDBTestSuite instanciate a new DBTestSuite
func NewDBTestSuite(configFilePath string) DBTestSuite {
	return DBTestSuite{configFile: configFilePath}
}

// DBTestSuite is a base for tests using a gorm db
type DBTestSuite struct {
	suite.Suite
	configFile    string
	Configuration *config.Data
	DB            *gorm.DB
	Repo          tenant.Service
	clean         func()
	Ctx           context.Context
}

// SetupSuite implements suite.SetupAllSuite
func (s *DBTestSuite) SetupSuite() {
	ready, _ := resource.IsReady(resource.Database)
	if ready {
		configuration, err := config.NewData()
		if err != nil {
			log.Panic(nil, map[string]interface{}{
				"err": err,
			}, "failed to setup the configuration")
		}
		s.Configuration = configuration
		if _, c := os.LookupEnv(resource.Database); c != false {
			s.DB, err = gorm.Open("postgres", s.Configuration.GetPostgresConfigString())
			if err != nil {
				log.Panic(nil, map[string]interface{}{
					"err":             err,
					"postgres_config": configuration.GetPostgresConfigString(),
				}, "failed to connect to the database")
			}
		}
		s.Ctx = migration.NewMigrationContext(context.Background())
	}
}

// SetupTest implements suite.SetupTest
func (s *DBTestSuite) SetupTest() {
	if s.DB != nil {
		s.clean = DeleteCreatedEntities(s.DB)
		s.Repo = tenant.NewDBService(s.DB)
	} else {
		repo, _ := NewEmptyDBServiceStub()
		s.Repo = repo
		s.clean = func() {
			repo, _ := NewEmptyDBServiceStub()
			s.Repo = repo
		}
	}
}

// TearDownTest implements suite.TearDownTest
func (s *DBTestSuite) TearDownTest() {
	s.clean()
}

// TearDownSuite implements suite.TearDownAllSuite
func (s *DBTestSuite) TearDownSuite() {
	if s.DB != nil {
		s.DB.Close()
	}
}

// DisableGormCallbacks will turn off gorm's automatic setting of `created_at`
// and `updated_at` columns. Call this function and make sure to `defer` the
// returned function.
//
//    resetFn := DisableGormCallbacks()
//    defer resetFn()
func (s *DBTestSuite) DisableGormCallbacks() func() {
	gormCallbackName := "gorm:update_time_stamp"
	// remember old callbacks
	oldCreateCallback := s.DB.Callback().Create().Get(gormCallbackName)
	oldUpdateCallback := s.DB.Callback().Update().Get(gormCallbackName)
	// remove current callbacks
	s.DB.Callback().Create().Remove(gormCallbackName)
	s.DB.Callback().Update().Remove(gormCallbackName)
	// return a function to restore old callbacks
	return func() {
		s.DB.Callback().Create().Register(gormCallbackName, oldCreateCallback)
		s.DB.Callback().Update().Register(gormCallbackName, oldUpdateCallback)
	}
}
