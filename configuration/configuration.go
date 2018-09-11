package configuration

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"

	"encoding/base64"

	"github.com/spf13/viper"
)

const (
	// Constants for viper variable names. Will be used to set
	// default values as well as to get each value

	varPostgresHost                    = "postgres.host"
	varPostgresPort                    = "postgres.port"
	varPostgresUser                    = "postgres.user"
	varPostgresDatabase                = "postgres.database"
	varPostgresPassword                = "postgres.password"
	varPostgresSSLMode                 = "postgres.sslmode"
	varPostgresConnectionTimeout       = "postgres.connection.timeout"
	varPostgresConnectionRetrySleep    = "postgres.connection.retrysleep"
	varPostgresConnectionMaxIdle       = "postgres.connection.maxidle"
	varPostgresConnectionMaxOpen       = "postgres.connection.maxopen"
	varHTTPAddress                     = "http.address"
	varMetricsHTTPAddress              = "metrics.http.address"
	varDeveloperModeEnabled            = "developer.mode.enabled"
	varKeycloakClientID                = "keycloak.client.id"
	varKeycloakRealm                   = "keycloak.realm"
	varKeycloakOpenshiftBroker         = "keycloak.openshift.broker"
	varKeycloakURL                     = "keycloak.url"
	varTogglesURL                      = "toggles.url"
	varConsoleURL                      = "console.url"
	varOpenshiftUseCurrentCluster      = "openshift.use.current.cluster"
	varTemplateJenkinsRootURL          = "template.jenkins.root.url"
	VarTemplateRecommenderExternalName = "template.recommender.external.name"
	VarTemplateRecommenderAPIToken     = "template.recommender.api.token"
	VarTemplateDomain                  = "template.domain"
	varTemplateCheMultiTenantServer    = "template.che.multitenant.server"
	varAPIServerInsecureSkipTLSVerify  = "api.server.insecure.skip.tls.verify"
	varLogLevel                        = "log.level"
	varLogJSON                         = "log.json"

	varAuthURL              = "auth.url"
	varClustersRefreshDelay = "cluster.refresh.delay"
	varAuthClientID         = "service.account.id"
	varClientSecret         = "service.account.secret"
	varAuthTokenKey         = "auth.token.key"
)

// Data encapsulates the Viper configuration object which stores the configuration data in-memory.
type Data struct {
	v *viper.Viper
}

// NewData creates a configuration reader object using a configurable configuration file path
func NewData() (*Data, error) {
	c := Data{
		v: viper.New(),
	}
	c.v.SetEnvPrefix("F8")
	c.v.AutomaticEnv()
	c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.v.SetTypeByDefaultValue(true)
	c.setConfigDefaults()

	return &c, nil
}

// String returns the current configuration as a string
func (c *Data) String() string {
	allSettings := c.v.AllSettings()
	y, err := yaml.Marshal(&allSettings)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"settings": allSettings,
			"err":      err,
		}).Panicln("Failed to marshall config to string")
	}
	return fmt.Sprintf("%s\n", y)
}

// GetData is a wrapper over NewData which reads configuration file path
// from the environment variable.
func GetData() (*Data, error) {
	cd, err := NewData()
	return cd, err
}

func (c *Data) setConfigDefaults() {
	//---------
	// Postgres
	//---------
	c.v.SetTypeByDefaultValue(true)
	c.v.SetDefault(varPostgresHost, "localhost")
	c.v.SetDefault(varPostgresPort, 5432)
	c.v.SetDefault(varPostgresUser, "postgres")
	c.v.SetDefault(varPostgresDatabase, "tenant")
	c.v.SetDefault(varPostgresPassword, "mysecretpassword")
	c.v.SetDefault(varPostgresSSLMode, "disable")
	c.v.SetDefault(varPostgresConnectionTimeout, 5)
	c.v.SetDefault(varPostgresConnectionMaxIdle, -1)
	c.v.SetDefault(varPostgresConnectionMaxOpen, -1)

	// Number of seconds to wait before trying to connect again
	c.v.SetDefault(varPostgresConnectionRetrySleep, time.Duration(time.Second))

	//-----
	// HTTP
	//-----
	c.v.SetDefault(varHTTPAddress, "0.0.0.0:8080")
	c.v.SetDefault(varMetricsHTTPAddress, "0.0.0.0:8080")

	//-----
	// Misc
	//-----
	c.v.SetDefault(varKeycloakOpenshiftBroker, defaultKeycloakOpenshiftBroker)
	c.v.SetDefault(varOpenshiftUseCurrentCluster, false)
	c.v.SetDefault(varAPIServerInsecureSkipTLSVerify, false)
	c.v.SetDefault(varAuthURL, defaultAuthURL)
	c.v.SetDefault(varClustersRefreshDelay, defaultClustersRefreshDelay)
	c.v.SetDefault(varKeycloakClientID, defaultKeycloakClientID)
	c.v.SetDefault(varTogglesURL, defaultTogglesURL)
	c.v.SetDefault(varTemplateCheMultiTenantServer, defaultCheMultiTenantServer)

	// Enable development related features, e.g. token generation endpoint
	c.v.SetDefault(varDeveloperModeEnabled, false)
	c.v.SetDefault(varLogLevel, defaultLogLevel)

	//-----
	// Auth
	// ----
	c.v.SetDefault(varAuthClientID, "c211f1bd-17a7-4f8c-9f80-0917d167889d")
	c.v.SetDefault(varClientSecret, "tenantsecretNew")
}

// GetPostgresHost returns the postgres host as set via default, config file, or environment variable
func (c *Data) GetPostgresHost() string {
	return c.v.GetString(varPostgresHost)
}

// GetPostgresPort returns the postgres port as set via default, config file, or environment variable
func (c *Data) GetPostgresPort() int64 {
	return c.v.GetInt64(varPostgresPort)
}

// GetPostgresUser returns the postgres user as set via default, config file, or environment variable
func (c *Data) GetPostgresUser() string {
	return c.v.GetString(varPostgresUser)
}

// GetPostgresDatabase returns the postgres database as set via default, config file, or environment variable
func (c *Data) GetPostgresDatabase() string {
	return c.v.GetString(varPostgresDatabase)
}

// GetPostgresPassword returns the postgres password as set via default, config file, or environment variable
func (c *Data) GetPostgresPassword() string {
	return c.v.GetString(varPostgresPassword)
}

// GetPostgresSSLMode returns the postgres sslmode as set via default, config file, or environment variable
func (c *Data) GetPostgresSSLMode() string {
	return c.v.GetString(varPostgresSSLMode)
}

// GetPostgresConnectionTimeout returns the postgres connection timeout as set via default, config file, or environment variable
func (c *Data) GetPostgresConnectionTimeout() int64 {
	return c.v.GetInt64(varPostgresConnectionTimeout)
}

// GetPostgresConnectionRetrySleep returns the number of seconds (as set via default, config file, or environment variable)
// to wait before trying to connect again
func (c *Data) GetPostgresConnectionRetrySleep() time.Duration {
	return c.v.GetDuration(varPostgresConnectionRetrySleep)
}

// GetPostgresConnectionMaxIdle returns the number of connections that should be keept alive in the database connection pool at
// any given time. -1 represents no restrictions/default behavior
func (c *Data) GetPostgresConnectionMaxIdle() int {
	return c.v.GetInt(varPostgresConnectionMaxIdle)
}

// GetPostgresConnectionMaxOpen returns the max number of open connections that should be open in the database connection pool.
// -1 represents no restrictions/default behavior
func (c *Data) GetPostgresConnectionMaxOpen() int {
	return c.v.GetInt(varPostgresConnectionMaxOpen)
}

// GetPostgresConfigString returns a ready to use string for usage in sql.Open()
func (c *Data) GetPostgresConfigString() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		c.GetPostgresHost(),
		c.GetPostgresPort(),
		c.GetPostgresUser(),
		c.GetPostgresPassword(),
		c.GetPostgresDatabase(),
		c.GetPostgresSSLMode(),
		c.GetPostgresConnectionTimeout(),
	)
}

// GetHTTPAddress returns the HTTP address (as set via default, config file, or environment variable)
// that the alm server binds to (e.g. "0.0.0.0:8080")
func (c *Data) GetHTTPAddress() string {
	return c.v.GetString(varHTTPAddress)
}

// GetMetricsHTTPAddress returns the address the /metrics endpoing will be mounted.
// By default GetMetricsHTTPAddress is the same as GetHTTPAddress
func (c *Data) GetMetricsHTTPAddress() string {
	return c.v.GetString(varMetricsHTTPAddress)
}

// IsDeveloperModeEnabled returns if development related features (as set via default, config file, or environment variable),
// e.g. token generation endpoint are enabled
func (c *Data) IsDeveloperModeEnabled() bool {
	return c.v.GetBool(varDeveloperModeEnabled)
}

// GetKeycloakClientID returns the keycloak client id (mostly for Che)
func (c *Data) GetKeycloakClientID() string {
	return c.v.GetString(varKeycloakClientID)
}

// GetKeycloakRealm returns the keycloak realm name
func (c *Data) GetKeycloakRealm() string {
	if c.v.IsSet(varKeycloakRealm) {
		return c.v.GetString(varKeycloakRealm)
	}
	if c.IsDeveloperModeEnabled() {
		return devModeKeycloakRealm
	}
	return defaultKeycloakRealm
}

// GetKeycloakOpenshiftBroker returns the keycloak broker name for openshift
func (c *Data) GetKeycloakOpenshiftBroker() string {
	return c.v.GetString(varKeycloakOpenshiftBroker)
}

// GetKeycloakURL returns Keycloak URL used by default in Dev mode
func (c *Data) GetKeycloakURL() string {
	if c.v.IsSet(varKeycloakURL) {
		return c.v.GetString(varKeycloakURL)
	}
	if c.IsDeveloperModeEnabled() {
		return devModeKeycloakURL
	}
	return defaultKeycloakURL
}

// GetAuthGrantType returns the fabric8-auth Grant type used while retrieving
// user account token
func (c *Data) GetAuthGrantType() string {
	return "client_credentials"
}

// GetAuthClientID returns the tenant's client id used while
// communicating with fabric8-auth
func (c *Data) GetAuthClientID() string {
	return c.v.GetString(varAuthClientID)
}

// GetClientSecret returns the secret which will be used in
// conjunction with the tenant client id
func (c *Data) GetClientSecret() string {
	return c.v.GetString(varClientSecret)
}

// GetTokenKey returns the encryption key/passphrase which will be used
// to decrypt the cluster tokens stored in auth token mgm
func (c *Data) GetTokenKey() string {
	return c.v.GetString(varAuthTokenKey)
}

// GetConsoleURL returns the fabric8-ui Console URL
func (c *Data) GetConsoleURL() string {
	if c.v.IsSet(varConsoleURL) {
		return c.v.GetString(varConsoleURL)
	}
	return ""
}

// GetAuthURL returns Auth service URL
func (c *Data) GetAuthURL() string {
	return c.v.GetString(varAuthURL)
}

// GetClustersRefreshDelay returns delay of clusters refresh (in minutes)
func (c *Data) GetClustersRefreshDelay() time.Duration {
	return time.Duration(c.v.GetInt(varClustersRefreshDelay) * int(time.Minute))
}

// GetTogglesURL returns Toggle service URL
func (c *Data) GetTogglesURL() string {
	return c.v.GetString(varTogglesURL)
}

// UseOpenshiftCurrentCluster returns if we should use the current cluster to provision tenant service
func (c *Data) UseOpenshiftCurrentCluster() bool {
	return c.v.GetBool(varOpenshiftUseCurrentCluster)
}

// APIServerInsecureSkipTLSVerify returns if the server's certificate should be checked for validity. This will make your HTTPS connections insecure.
func (c *Data) APIServerInsecureSkipTLSVerify() bool {
	return c.v.GetBool(varAPIServerInsecureSkipTLSVerify)
}

// GetLogLevel returns the loggging level (as set via config file or environment variable)
func (c *Data) GetLogLevel() string {
	return c.v.GetString(varLogLevel)
}

// IsLogJSON returns if we should log json format (as set via config file or environment variable)
func (c *Data) IsLogJSON() bool {
	if c.v.IsSet(varLogJSON) {
		return c.v.GetBool(varLogJSON)
	}
	if c.IsDeveloperModeEnabled() {
		return false
	}
	return true
}

func (c *Data) Set(key string, value interface{}) {
	c.v.Set(key, value)
}

// GetTemplateValues return a Map of additional variables used to process the templates
func (c *Data) GetTemplateValues() (map[string]string, error) {
	if !c.v.IsSet(VarTemplateRecommenderExternalName) {
		return nil, fmt.Errorf("Missing required configuration %v", VarTemplateRecommenderExternalName)
	}
	if !c.v.IsSet(VarTemplateRecommenderAPIToken) {
		return nil, fmt.Errorf("Missing required configuration %v", VarTemplateRecommenderAPIToken)
	}
	if !c.v.IsSet(VarTemplateDomain) {
		return nil, fmt.Errorf("Missing required configuration %v", VarTemplateDomain)
	}

	return map[string]string{
		"RECOMMENDER_EXTERNAL_NAME": c.v.GetString(VarTemplateRecommenderExternalName),
		"RECOMMENDER_API_TOKEN":     base64.StdEncoding.EncodeToString([]byte(c.v.GetString(VarTemplateRecommenderAPIToken))),
		"DOMAIN":                    c.v.GetString(VarTemplateDomain),
		"CHE_KEYCLOAK_AUTH__SERVER__URL": c.GetKeycloakURL() + "/auth",
		"CHE_KEYCLOAK_REALM":             c.GetKeycloakRealm(),
		"CHE_KEYCLOAK_CLIENT__ID":        c.GetKeycloakClientID(),
		"JENKINS_ROOT_URL":               c.v.GetString(varTemplateJenkinsRootURL),
		"CHE_MULTITENANT_SERVER":         c.v.GetString(varTemplateCheMultiTenantServer),
		"OSIO_TOKEN":                     "", // set per request
		"IDENTITY_ID":                    "", // set per request
		"REQUEST_ID":                     "", // set per request
		"JOB_ID":                         "", // set per request
	}, nil
}

const (
	// Auth-related defaults

	defaultKeycloakURL             = "https://sso.prod-preview.openshift.io"
	defaultKeycloakRealm           = "fabric8"
	defaultKeycloakClientID        = "openshiftio-public"
	defaultKeycloakOpenshiftBroker = "openshift-v3"

	// Keycloak vars to be used in dev mode. Can be overridden by setting up keycloak.url & keycloak.realm
	devModeKeycloakURL   = "https://sso.prod-preview.openshift.io"
	devModeKeycloakRealm = "fabric8-test"

	defaultAuthURL              = "https://auth.prod-preview.openshift.io"
	defaultClustersRefreshDelay = 60 // 1 hour
	defaultCheMultiTenantServer = "https://che.prod-preview.openshift.io"

	defaultLogLevel = "info"

	defaultWitURL     = "https://api.prod-preview.openshift.io/api/"
	defaultTogglesURL = "http://f8toggles/api"
)
