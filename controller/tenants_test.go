package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
)

type TenantsControllerTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantsController(t *testing.T) {
	suite.Run(t, &TenantsControllerTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

var resolveCluster = func(ctx context.Context, target string) (cluster.Cluster, error) {
	return cluster.Cluster{
		APIURL:     "https://api.example.com",
		ConsoleURL: "https://console.example.com/console",
		MetricsURL: "https://metrics.example.com",
		LoggingURL: "https://console.example.com/console", // not a typo; logging and console are on the same host
		AppDNS:     "apps.example.com",
		User:       "service-account",
		Token:      "XX",
	}, nil
}

func (s *TenantsControllerTestSuite) TestShowTenants() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("https://api.cluster1")
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(t, s.DB, testfixture.Tenants(1), testfixture.Namespaces(1))
		// when
		_, tenant := test.ShowTenantsOK(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data.ID)
		assert.Equal(t, 1, len(tenant.Data.Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			// when/then
			test.ShowTenantsUnauthorized(t, context.Background(), svc, ctrl, uuid.NewV4())
		})

		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			// when/then
			test.ShowTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, uuid.NewV4())
		})

		t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
			// when/then
			test.ShowTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, uuid.NewV4())
		})

		t.Run("Not found", func(t *testing.T) {
			// when/then
			test.ShowTenantsNotFound(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, uuid.NewV4())
		})
	})
}

func (s *TenantsControllerTestSuite) TestSearchTenants() {

	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("https://api.cluster1")
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(t, s.DB, testfixture.Tenants(1), testfixture.Namespaces(1))
		// when
		_, tenant := test.SearchTenantsOK(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, fxt.Namespaces[0].MasterURL, fxt.Namespaces[0].Name)
		// then
		require.Len(t, tenant.Data, 1)
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data[0].ID)
		assert.Equal(t, 1, len(tenant.Data[0].Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			test.SearchTenantsUnauthorized(t, context.Background(), svc, ctrl, "foo", "bar")
		})

		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			test.SearchTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, "foo", "bar")
		})

		t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
			test.SearchTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, "foo", "bar")
		})

		t.Run("Not found", func(t *testing.T) {
			test.SearchTenantsNotFound(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, "foo", "bar")
		})
	})
}

func (s *TenantsControllerTestSuite) TestFailedDeleteTenants() {
	s.T().Run("Failures", func(t *testing.T) {
		t.Run("Unauhorized failures", func(t *testing.T) {
			defer gock.Off()
			testdoubles.MockCommunicationWithAuth("https://api.cluster1")
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/foo").
				SetMatcher(testsupport.ExpectRequest(testsupport.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/foo-che").
				SetMatcher(testsupport.ExpectRequest(testsupport.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

			svc, ctrl, reset := s.newTestTenantsController()
			defer reset()

			t.Run("Unauhorized - no token", func(t *testing.T) {
				// when/then
				test.DeleteTenantsUnauthorized(t, context.Background(), svc, ctrl, uuid.NewV4())
			})

			t.Run("Unauhorized - no SA token", func(t *testing.T) {
				// when/then
				test.DeleteTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, uuid.NewV4())
			})

			t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
				// when/then
				test.DeleteTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, uuid.NewV4())
			})
		})

		t.Run("namespace deletion failed", func(t *testing.T) {
			// case where the first namespace could not be deleted: the tenant and the namespaces should still be in the DB
			// given
			repo := tenant.NewDBService(s.DB)
			defer gock.Off()
			testdoubles.MockCommunicationWithAuth("https://api.cluster1")
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/baz-che").
				SetMatcher(testsupport.ExpectRequest(testsupport.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/baz").
				SetMatcher(testsupport.ExpectRequest(testsupport.HasJWTWithSub("devtools-sre"))).
				Reply(500).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Internal Server Error"}`)

			svc, ctrl, reset := s.newTestTenantsController()
			defer reset()
			fxt := testfixture.NewTestFixture(t, s.DB, testfixture.Tenants(1, func(fxt *testfixture.TestFixture, idx int) error {
				id, err := uuid.FromString("5a95c51b-120a-4d03-b529-98bd7d4a5689") // force the ID to match the go-vcr cassette in the `delete-tenants.yaml` file
				if err != nil {
					return err
				}
				fxt.Tenants[0].ID = id
				fxt.Tenants[0].OSUsername = "baz"
				fxt.Tenants[0].NsBaseName = "baz"
				return nil
			}), testfixture.Namespaces(2, func(fxt *testfixture.TestFixture, idx int) error {
				fxt.Namespaces[idx].TenantID = fxt.Tenants[0].ID
				fxt.Namespaces[idx].MasterURL = "https://api.cluster1"
				if idx == 0 {
					fxt.Namespaces[idx].Name = "baz"
					fxt.Namespaces[idx].Type = "user"
				} else if idx == 1 {
					fxt.Namespaces[idx].TenantID = fxt.Tenants[0].ID
					fxt.Namespaces[idx].Name = "baz-che"
					fxt.Namespaces[idx].Type = "che"
				}
				return nil
			}))

			// when
			test.DeleteTenantsInternalServerError(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
			// then
			_, err := repo.GetTenant(fxt.Tenants[0].ID)
			require.NoError(t, err)
			namespaces, err := repo.GetNamespaces(fxt.Tenants[0].ID)
			require.NoError(t, err)
			assertContainsNames(t, namespaces, "baz", "baz-che")
		})
	})
}

func assertContainsNames(t *testing.T, slice []*tenant.Namespace, names ...string) {
	assert.Len(t, slice, len(names))
	var sliceNames []string
	for _, ns := range slice {
		sliceNames = append(sliceNames, ns.Name)
	}
	for _, name := range names {
		assert.Contains(t, sliceNames, name)
	}
}

func createValidSAContext(sub string) context.Context {
	claims := jwt.MapClaims{}
	claims["service_accountname"] = sub
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

func createInvalidSAContext() context.Context {
	claims := jwt.MapClaims{}
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

func prepareConfigClusterAndAuthService(t *testing.T) (cluster.Service, auth.Service, *configuration.Data, func()) {
	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	resetVars := testsupport.SetEnvironments(testsupport.Env("F8_AUTH_TOKEN_KEY", "foo"), testsupport.Env("F8_API_SERVER_USE_TLS", "false"))
	authService, _, cleanup :=
		testdoubles.NewAuthServiceWithRecorder(t, "", "http://authservice", saToken.Raw, recorder.WithJWTMatcher)
	config, resetConf := testsupport.LoadTestConfig(t)
	reset := func() {
		resetVars()
		cleanup()
		resetConf()
	}

	clusterService := cluster.NewClusterService(time.Hour, authService)
	err = clusterService.Start()
	require.NoError(t, err)
	return clusterService, authService, config, reset
}
func (s *TenantsControllerTestSuite) newTestTenantsController() (*goa.Service, *controller.TenantsController, func()) {
	clusterService, authService, _, reset := prepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantsController(svc, tenant.NewDBService(s.DB), clusterService, authService)
	return svc, ctrl, reset
}
