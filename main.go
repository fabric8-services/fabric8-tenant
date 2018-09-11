package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/migration"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-tenant/user"
	witmiddleware "github.com/fabric8-services/fabric8-wit/goamiddleware"
	"github.com/fabric8-services/fabric8-wit/log"
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

	// Initialized developer mode flag for the logger
	log.InitializeLogger(config.IsLogJSON(), config.GetLogLevel())

	db := connect(config)
	defer db.Close()
	migrate(db)

	// Nothing to here except exit, since the migration is already performed.
	if migrateDB {
		os.Exit(0)
	}

	toggles.Init("f8tenant", config.GetTogglesURL())

	publicKeys, err := token.GetPublicKeys(context.Background(), config.GetAuthURL())
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

	// fetch service account token for tenant service
	saTokenService := token.NewServiceAccountTokenService(config)
	saToken, err := saTokenService.GetOAuthToken(context.Background())
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to fetch service account token")
	}

	resolveToken := token.NewResolve(config.GetAuthURL())
	clusterService, err := cluster.NewService(
		config.GetAuthURL(),
		config.GetClustersRefreshDelay(),
		*saToken,
		resolveToken,
		token.NewGPGDecypter(config.GetTokenKey()),
	)
	if err != nil {
		log.Panic(nil, map[string]interface{}{
			"err": err,
		}, "failed to initialize the cluster.Service component")
	}
	defer clusterService.Stop()

	resolveCluster := cluster.NewResolve(clusterService)
	resolveTenant := func(ctx context.Context, target, userToken string) (user, accessToken string, err error) {
		return resolveToken(ctx, target, userToken, false, token.PlainText) // no need to use "forcePull=true" to validate the user's token on the target.
	}

	openshiftService := openshift.NewService()

	// create user profile client to get the user's cluster
	userService := user.NewService(config.GetAuthURL(), *saToken)

	tenantService := tenant.NewDBService(db)

	// Mount "status" controller
	statusCtrl := controller.NewStatusController(service, db)
	app.MountStatusController(service, statusCtrl)

	// Mount "tenant" controller
	tenantCtrl := controller.NewTenantController(service, tenantService, userService, resolveTenant, resolveCluster, config)
	app.MountTenantController(service, tenantCtrl)

	tenantsCtrl := controller.NewTenantsController(service, tenantService, userService, openshiftService, resolveTenant, resolveCluster)
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
