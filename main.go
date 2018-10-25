package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/migration"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	witmiddleware "github.com/fabric8-services/fabric8-wit/goamiddleware"
	"github.com/goadesign/goa"
	"github.com/goadesign/goa/logging/logrus"
	"github.com/goadesign/goa/middleware"
	"github.com/goadesign/goa/middleware/gzip"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {

	viper.GetStringMapString("TEST")

	var migrateDB bool
	flag.BoolVar(&migrateDB, "migrateDatabase", false, "Migrates the database to the newest version and exits.")
	flag.Parse()

	// Initialized configuration
	config, err := configuration.GetData()
	if err != nil {
		logrus.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to setup the configuration")
	}

	errorMsg := checkTemplateVersions()
	if errorMsg != "" {
		log.Panic(nil, map[string]interface{}{}, errorMsg)
	}

	db := connect(config)
	defer db.Close()
	migrate(db)

	// Nothing to here except exit, since the migration is already performed.
	if migrateDB {
		os.Exit(0)
	}

	toggles.Init("f8tenant", config.GetTogglesURL())

	authService, err := auth.NewAuthService(config)
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to initialize the auth.Service component")
	}

	publicKeys, err := authService.GetPublicKeys()
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err":    err,
			"target": config.GetAuthURL(),
		}, "failed to fetch public keys from token service")
	}

	// Create service
	service := goa.New("tenant")

	// Mount middleware
	service.WithLogger(goalogrus.New(log.Logger()))
	service.Use(middleware.RequestID())
	service.Use(gzip.Middleware(9))
	service.Use(jsonapi.ErrorHandler(service, true))
	service.Use(middleware.Recover())

	service.Use(witmiddleware.TokenContext(publicKeys, nil, app.NewJWTSecurity()))
	service.Use(log.LogRequest(config.IsDeveloperModeEnabled()))
	app.UseJWTMiddleware(service, goajwt.New(publicKeys, nil, app.NewJWTSecurity()))

	clusterService := cluster.NewClusterService(config.GetClustersRefreshDelay(), authService)
	err = clusterService.Start()
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to initialize the cluster.Service component")
	}
	defer clusterService.Stop()

	haltSentry, err := sentry.InitializeLogger(config, controller.Commit)
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to setup the sentry client")
	}
	defer haltSentry()

	tenantService := tenant.NewDBService(db)

	// Mount "status" controller
	statusCtrl := controller.NewStatusController(service, db)
	app.MountStatusController(service, statusCtrl)

	// Mount "tenant" controller
	tenantCtrl := controller.NewTenantController(service, clusterService, authService, config, tenantService)
	app.MountTenantController(service, tenantCtrl)

	tenantsCtrl := controller.NewTenantsController(service, tenantService, clusterService, authService, config)
	app.MountTenantsController(service, tenantsCtrl)

	log.Logger().Infoln("Git Commit SHA: ", controller.Commit)
	log.Logger().Infoln("UTC Build Time: ", controller.BuildTime)
	log.Logger().Infoln("UTC Start Time: ", controller.StartTime)
	log.Logger().Infoln("Dev mode:       ", config.IsDeveloperModeEnabled())
	log.Logger().Infoln("Auth URL:       ", config.GetAuthURL())

	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.Handle("/", service.Mux)

	// Start/mount metrics http
	if config.GetHTTPAddress() == config.GetMetricsHTTPAddress() {
		http.Handle("/metrics", prometheus.Handler())
	} else {
		go func(metricAddress string) {
			mx := http.NewServeMux()
			mx.Handle("/metrics", prometheus.Handler())
			if err := http.ListenAndServe(metricAddress, mx); err != nil {
				log.Error(nil, map[string]interface{}{
					"addr": metricAddress,
					"err":  err,
				}, "unable to connect to metrics server")
				service.LogError("startup", "err", err)
			}
		}(config.GetMetricsHTTPAddress())
	}

	// Start http
	if err := http.ListenAndServe(config.GetHTTPAddress(), nil); err != nil {
		log.Error(nil, map[string]interface{}{
			"addr": config.GetHTTPAddress(),
			"err":  err,
		}, "unable to connect to server")
		service.LogError("startup", "err", err)
	}
}

func checkTemplateVersions() string {
	errorMsg := ""
	if environment.VersionFabric8TenantUserFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantUserFile")
	} else {
		logVersionInfo("fabric8-tenant-user.yml", environment.VersionFabric8TenantUserFile)
	}
	if environment.VersionFabric8TenantJenkinsFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantJenkinsFile")
	} else {
		logVersionInfo("fabric8-tenant-jenkins.yml", environment.VersionFabric8TenantJenkinsFile)
	}
	if environment.VersionFabric8TenantJenkinsQuotasFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantJenkinsQuotasFile")
	} else {
		logVersionInfo("fabric8-tenant-jenkins-quotas.yml", environment.VersionFabric8TenantJenkinsQuotasFile)
	}
	if environment.VersionFabric8TenantDeployFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantDeployFile")
	} else {
		logVersionInfo("fabric8-tenant-deploy.yml", environment.VersionFabric8TenantDeployFile)
	}
	if environment.VersionFabric8TenantCheMtFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantCheMtFile")
	} else {
		logVersionInfo("fabric8-tenant-che-mt.yml", environment.VersionFabric8TenantCheMtFile)
	}
	if environment.VersionFabric8TenantCheFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantCheFile")
	} else {
		logVersionInfo("fabric8-tenant-che.yml", environment.VersionFabric8TenantCheFile)
	}
	if environment.VersionFabric8TenantCheQuotasFile == "" {
		errorMsg = errorMsg + createNotSetVersionError("VersionFabric8TenantCheQuotasFile")
	} else {
		logVersionInfo("fabric8-tenant-che-quotas.yml", environment.VersionFabric8TenantCheQuotasFile)
	}
	return errorMsg
}

func createNotSetVersionError(variable string) string {
	return fmt.Sprintf("The variable %s representing a template version is not set.\n", variable)
}

func logVersionInfo(target, version string) {
	log.Logger().Infof("Using %s of version: %s", target, version)
}

func connect(config *configuration.Data) *gorm.DB {
	var err error
	var db *gorm.DB
	for {
		db, err = gorm.Open("postgres", config.GetPostgresConfigString())
		if err != nil {
			log.Logger().Errorf("ERROR: Unable to open connection to database %v", err)
			log.Logger().Infof("Retrying to connect in %v...", config.GetPostgresConnectionRetrySleep())
			time.Sleep(config.GetPostgresConnectionRetrySleep())
		} else {
			break
		}
	}

	if config.IsDeveloperModeEnabled() {
		db = db.Debug()
	}

	if config.GetPostgresConnectionMaxIdle() > 0 {
		log.Logger().Infof("Configured connection pool max idle %v", config.GetPostgresConnectionMaxIdle())
		db.DB().SetMaxIdleConns(config.GetPostgresConnectionMaxIdle())
	}
	if config.GetPostgresConnectionMaxOpen() > 0 {
		log.Logger().Infof("Configured connection pool max open %v", config.GetPostgresConnectionMaxOpen())
		db.DB().SetMaxOpenConns(config.GetPostgresConnectionMaxOpen())
	}
	return db
}

func migrate(db *gorm.DB) {
	// Migrate the schema
	err := migration.Migrate(db.DB())
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed migration")
	}
}
