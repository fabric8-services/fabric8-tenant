package minishift

import (
	"context"
	"crypto/tls"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	"github.com/fabric8-services/fabric8-tenant/test/stub"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestSuite is a base for tests using Minishift and gorm db
type TestSuite struct {
	gormsupport.DBTestSuite
	clusterService      *stub.ClusterService
	authService         *stub.AuthService
	config              *configuration.Data
	toReset             func()
	minishiftUrl        string
	minishiftAdminName  string
	minishiftAdminToken string
	minishiftUserName   string
	minishiftUserToken  string
}

func (s *TestSuite) SetupTest() {
	resource.Require(s.T(), resource.Database, "F8_MINISHIFT")
	s.minishiftUrl = os.Getenv("F8_MINISHIFT_URL")
	s.minishiftAdminName = os.Getenv("F8_MINISHIFT_ADMIN_NAME")
	s.minishiftAdminToken = os.Getenv("F8_MINISHIFT_ADMIN_TOKEN")
	s.minishiftUserName = os.Getenv("F8_MINISHIFT_USER_NAME")
	s.minishiftUserToken = os.Getenv("F8_MINISHIFT_USER_TOKEN")
	s.DBTestSuite.SetupTest()
	s.config, s.toReset = prepareConfig(s.T())

	log.InitializeLogger(s.config.IsLogJSON(), s.config.GetLogLevel())

	s.clusterService = &stub.ClusterService{
		APIURL: s.minishiftUrl,
		User:   s.minishiftAdminName,
		Token:  s.minishiftAdminToken,
	}
	s.authService = &stub.AuthService{
		OpenShiftUsername:  s.minishiftUserName,
		OpenShiftUserToken: s.minishiftUserToken,
	}
}

func (s *TestSuite) TearDownTest() {
	s.DBTestSuite.TearDownTest()
	s.toReset()
}

func (s *TestSuite) GetClusterService() cluster.Service {
	return s.clusterService
}

func (s *TestSuite) GetAuthService(tenantID uuid.UUID) auth.Service {
	s.authService.TenantID = tenantID
	return s.authService
}

func (s *TestSuite) GetConfig() *configuration.Data {
	return s.config
}

func prepareConfig(t *testing.T) (*configuration.Data, func()) {
	resetVars := test.SetEnvironments(
		test.Env("F8_AUTH_TOKEN_KEY", "foo"),
		test.Env("F8_API_SERVER_USE_TLS", "false"),
		test.Env("F8_LOG_LEVEL", "error"),
		test.Env("F8_KEYCLOAK_URL", "http://keycloak.url.com"))
	config, resetConf := test.LoadTestConfig(t)
	reset := func() {
		resetVars()
		resetConf()
	}
	return config, reset
}

func VerifyObjectsPresence(t *testing.T, mappedObjects map[string]environment.Objects, options openshift.ApplyOptions, version string) {
	for ns, objects := range mappedObjects {
		iterations := 0
		for _, obj := range objects {
			for {
				if openshift.IsOfKind(environment.ValKindProjectRequest, environment.ValKindProject, environment.ValKindNamespace)(obj) {
					if environment.GetKind(obj) == environment.ValKindProjectRequest {
						obj["kind"] = environment.ValKindNamespace
					}
					response, err := openshift.Apply(obj, "GET", options)
					if err == nil && environment.HasValidStatus(response) {
						break
					} else if iterations >= 20 {
						assert.NoError(t, err)
						assert.True(t, environment.HasValidStatus(response),
							"The status %s is not valid for namespace %s and object %s of kind %s",
							environment.GetStatus(response), ns, environment.GetName(obj), environment.GetKind(obj))
						break
					}
				} else {
					response, err := openshift.Apply(obj, "GET", options)

					if err == nil && version == environment.GetLabelVersion(response) {
						break
					} else if iterations >= 20 {
						assert.NoError(t, err)
						assert.Equal(t, version, environment.GetLabelVersion(response),
							"the version doesn't match for namespace %s and object %s of kind %s", ns, environment.GetName(obj), environment.GetKind(obj))
						break
					}
				}
				time.Sleep(500 * time.Millisecond)
				iterations++
			}
		}
	}
}

func (s *TestSuite) GetMappedTemplateObjects(nsBaseName string) (map[string]environment.Objects, openshift.ApplyOptions) {
	config := openshift.Config{
		OriginalConfig: s.config,
		MasterURL:      s.minishiftUrl,
		ConsoleURL:     s.minishiftUrl,
		HTTPTransport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		MasterUser: s.minishiftAdminName,
		Token:      s.minishiftAdminToken,
	}

	templs, err := openshift.LoadProcessedTemplates(context.Background(), config, s.minishiftUserName, nsBaseName)
	assert.NoError(s.T(), err)
	mapped, err := openshift.MapByNamespaceAndSort(templs)
	assert.NoError(s.T(), err)
	masterOpts := openshift.ApplyOptions{
		Config:   config,
		Callback: nil,
	}
	return mapped, masterOpts
}
