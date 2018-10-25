package controller_test

import (
	"context"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	goatest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-wit/errors"
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

	gockMocks()
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(t, s.Repo, testfixture.Tenants(1), testfixture.Namespaces(1))
		// when
		_, tenant := goatest.ShowTenantsOK(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data.ID)
		assert.Equal(t, 1, len(tenant.Data.Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			// when/then
			goatest.ShowTenantsUnauthorized(t, context.Background(), svc, ctrl, uuid.NewV4())
		})

		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			// when/then
			goatest.ShowTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, uuid.NewV4())
		})

		t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
			// when/then
			goatest.ShowTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, uuid.NewV4())
		})

		t.Run("Not found", func(t *testing.T) {
			// when/then
			goatest.ShowTenantsNotFound(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, uuid.NewV4())
		})
	})
}

func (s *TenantsControllerTestSuite) TestSearchTenants() {

	// given
	defer gock.Off()
	gockMocks()
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := testfixture.NewTestFixture(t, s.Repo, testfixture.Tenants(1), testfixture.Namespaces(1))
		// when
		_, tenant := goatest.SearchTenantsOK(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, fxt.Namespaces[0].MasterURL, fxt.Namespaces[0].Name)
		// then
		require.Len(t, tenant.Data, 1)
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data[0].ID)
		assert.Equal(t, 1, len(tenant.Data[0].Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			goatest.SearchTenantsUnauthorized(t, context.Background(), svc, ctrl, "foo", "bar")
		})

		t.Run("Unauhorized - no SA token", func(t *testing.T) {
			goatest.SearchTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, "foo", "bar")
		})

		t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
			goatest.SearchTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, "foo", "bar")
		})

		t.Run("Not found", func(t *testing.T) {
			goatest.SearchTenantsNotFound(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, "foo", "bar")
		})
	})
}

func (s *TenantsControllerTestSuite) TestSuccessfullyDeleteTenants() {
	s.T().Run("delete method", func(t *testing.T) {
		cl := client.New(nil)
		req, err := cl.NewDeleteTenantsRequest(context.Background(), "")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "DELETE", req.Method)
	})

	s.T().Run("all ok", func(t *testing.T) {
		// given
		defer gock.Off()
		gockMocks()
		gock.New("https://api.cluster1").
			Delete("/oapi/v1/projects/foo").
			Persist().
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		gock.New("https://api.cluster1").
			Delete("/oapi/v1/projects/foo-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		fxt := testfixture.NewTestFixture(t, s.Repo, testfixture.Tenants(1, func(fxt *testfixture.TestFixture, idx int) error {
			id, err := uuid.FromString("8c97b9fc-2a3f-4bef-8579-75e676ab1348") // force the ID to match the go-vcr cassette in the `delete-tenants.yaml` file
			if err != nil {
				return err
			}
			fxt.Tenants[0].ID = id
			fxt.Tenants[0].OSUsername = "foo"
			return nil
		}), testfixture.Namespaces(2, func(fxt *testfixture.TestFixture, idx int) error {
			fxt.Namespaces[idx].TenantID = fxt.Tenants[0].ID
			fxt.Namespaces[idx].MasterURL = "https://api.cluster1"
			if idx == 0 {
				fxt.Namespaces[idx].Name = "foo"
				fxt.Namespaces[idx].Type = "user"
			} else if idx == 1 {
				fxt.Namespaces[idx].Name = "foo-che"
				fxt.Namespaces[idx].Type = "che"
			}
			return nil
		}))

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		_, err := s.Repo.GetTenant(fxt.Tenants[0].ID)
		require.IsType(t, errors.NotFoundError{}, err)
		namespaces, err := s.Repo.GetNamespaces(fxt.Tenants[0].ID)
		require.NoError(t, err)
		assert.Empty(t, namespaces)
	})

	s.T().Run("ok even if namespace missing", func(t *testing.T) {
		// if the namespace record exist in the DB, but the `delete namespace` call on the cluster endpoint fails with a 404
		// given
		defer gock.Off()
		gockMocks()
		gock.New("https://api.cluster1").
			Delete("/oapi/v1/projects/bar").
			Persist().
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
		gock.New("https://api.cluster1").
			Delete("/oapi/v1/projects/bar-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(403).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Not Found"}`)

		fxt := testfixture.NewTestFixture(t, s.Repo, testfixture.Tenants(1, func(fxt *testfixture.TestFixture, idx int) error {
			id, err := uuid.FromString("0257147d-0bb8-4624-a054-853e49c97d07") // force the ID to match the go-vcr cassette in the `delete-tenants.yaml` file
			if err != nil {
				return err
			}
			fxt.Tenants[0].ID = id
			fxt.Tenants[0].OSUsername = "bar"
			return nil
		}), testfixture.Namespaces(2, func(fxt *testfixture.TestFixture, idx int) error {
			fxt.Namespaces[idx].TenantID = fxt.Tenants[0].ID
			fxt.Namespaces[idx].MasterURL = "https://api.cluster1"
			if idx == 0 {
				fxt.Namespaces[idx].Name = "bar"
				fxt.Namespaces[idx].Type = "user"
			} else if idx == 1 {
				fxt.Namespaces[idx].Name = "bar-che"
				fxt.Namespaces[idx].Type = "che"
			}
			return nil
		}))

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		_, err := s.Repo.GetTenant(fxt.Tenants[0].ID)
		require.IsType(t, errors.NotFoundError{}, err)
		namespaces, err := s.Repo.GetNamespaces(fxt.Tenants[0].ID)
		require.NoError(t, err)
		assert.Empty(t, namespaces)
	})

}

func (s *TenantsControllerTestSuite) TestFailedDeleteTenants() {
	s.T().Run("Failures", func(t *testing.T) {
		t.Run("Unauhorized failures", func(t *testing.T) {
			defer gock.Off()
			gockMocks()
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/foo").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/foo-che").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

			svc, ctrl, reset := s.newTestTenantsController()
			defer reset()

			t.Run("Unauhorized - no token", func(t *testing.T) {
				// when/then
				goatest.DeleteTenantsUnauthorized(t, context.Background(), svc, ctrl, uuid.NewV4())
			})

			t.Run("Unauhorized - no SA token", func(t *testing.T) {
				// when/then
				goatest.DeleteTenantsUnauthorized(t, createInvalidSAContext(), svc, ctrl, uuid.NewV4())
			})

			t.Run("Unauhorized - wrong SA token", func(t *testing.T) {
				// when/then
				goatest.DeleteTenantsUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, uuid.NewV4())
			})
		})

		t.Run("namespace deletion failed", func(t *testing.T) {
			// case where the first namespace could not be deleted: the tenant and the namespaces should still be in the DB
			// given
			defer gock.Off()
			gockMocks()
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/baz").
				Persist().
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Internal Server Error"}`)
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/baz-che").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

			svc, ctrl, reset := s.newTestTenantsController()
			defer reset()
			fxt := testfixture.NewTestFixture(t, s.Repo, testfixture.Tenants(1, func(fxt *testfixture.TestFixture, idx int) error {
				id, err := uuid.FromString("5a95c51b-120a-4d03-b529-98bd7d4a5689") // force the ID to match the go-vcr cassette in the `delete-tenants.yaml` file
				if err != nil {
					return err
				}
				fxt.Tenants[0].ID = id
				fxt.Tenants[0].OSUsername = "baz"
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
			goatest.DeleteTenantsInternalServerError(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
			// then
			_, err := s.Repo.GetTenant(fxt.Tenants[0].ID)
			require.NoError(t, err)
			namespaces, err := s.Repo.GetNamespaces(fxt.Tenants[0].ID)
			require.NoError(t, err)
			require.Len(t, namespaces, 2)
			// firs namespace could not be deleted, both still exist in the DB (and in the cluster)
			assertContainsNs(t, namespaces, "baz")
			assertContainsNs(t, namespaces, "baz-che")
		})
	})
}

func assertContainsNs(t *testing.T, slice []*tenant.Namespace, name string) {
	for _, ns := range slice {
		if ns.Name == name {
			return
		}
	}
	assert.Fail(t, "The slice %s should contain a namespace with a name %s", slice, name)
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

func (s *TenantsControllerTestSuite) newTestTenantsController() (*goa.Service, *controller.TenantsController, func()) {
	resetVars := test.SetEnvironments(test.Env("F8_AUTH_TOKEN_KEY", "foo"), test.Env("F8_API_SERVER_USE_TLS", "false"))
	authService, _, cleanup := testdoubles.NewAuthServiceWithRecorder(s.T(), "", "http://authservice", recorder.WithJWTMatcher)
	config, resetConf := test.LoadTestConfig(s.T())
	reset := func() {
		resetVars()
		cleanup()
		resetConf()
	}

	saToken, err := test.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(s.T(), err)
	authService.SaToken = saToken.Raw

	clusterService := cluster.NewClusterService(time.Hour, authService)
	err = clusterService.Start()
	require.NoError(s.T(), err)

	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantsController(svc, s.Repo, clusterService, authService, config)
	return svc, ctrl, reset
}

func gockMocks() {
	gock.New("http://authservice").
		Get("/api/clusters/").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
		Reply(200).
		BodyString(`{
      "data":[
        {
          "name": "cluster_name",
          "api-url": "https://api.cluster1/",
          "console-url": "http://console.cluster1/",
          "metrics-url": "http://metrics.cluster1/",
          "logging-url": "http://logs.cluster1/",
          "app-dns": "foo"
        }
      ]
    }`)

	gock.New("http://authservice").
		Get("/api/token").
		MatchParam("for", "https://api.cluster1").
		MatchParam("force_pull", "false").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
		Persist().
		Reply(200).
		BodyString(`{ 
      "token_type": "bearer",
      "username": "devtools-sre",
      "access_token": "jA0ECQMCWbHrs0GtZQlg0sDQAYMwVoNofrjMocCLv5+FR4GkCPEOiKvK6ifRVsZ6VWLcBVF5k/MFO0Y3EmE8O77xDFRvA9AVPETb7M873tGXMEmqFjgpWvppN81zgmk/enaeJbTBeYhXScyShw7G7kIbgaRy2ufPzVj7f2muM0PHRS334xOVtWZIuaq4lP7EZvW4u0JinSVT0oIHBoCKDFlMlNS1sTygewyI3QOX1quLEEhaDr6/eTG66aTfqMYZQpM4B+m78mi02GLPx3Z24DpjzgshagmGQ8f2kj49QA0LbbFaCUvpqlyStkXNwFm7z+Vuefpp+XYGbD+8MfOKsQxDr7S6ziEdjs+zt/QAr1ZZyoPsC4TaE6kkY1JHIIcrdO5YoX6mbxDMdkLY1ybMN+qMNKtVW4eV9eh34fZKUJ6sjTfdaZ8DjN+rGDKMtZDqwa1h+YYz938jl/bRBEQjK479o7Y6Iu/v4Rwn4YjM4YGjlXs/T/rUO1uye3AWmVNFfi6GtqNpbsKEbkr80WKOOWiSuYeZHbXA7pWMit17U9LtUA=="
    }`)

	gock.New("https://api.cluster1").
		Get("/apis/user.openshift.io/v1/users/~").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
		Reply(200).
		BodyString(`{
      "kind":"User",
      "apiVersion":"user.openshift.io/v1",
      "metadata":{
        "name":"devtools-sre",
      },
      "identities":[],
      "groups":[]
    }`)
}
