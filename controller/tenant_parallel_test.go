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

type TenantControllerParallelTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantControllerInParallel(t *testing.T) {
	suite.Run(t, &TenantControllerParallelTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

type whenInParallel func(ctx context.Context, service *goa.Service, ctrl app.TenantController)

func (s *TenantControllerParallelTestSuite) TestShowTenantOKWhenNoTenantExistsInParallelForOneUser() {
	s.verifyTenantCreationWhenNoTenantExistsInParallelForOneUser(
		func(ctx context.Context, service *goa.Service, ctrl app.TenantController) {
			// when
			apptest.ShowTenantOK(s.T(), ctx, service, ctrl)
		})
}

func (s *TenantControllerParallelTestSuite) TestSetupTenantOKWhenNoTenantExistsInParallelForOneUser() {
	s.verifyTenantCreationWhenNoTenantExistsInParallelForOneUser(
		// when
		s.newSetupTenantParallelWhenAction())
}

func (s *TenantControllerParallelTestSuite) newSetupTenantParallelWhenAction() whenInParallel {
	return func(ctx context.Context, service *goa.Service, ctrl app.TenantController) {
		// Setup request context
		req, err := http.NewRequest("POST", "/api/tenant", nil)
		require.NoError(s.T(), err)
		goaCtx := goa.NewContext(goa.WithAction(ctx, "TenantTest"), httptest.NewRecorder(), req, url.Values{})
		setupCtx, err := app.NewSetupTenantContext(goaCtx, req, service)
		require.NoError(s.T(), err)
		err = ctrl.Setup(setupCtx)
		require.NoError(s.T(), err)
	}
}

func (s *TenantControllerParallelTestSuite) verifyTenantCreationWhenNoTenantExistsInParallelForOneUser(when whenInParallel) {
	// given
	defer gock.OffAll()

	calls := 0
	service, ctrl, config, reset := s.newTestTenantController()
	defer reset()

	var wg sync.WaitGroup
	wg.Add(100)
	var run sync.WaitGroup
	run.Add(1)

	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddNamespaces())
	id := fxt.Tenants[0].ID

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
			ctx := createAndMockUserAndToken(s.T(), id.String(), false)

			run.Wait()
			// when
			when(ctx, service, ctrl)
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

func (s *TenantControllerParallelTestSuite) TestShowTenantOKWhenNoTenantExistsInParallelForMultipleUsers() {
	s.verifyTenantCreationWhenNoTenantExistsInParallelForOneUser(
		func(ctx context.Context, service *goa.Service, ctrl app.TenantController) {
			// when
			apptest.ShowTenantOK(s.T(), ctx, service, ctrl)
		})
}

func (s *TenantControllerParallelTestSuite) TestSetupTenantOKWhenNoTenantExistsInParallelForMultipleUsers() {
	s.verifyTenantCreationWhenNoTenantExistsInParallelForOneUser(
		// when
		s.newSetupTenantParallelWhenAction())
}

func (s *TenantControllerParallelTestSuite) verifyTenantCreationWhenNoTenantExistsInParallelForMultipleUsers(when whenInParallel) {
	// given
	defer gock.OffAll()

	calls := 0
	service, ctrl, config, reset := s.newTestTenantController()
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
				ctx := createAndMockUserAndToken(s.T(), id.String(), false)

				run.Wait()
				// when
				when(ctx, service, ctrl)
			}()
		}
	}
	run.Done()
	wg.Wait()

	// then
	assert.Equal(s.T(), 10*testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	assert.Equal(s.T(), 0, deleteCalls)
	for index, id := range tenantIDs {
		assertion.AssertTenantFromDB(s.T(), s.DB, id).
			HasNsBaseName(fmt.Sprintf("%djohny", index)).
			HasNumberOfNamespaces(5)
	}
}

func (s *TenantControllerParallelTestSuite) newTestTenantController() (*goa.Service, *controller.TenantController, *configuration.Data, func()) {
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	clusterService, authService, config, reset := prepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, config, reset
}
