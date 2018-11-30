package minishift

import (
	"context"
	"crypto/tls"
	"fmt"
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
	"github.com/stretchr/testify/require"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestSuite is a base for tests using Minishift and gorm db
type TestSuite struct {
	gormsupport.DBTestSuite
	ClusterService  *stub.ClusterService
	AuthService     *stub.AuthService
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

	s.ClusterService = &stub.ClusterService{
		APIURL: s.minishiftConfig.GetMinishiftURL(),
		User:   s.minishiftConfig.GetMinishiftAdminName(),
		Token:  s.minishiftConfig.GetMinishiftAdminToken(),
	}
	s.AuthService = &stub.AuthService{
		OpenShiftUsername:  s.minishiftConfig.GetMinishiftUserName(),
		OpenShiftUserToken: s.minishiftConfig.GetMinishiftUserToken(),
	}
}

func (s *TestSuite) TearDownTest() {
	s.DBTestSuite.TearDownTest()
	s.toReset()
}

func (s *TestSuite) GetClusterService() cluster.Service {
	return s.ClusterService
}

func (s *TestSuite) GetAuthService(tenantID uuid.UUID) auth.Service {
	s.AuthService.TenantID = tenantID
	return s.AuthService
}

func (s *TestSuite) GetConfig() *configuration.Data {
	return s.Config
}

func prepareConfig(t *testing.T) (*configuration.Data, func()) {
	resetVars := test.SetEnvironments(
		test.Env("F8_AUTH_TOKEN_KEY", "foo"),
		test.Env("F8_API_SERVER_USE_TLS", "false"),
		test.Env("F8_LOG_LEVEL", "error"),
		test.Env("F8_KEYCLOAK_URL", "http://keycloak.url.com"),
		test.Env("DISABLE_OSO_QUOTAS", "true"))
	config, resetConf := test.LoadTestConfig(t)
	reset := func() {
		resetVars()
		resetConf()
	}
	return config, reset
}

func VerifyObjectsPresence(t *testing.T, mappedObjects map[string]environment.Objects, options openshift.ApplyOptions, version string, required bool) {
	size := 0
	for _, objects := range mappedObjects {
		size += len(objects)
	}
	errorChan := make(chan error, size)
	defer func() {
		close(errorChan)
		errWasFound := false
		for err := range errorChan {
			assert.NoError(t, err)
			errWasFound = errWasFound || err != nil
		}
		if required {
			require.False(t, errWasFound)
		}
	}()

	var wg sync.WaitGroup
	for ns, objects := range mappedObjects {
		for _, obj := range objects {
			wg.Add(1)
			go func(obj environment.Object, ns string) {
				defer wg.Done()
				errorChan <- test.WaitWithTimeout(1 * time.Minute).Until(objectIsUpToDate(obj, ns, options, version))
			}(obj, ns)
		}
	}
	wg.Wait()
}

func objectIsUpToDate(obj environment.Object, ns string, options openshift.ApplyOptions, version string) func() error {
	return func() error {
		shouldVerifyVersion := true

		if openshift.IsOfKind(environment.ValKindProjectRequest, environment.ValKindProject, environment.ValKindNamespace)(obj) {
			shouldVerifyVersion = false
			if environment.GetKind(obj) == environment.ValKindProjectRequest {
				obj["kind"] = environment.ValKindNamespace
			}
		}
		response, err := openshift.Apply(obj, "GET", options)
		if err != nil {
			return err
		}
		if shouldVerifyVersion {
			if version != environment.GetLabelVersion(response) {
				return fmt.Errorf("the actual version [%s] doesn't match the expected one [%s] for namespace %s and object %s of kind %s. The response was:\n %s",
					environment.GetLabelVersion(response), version, ns, environment.GetName(obj), environment.GetKind(obj), response)
			}
		} else if !environment.HasValidStatus(response) {
			return fmt.Errorf("the status %s is not valid for namespace %s and object %s of kind %s. The response was:\n %s",
				environment.GetStatus(response), ns, environment.GetName(obj), environment.GetKind(obj), response)

		}
		return nil
	}
}

func (s *TestSuite) GetMappedTemplateObjects(nsBaseName string) (map[string]environment.Objects, openshift.ApplyOptions) {
	config := openshift.Config{
		OriginalConfig: s.Config,
		MasterURL:      s.minishiftConfig.GetMinishiftURL(),
		ConsoleURL:     s.minishiftConfig.GetMinishiftURL(),
		HTTPTransport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		MasterUser: s.minishiftConfig.GetMinishiftAdminName(),
		Token:      s.minishiftConfig.GetMinishiftAdminToken(),
	}

	templs, _, err :=
		openshift.LoadProcessedTemplates(context.Background(), config, s.minishiftConfig.GetMinishiftUserName(), nsBaseName, environment.DefaultEnvTypes)
	assert.NoError(s.T(), err)
	mapped, err := openshift.MapByNamespaceAndSort(templs)
	assert.NoError(s.T(), err)
	masterOpts := openshift.ApplyOptions{
		Config:   config,
		Callback: nil,
	}
	return mapped, masterOpts
}
