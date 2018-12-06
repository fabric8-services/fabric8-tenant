package minishift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	"github.com/fabric8-services/fabric8-tenant/test/stub"
	"github.com/ghodss/yaml"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

// TestSuite is a base for tests using Minishift and gorm db
type TestSuite struct {
	gormsupport.DBTestSuite
	clusterService  *stub.ClusterService
	authService     *stub.AuthService
	Config          *configuration.Data
	toReset         func()
	minishiftConfig *Data
}

func (s *TestSuite) SetupTest() {
	resource.Require(s.T(), resource.Database)

	config, err := NewData()
	require.NoError(s.T(), err)
	if config.GetMinishiftURL() == "" {
		s.T().Skip("Test is skipped because of missing Minishift variables")
	}

	s.minishiftConfig = config

	s.DBTestSuite.SetupTest()
	s.Config, s.toReset = prepareConfig(s.T())

	log.InitializeLogger(s.Config.IsLogJSON(), s.Config.GetLogLevel())

	s.clusterService = &stub.ClusterService{
		APIURL: s.minishiftConfig.GetMinishiftURL(),
		User:   s.minishiftConfig.GetMinishiftAdminName(),
		Token:  s.minishiftConfig.GetMinishiftAdminToken(),
	}
	s.authService = &stub.AuthService{
		OpenShiftUsername:  s.minishiftConfig.GetMinishiftUserName(),
		OpenShiftUserToken: s.minishiftConfig.GetMinishiftUserToken(),
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
	return s.Config
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

func (s *TestSuite) VerifyObjectsPresence(t *testing.T, nsBaseName, version string) {
	minCfg := s.minishiftConfig
	clusterMapping := testdoubles.SingleClusterMapping(minCfg.GetMinishiftURL(), minCfg.GetMinishiftAdminName(), minCfg.GetMinishiftAdminToken())
	userInfo := testdoubles.UserInfo{
		OsUserToken: minCfg.GetMinishiftUserToken(),
		OsUsername:  minCfg.GetMinishiftUserName(),
		NsBaseName:  nsBaseName,
	}
	templatesObjects := testdoubles.AllTemplatesObjects(t, s.Config, clusterMapping, userInfo)

	errorChan := make(chan error, len(templatesObjects))
	defer func() {
		close(errorChan)
		for err := range errorChan {
			assert.NoError(t, err)
		}
	}()

	client := openshift.NewClient(nil, minCfg.GetMinishiftURL(), func(forceMasterToken bool) string {
		return minCfg.GetMinishiftAdminToken()
	})

	var wg sync.WaitGroup
	for _, obj := range templatesObjects {
		wg.Add(1)
		go func(obj environment.Object, ns string) {
			defer wg.Done()
			errorChan <- test.WaitWithTimeout(1 * time.Minute).Until(objectIsUpToDate(obj, ns, *client, version))
		}(obj, environment.GetNamespace(obj))
	}
	wg.Wait()
}

func objectIsUpToDate(obj environment.Object, ns string, client openshift.Client, version string) func() error {
	return func() error {
		shouldVerifyVersion := true
		kind := environment.GetKind(obj)

		if kind == environment.ValKindProjectRequest || kind == environment.ValKindProject || kind == environment.ValKindNamespace {
			shouldVerifyVersion = false
			if kind == environment.ValKindProjectRequest {
				obj["kind"] = environment.ValKindNamespace
			}
		}
		result, err := openshift.Apply(client, "GET", obj)
		if err != nil {
			return err
		}
		var respondedObj environment.Object
		err = yaml.Unmarshal(result.Body, &respondedObj)
		if err != nil {
			return err
		}
		if shouldVerifyVersion {
			if version != environment.GetLabelVersion(respondedObj) {
				return fmt.Errorf("the actual version [%s] doesn't match the expected one [%s] for namespace %s and object %s of kind %s. The response was:\n %s",
					environment.GetLabelVersion(respondedObj), version, ns, environment.GetName(obj), environment.GetKind(obj), respondedObj)
			}
		} else if !environment.HasValidStatus(respondedObj) {
			return fmt.Errorf("the status %s is not valid for namespace %s and object %s of kind %s. The response was:\n %s",
				environment.GetStatus(respondedObj), ns, environment.GetName(obj), environment.GetKind(obj), respondedObj)

		}
		return nil
	}
}

//func (s *TestSuite) GetMappedTemplateObjects(nsBaseName string) (map[string]environment.Objects, openshift.ApplyOptions) {
//	config := openshift.Config{
//		OriginalConfig: s.Config,
//		MasterURL:      s.minishiftConfig.GetMinishiftURL(),
//		ConsoleURL:     s.minishiftConfig.GetMinishiftURL(),
//		HTTPTransport: &http.Transport{
//			TLSClientConfig: &tls.Config{
//				InsecureSkipVerify: true,
//			},
//		},
//		MasterUser: s.minishiftConfig.GetMinishiftAdminName(),
//		Token:      s.minishiftConfig.GetMinishiftAdminToken(),
//	}
//
//	templs, err := openshift.LoadProcessedTemplates(context.Background(), config, s.minishiftConfig.GetMinishiftUserName(), nsBaseName)
//	assert.NoError(s.T(), err)
//	mapped, err := openshift.MapByNamespaceAndSort(templs)
//	assert.NoError(s.T(), err)
//	masterOpts := openshift.ApplyOptions{
//		Config:   config,
//		Callback: nil,
//	}
//	return mapped, masterOpts
//}
