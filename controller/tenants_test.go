package controller_test

import (
	"context"
	"github.com/dgrijalva/jwt-go"
	goatest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/assertion"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"testing"
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
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := tf.NewTestFixture(t, s.DB, tf.Tenants(1), tf.Namespaces(1))
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
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		fxt := tf.NewTestFixture(t, s.DB, tf.Tenants(1), tf.Namespaces(1))
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
	repo := tenant.NewDBService(s.DB)

	s.T().Run("delete method", func(t *testing.T) {
		cl := client.New(nil)
		req, err := cl.NewDeleteTenantsRequest(context.Background(), "")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "DELETE", req.Method)
	})

	s.T().Run("all ok", func(t *testing.T) {
		// given
		defer gock.OffAll()
		testdoubles.MockCommunicationWithAuth(test.ClusterURL)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/foo-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/foo").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		fxt := tf.FillDB(t, s.DB, tf.AddSpecificTenants(tf.SingleWithName("foo")), tf.AddNamespaces(environment.TypeUser, environment.TypeChe))

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		assertion.AssertTenantFromService(t, repo, fxt.Tenants[0].ID).
			DoesNotExist().
			HasNoNamespace()
	})

	s.T().Run("ok even if namespace missing while returning 404", func(t *testing.T) {
		// if the namespace record exist in the DB, but the `delete namespace` call on the cluster endpoint fails with a 404
		// given
		defer gock.OffAll()
		testdoubles.MockCommunicationWithAuth(test.ClusterURL)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/bar-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(404).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Not Found"}`)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/bar").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		fxt := tf.FillDB(t, s.DB, tf.AddSpecificTenants(tf.SingleWithName("bar")), tf.AddNamespaces(environment.TypeUser, environment.TypeChe))
		id := fxt.Tenants[0].ID

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, id)
		// then
		assertion.AssertTenantFromService(t, repo, id).
			DoesNotExist().
			HasNoNamespace()
	})

	s.T().Run("ok even if namespace missing while returning 403", func(t *testing.T) {
		// if the namespace record exist in the DB, but the `delete namespace` call on the cluster endpoint fails with a 403
		// given
		defer gock.OffAll()
		testdoubles.MockCommunicationWithAuth(test.ClusterURL)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/bar-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(403).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Forbidden"}`)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/bar").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		fxt := tf.FillDB(t, s.DB, tf.AddSpecificTenants(tf.SingleWithName("bar")), tf.AddNamespaces(environment.TypeUser, environment.TypeChe))
		id := fxt.Tenants[0].ID

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, id)
		// then
		assertion.AssertTenantFromService(t, repo, id).
			DoesNotExist().
			HasNoNamespace()
	})

	s.T().Run("ok when unsupported namespaces exist in DB", func(t *testing.T) {
		// given
		defer gock.OffAll()
		testdoubles.MockCommunicationWithAuth(test.ClusterURL)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/foo-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/foo").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(200).
			BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)

		fxt := tf.FillDB(t, s.DB, tf.AddSpecificTenants(tf.SingleWithName("foo")),
			tf.AddNamespaces(environment.TypeUser, environment.TypeChe, environment.Type("stage"),
				environment.Type("run"), environment.Type("jenkins")))

		svc, ctrl, reset := s.newTestTenantsController()
		defer reset()
		// when
		goatest.DeleteTenantsNoContent(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
		// then
		assertion.AssertTenantFromService(t, repo, fxt.Tenants[0].ID).
			DoesNotExist().
			HasNoNamespace()
	})
}

func (s *TenantsControllerTestSuite) TestFailedDeleteTenants() {
	s.T().Run("Failures", func(t *testing.T) {
		t.Run("Unauhorized failures", func(t *testing.T) {
			defer gock.OffAll()
			testdoubles.MockCommunicationWithAuth(test.ClusterURL)
			gock.New(test.ClusterURL).
				Delete("/oapi/v1/projects/foo").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
			gock.New(test.ClusterURL).
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
			// case where the first namespace could not be deleted: the tenant and the namespace that failed should still be in the DB - the rest should be deleted
			// given
			repo := tenant.NewDBService(s.DB)
			defer gock.OffAll()
			testdoubles.MockCommunicationWithAuth(test.ClusterURL)
			gock.New(test.ClusterURL).
				Delete("/oapi/v1/projects/baz-che").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(200).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Success"}`)
			gock.New(test.ClusterURL).
				Delete("/oapi/v1/projects/baz").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Times(2).
				Reply(500).
				BodyString(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Internal Server Error"}`)

			svc, ctrl, reset := s.newTestTenantsController()
			defer reset()
			fxt := tf.FillDB(t, s.DB, tf.AddSpecificTenants(tf.SingleWithName("baz")),
				tf.AddNamespaces(environment.TypeUser, environment.TypeChe))

			// when
			goatest.DeleteTenantsInternalServerError(t, createValidSAContext("fabric8-auth"), svc, ctrl, fxt.Tenants[0].ID)
			// then
			assertion.AssertTenantFromService(t, repo, fxt.Tenants[0].ID).
				Exists().
				HasNumberOfNamespaces(1).
				HasNamespaceOfTypeThat(environment.TypeUser).HasName("baz")
		})
	})
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
	clusterService, authService, config, reset := testdoubles.PrepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantsController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, reset
}
