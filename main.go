package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/migration"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	witmiddleware "github.com/fabric8-services/fabric8-wit/goamiddleware"
	"github.com/fabric8-services/fabric8-wit/log"
	"github.com/goadesign/goa"
	"github.com/goadesign/goa/middleware"
	"github.com/goadesign/goa/middleware/gzip"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"

	"fmt"

	goalogrus "github.com/goadesign/goa/logging/logrus"
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

	serviceToken := config.GetOpenshiftServiceToken()
	if serviceToken == "" {
		if config.UseOpenshiftCurrentCluster() {
			file, err := ioutil.ReadFile("/run/secrets/kubernetes.io/serviceaccount/token")
			if err != nil {
				logrus.Panic(nil, map[string]interface{}{
					"err": err,
				}, "failed to read service account token")
			}
			serviceToken = strings.TrimSpace(string(file))
		} else {
			logrus.Panic(nil, map[string]interface{}{}, "missing service token")
		}
	}

	var tr *http.Transport
	if config.APIServerInsecureSkipTLSVerify() {
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	if config.GetOpenshiftCheVersion() != "" {
		log.Logger().Infof("Che Version: %s", config.GetOpenshiftCheVersion())
	}
	if config.GetOpenshiftJenkinsVersion() != "" {
		log.Logger().Infof("Jenkins Version: %s", config.GetOpenshiftJenkinsVersion())
	}
	if config.GetOpenshiftTeamVersion() != "" {
		log.Logger().Infof("Team Version: %s", config.GetOpenshiftTeamVersion())
	}
	if config.GetOpenshiftTemplateDir() != "" {
		log.Logger().Infof("Template Dir: %s", config.GetOpenshiftTemplateDir())
	}

	toggles.Init("f8tenant", config.GetTogglesURL())

	openshiftConfig := openshift.Config{
		MasterURL:      config.GetOpenshiftTenantMasterURL(),
		ConsoleURL:     config.GetConsoleURL(),
		Token:          serviceToken,
		HttpTransport:  tr,
		CheVersion:     config.GetOpenshiftCheVersion(),
		JenkinsVersion: config.GetOpenshiftJenkinsVersion(),
		TeamVersion:    config.GetOpenshiftTeamVersion(),
		TemplateDir:    config.GetOpenshiftTemplateDir(),
	}

	openshiftMasterUser, err := openshift.WhoAmI(openshiftConfig)
	if err != nil {
		logrus.Panic(nil, map[string]interface{}{
			"err": err,
		}, "unknown master user based on service token")
	}
	openshiftConfig.MasterUser = openshiftMasterUser

	keycloakConfig := keycloak.Config{
		BaseURL: config.GetKeycloakURL(),
		Realm:   config.GetKeycloakRealm(),
		Broker:  config.GetKeycloakOpenshiftBroker(),
	}

	templateVars, err := config.GetTemplateValues()
	if err != nil {
		panic(err)
	}

	templateVars["KEYCLOAK_URL"] = ""
	templateVars["FABRIC8_CONSOLE_URL"] = openshiftConfig.ConsoleURL
	templateVars["KEYCLOAK_OSO_ENDPOINT"] = keycloakConfig.CustomBrokerTokenURL("openshift-v3")
	templateVars["KEYCLOAK_GITHUB_ENDPOINT"] = fmt.Sprintf("%s%s?for=https://github.com", config.GetAuthURL(), auth.RetrieveTokenPath())

	publicKeys, err := keycloak.GetPublicKeys(config.GetAuthURL())
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

	// Mount "status" controller
	statusCtrl := controller.NewStatusController(service, db)
	app.MountStatusController(service, statusCtrl)

	// Mount "tenant" controller
	witURL := config.GetWitURL()
	tenantService := tenant.NewDBService(db)

	tenantCtrl := controller.NewTenantController(service, tenantService, keycloakConfig, openshiftConfig, templateVars, witURL)
	app.MountTenantController(service, tenantCtrl)

	tenantsCtrl := controller.NewTenantsController(service, tenantService)
	app.MountTenantsController(service, tenantsCtrl)

	// Mount "tenantkube" controller
	tenanKubetCtrl := controller.NewTenantKubeController(service, tenantService, keycloakConfig, openshiftConfig, templateVars)
	app.MountTenantKubeController(service, tenanKubetCtrl)

	// Mount "auth" controller
	authCtrl := controller.NewAuthController(service, tenantService, keycloakConfig, openshiftConfig, templateVars)
	app.MountAuthController(service, authCtrl)

	log.Logger().Infoln("Git Commit SHA: ", controller.Commit)
	log.Logger().Infoln("UTC Build Time: ", controller.BuildTime)
	log.Logger().Infoln("UTC Start Time: ", controller.StartTime)
	log.Logger().Infoln("Dev mode:       ", config.IsDeveloperModeEnabled())
	log.Logger().Infoln("WIT URL:        ", witURL)

	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.Handle("/", service.Mux)

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
