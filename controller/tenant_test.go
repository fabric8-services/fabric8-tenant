package controller_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/app"
	apptest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/assertion"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/goadesign/goa"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

type TenantControllerTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantController(t *testing.T) {
	suite.Run(t, &TenantControllerTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantControllerTestSuite) TestShowTenant() {
	// given
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		defer gock.OffAll()
		fxt := tf.NewTestFixture(t, s.DB, tf.Tenants(1), tf.Namespaces(1))
		// when
		_, tnnt := apptest.ShowTenantOK(t,
			testdoubles.CreateAndMockUserAndToken(s.T(), fxt.Tenants[0].ID.String(), false), svc, ctrl)
		// then
		assert.Equal(t, fxt.Tenants[0].ID, *tnnt.Data.ID)
		assert.Equal(t, 1, len(tnnt.Data.Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.ShowTenantUnauthorized(t, context.Background(), svc, ctrl)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.OffAll()

			// when/then
			apptest.ShowTenantUnauthorized(t,
				testdoubles.CreateAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.ShowTenantNotFound(t,
				testdoubles.CreateAndMockUserAndToken(t, uuid.NewV4().String(), false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenNoTenantExists() {
	// given
	defer gock.OffAll()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny")
	// when
	apptest.SetupTenantAccepted(s.T(),
		testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	// then
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)

}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenNoTenantExistsInParallelForOneUser() {
	// given
	defer gock.OffAll()

	calls := 0
	service, ctrl, config, reset := s.newTestTenantController()
	defer reset()

	var wg sync.WaitGroup
	wg.Add(100)
	var run sync.WaitGroup
	run.Add(1)

	id := uuid.NewV4()

	deleteCalls := 0
	gock.New("http://api.cluster1").
		Delete("/oapi/v1/projects/.*").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Persist().
		Reply(200)
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny.*")

	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			ctx := testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), false)

			// Setup request context
			req, err := http.NewRequest("POST", "/api/tenant", nil)
			require.NoError(s.T(), err)
			goaCtx := goa.NewContext(goa.WithAction(ctx, "TenantTest"), httptest.NewRecorder(), req, url.Values{})
			setupCtx, err := app.NewSetupTenantContext(goaCtx, req, service)
			require.NoError(s.T(), err)

			run.Wait()

			// when
			err = ctrl.Setup(setupCtx)

			// then
			if err != nil {
				test.AssertError(s.T(), err, test.HasMessageContaining("conflict"))
			}
		}()
	}
	run.Done()
	wg.Wait()

	// then
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	assert.Equal(s.T(), 0, deleteCalls)
	assertion.AssertTenantFromDB(s.T(), s.DB, id).
		HasNsBaseName("johny").
		HasNumberOfNamespaces(5)

}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenTenantExistsInParallelForMultipleUsers() {
	// given
	defer gock.OffAll()

	calls := 0
	service, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	var wg sync.WaitGroup
	wg.Add(100)
	var run sync.WaitGroup
	run.Add(1)

	deleteCalls := 0
	gock.New("http://api.cluster1").
		Delete("/oapi/v1/projects/.*").
		SetMatcher(test.SpyOnCalls(&deleteCalls)).
		Persist().
		Reply(200)

	var tenantIDs []uuid.UUID

	for i := 0; i < 10; i++ {
		userName := fmt.Sprintf("%djohny", i)
		id := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName(userName)), tf.AddNamespaces()).Tenants[0].ID
		tenantIDs = append(tenantIDs, id)
		testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, userName+".*")
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				ctx := testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), false)

				// Setup request context
				req, err := http.NewRequest("POST", "/api/tenant", nil)
				require.NoError(s.T(), err)
				goaCtx := goa.NewContext(goa.WithAction(ctx, "TenantTest"), httptest.NewRecorder(), req, url.Values{})
				setupCtx, err := app.NewSetupTenantContext(goaCtx, req, service)
				require.NoError(s.T(), err)

				run.Wait()

				// when
				ctrl.Setup(setupCtx)

				// then
				if err != nil {
					test.AssertError(s.T(), err, test.HasMessageContaining("conflict"))
				}
			}()
		}
	}
	run.Done()
	wg.Wait()

	// then
	//assert.Equal(s.T(), 10*testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	//assert.Equal(s.T(), 0, deleteCalls)
	assert.Equal(s.T(), 0, calls)
	assert.Equal(s.T(), 0, deleteCalls)
	for index, id := range tenantIDs {
		assertion.AssertTenantFromDB(s.T(), s.DB, id).
			HasNsBaseName(fmt.Sprintf("%djohny", index)).
			HasNumberOfNamespaces(0)
	}
}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenAlreadyExists() {
	// given
	defer gock.OffAll()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddNamespaces(environment.TypeChe))
	id := fxt.Tenants[0].ID
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny1")

	// when
	apptest.SetupTenantConflict(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
	// then
	//totalNumber := testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config)
	//cheObjects := testdoubles.SingleTemplatesObjectsWithDefaults(s.T(), config, environment.TypeChe)
	//numberOfGetChecksForChe := testdoubles.NumberOfGetChecks(cheObjects)
	//assert.Equal(s.T(), totalNumber-(len(cheObjects)+numberOfGetChecksForChe+1), calls)
}

func (s *TenantControllerTestSuite) TestSetupUnauthorizedFailures() {

	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("Unauhorized - no token", func(t *testing.T) {
		defer gock.OffAll()
		// when/then
		apptest.SetupTenantUnauthorized(t, context.Background(), svc, ctrl)
	})

	s.T().Run("Unauhorized - invalid token", func(t *testing.T) {
		// given
		defer gock.OffAll()

		// when/then
		apptest.SetupTenantUnauthorized(t, testdoubles.CreateAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
	})

	s.T().Run("Internal error because of 500 returned from OS", func(t *testing.T) {
		// given
		defer gock.OffAll()
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()
		calls := 0
		gock.New(test.ClusterURL).
			Post(".*/rolebindingrestrictions").
			Reply(500)
		testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny")
		// when
		apptest.SetupTenantInternalServerError(t,
			testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	})
}
func (s *TenantControllerTestSuite) TestSetupConflictFailure() {

	defer gock.OffAll()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	// when/then
	apptest.SetupTenantConflict(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
}

func (s *TenantControllerTestSuite) TestDeleteTenantOK() {
	// given
	defer gock.OffAll()
	repo := tenant.NewDBService(s.DB)

	s.T().Run("with existing namespaces", func(t *testing.T) {
		fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
		id := fxt.Tenants[0].ID
		svc, ctrl, config, reset := s.newTestTenantController()
		defer reset()

		t.Run("only clean namespaces", func(t *testing.T) {
			// given
			defer gock.OffAll()
			calls := 0
			testdoubles.MockCleanRequestsToOS(&calls, test.ClusterURL)
			// when
			apptest.CleanTenantNoContent(s.T(),
				testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl, false)
			// then
			assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenClean(environment.DefaultEnvTypes...), calls)
			assertion.AssertTenantFromService(t, repo, id).
				Exists().
				HasNumberOfNamespaces(5)
		})

		t.Run("remove namespaces and tenant", func(t *testing.T) {
			// given
			defer gock.OffAll()
			calls := 0
			testdoubles.MockRemoveRequestsToOS(&calls, test.ClusterURL)
			// when
			apptest.CleanTenantNoContent(s.T(),
				testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
			// then
			objects := testdoubles.AllDefaultObjects(s.T(), config)
			assert.Equal(s.T(), testdoubles.NumberOfObjectsToRemove(objects), calls)
			assertion.AssertTenantFromService(t, repo, id).
				DoesNotExist().
				HasNoNamespace()
		})
	})

	s.T().Run("remove namespaces and tenant is ok even when one namespace was already removed", func(t *testing.T) {
		// given
		defer gock.OffAll()
		calls := 0
		fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
		id := fxt.Tenants[0].ID
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()
		gock.New(test.ClusterURL).
			Delete("/oapi/v1/projects/johny1-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(404)
		testdoubles.MockRemoveRequestsToOS(&calls, test.ClusterURL)
		// when
		apptest.CleanTenantNoContent(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
		// then
		assertion.AssertTenantFromService(t, repo, id).
			DoesNotExist().
			HasNoNamespace()
	})
}

func (s *TenantControllerTestSuite) TestDeleteTenantFailures() {
	// given
	svc, ctrl, _, reset := s.newTestTenantController()
	repo := tenant.NewDBService(s.DB)
	defer reset()

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.CleanTenantUnauthorized(t, context.Background(), svc, ctrl, false)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.OffAll()

			// when/then
			apptest.CleanTenantUnauthorized(t,
				testdoubles.CreateAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl, false)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.CleanTenantNotFound(t,
				testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl, false)
		})
	})

	s.T().Run("clean tenant fails when one namespace removal fails", func(t *testing.T) {
		// given
		defer gock.OffAll()
		calls := 0
		fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
		id := fxt.Tenants[0].ID
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()

		t.Run("clean tenant fails when one namespace removal fails", func(t *testing.T) {
			// given
			defer gock.OffAll()
			gock.New(test.ClusterURL).
				Delete("/api/v1/namespaces/johny1-jenkins/configmaps").
				Persist().
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			testdoubles.MockCleanRequestsToOS(&calls, test.ClusterURL)
			// when
			apptest.CleanTenantInternalServerError(s.T(),
				testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, false)
			// then
			assertion.AssertTenantFromService(t, repo, id).
				Exists().
				HasNumberOfNamespaces(5)
		})

		t.Run("remove tenant fails when one namespace removal fails", func(t *testing.T) {
			// given
			defer gock.OffAll()
			gock.New(test.ClusterURL).
				Delete("/oapi/v1/projects/johny1-che").
				Times(2).
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			testdoubles.MockRemoveRequestsToOS(&calls, test.ClusterURL)
			// when
			apptest.CleanTenantInternalServerError(s.T(),
				testdoubles.CreateAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
			// then
			assertion.AssertTenantFromService(t, repo, id).
				Exists().
				HasNumberOfNamespaces(1).
				HasNamespaceOfTypeThat(environment.TypeChe).HasName("johny1-che")
		})
	})
}

func (s *TenantControllerTestSuite) TestUpdateTenant() {
	// given
	defer gock.OffAll()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID.String()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		defer gock.OffAll()
		calls := 0
		testdoubles.MockPatchRequestsToOS(&calls, test.ClusterURL)
		// when
		apptest.UpdateTenantAccepted(t, testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl)
		// then
		objects := testdoubles.AllDefaultObjects(t, config)
		// get and patch requests for all objects but ProjectRequest
		assert.Equal(t, (len(objects)-5)*2, calls)
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.UpdateTenantUnauthorized(t, context.Background(), svc, ctrl)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.OffAll()

			// when/then
			apptest.UpdateTenantUnauthorized(t, testdoubles.CreateAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.UpdateTenantNotFound(t, testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("fails when an update of one object fails", func(t *testing.T) {
			// given
			defer gock.OffAll()
			gock.New(test.ClusterURL).
				Patch("/api/v1/namespaces/johny1-jenkins/configmaps").
				Times(2).
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			calls := 0
			testdoubles.MockPatchRequestsToOS(&calls, test.ClusterURL)
			// when/then
			apptest.UpdateTenantInternalServerError(t, testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) newTestTenantController() (*goa.Service, *controller.TenantController, *configuration.Data, func()) {
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	clusterService, authService, config, reset := testdoubles.PrepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, config, reset
}
