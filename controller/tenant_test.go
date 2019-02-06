package controller_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/app"
	apptest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/auth"
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
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
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
		fxt := tf.FillDB(t, s.DB, tf.AddTenants(10), tf.AddDefaultNamespaces())
		// when
		_, tnnt := apptest.ShowTenantOK(t, createAndMockUserAndToken(s.T(), fxt.Tenants[0].ID.String(), false), svc, ctrl)
		// then
		assert.Equal(t, fxt.Tenants[0].ID, *tnnt.Data.ID)
		assert.Len(t, tnnt.Data.Attributes.Namespaces, 5)
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
			apptest.ShowTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) TestShowTenantOKWhenNoTenantExists() {
	// given
	defer gock.OffAll()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny")
	// when
	_, tnnt := apptest.ShowTenantOK(s.T(), createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	// then
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
	assertion.AssertTenantFromDB(s.T(), s.DB, *tnnt.Data.ID).
		Exists().
		HasNsBaseName("johny").
		HasNumberOfNamespaces(5)
	assert.Equal(s.T(), 5, len(tnnt.Data.Attributes.Namespaces))
}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenNoTenantExists() {
	// given
	defer gock.OffAll()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny")
	// when
	apptest.SetupTenantAccepted(s.T(), createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	// then
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)
}

func (s *TenantControllerTestSuite) TestShowTenantWhenSomeNamespacesAreMissing() {
	var tnnt *app.TenantSingle
	s.verifyTenantCreationWhenAlreadyExists(
		// when
		func(ctx context.Context, service *goa.Service, ctrl app.TenantController) {
			_, singleTnnt := apptest.ShowTenantOK(s.T(), ctx, service, ctrl)
			tnnt = singleTnnt
		},
		// then
		func() {
			assertion.AssertTenantFromDB(s.T(), s.DB, *tnnt.Data.ID).
				Exists().
				HasNumberOfNamespaces(5)
			namespaceAttributes := tnnt.Data.Attributes.Namespaces
			assert.Equal(s.T(), 5, len(namespaceAttributes))
			for _, ns := range namespaceAttributes {
				assert.Equal(s.T(), test.Normalize(test.ClusterURL), *ns.ClusterURL)
				assert.Equal(s.T(), tenant.Ready.String(), *ns.State)
				assert.Equal(s.T(), tenant.ConstructNamespaceName(environment.Type(*ns.Type), "johny1"), *ns.Name)
			}
		})
}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenAlreadyExists() {
	s.verifyTenantCreationWhenAlreadyExists(
		// when
		func(ctx context.Context, service *goa.Service, ctrl app.TenantController) {
			apptest.SetupTenantAccepted(s.T(), ctx, service, ctrl)
		},
		// then just verify the calls
		func() {})
}

func (s *TenantControllerTestSuite) verifyTenantCreationWhenAlreadyExists(
	when func(ctx context.Context, service *goa.Service, ctrl app.TenantController),
	then func()) {
	// given
	defer gock.OffAll()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddNamespaces(environment.TypeChe))
	id := fxt.Tenants[0].ID
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, test.ClusterURL, environment.DefaultEnvTypes, "johny1")

	// when
	when(createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
	// then
	totalNumber := testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config)
	cheObjects := testdoubles.SingleTemplatesObjectsWithDefaults(s.T(), config, environment.TypeChe)
	numberOfGetChecksForChe := testdoubles.NumberOfGetChecks(cheObjects)
	assert.Equal(s.T(), totalNumber-(len(cheObjects)+numberOfGetChecksForChe+1), calls)
	then()
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
		apptest.SetupTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
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
		apptest.SetupTenantInternalServerError(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	})
}
func (s *TenantControllerTestSuite) TestSetupConflictFailure() {

	defer gock.OffAll()
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithNames("johny", "johny1")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	// when/then
	apptest.SetupTenantConflict(s.T(), createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
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
			apptest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl, false)
			// then
			assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenClean(t, config, environment.DefaultEnvTypes...), calls)
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
			apptest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
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
		apptest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
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
			apptest.CleanTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl, false)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.CleanTenantNotFound(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl, false)
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
			apptest.CleanTenantInternalServerError(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, false)
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
			apptest.CleanTenantInternalServerError(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
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
		apptest.UpdateTenantAccepted(t, createAndMockUserAndToken(s.T(), id, false), svc, ctrl)
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
			apptest.UpdateTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.OffAll()
			// when/then
			apptest.UpdateTenantNotFound(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
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
			apptest.UpdateTenantInternalServerError(t, createAndMockUserAndToken(s.T(), id, false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) newTestTenantController() (*goa.Service, *controller.TenantController, *configuration.Data, func()) {
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	clusterService, authService, config, reset := prepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, config, reset
}

func createAndMockUserAndToken(t *testing.T, sub string, internal bool) context.Context {
	createTokenMock(sub)
	return createAndMockUser(t, sub, internal)
}

func createAndMockUser(t *testing.T, sub string, internal bool) context.Context {
	userToken, err := test.NewToken(
		map[string]interface{}{
			"sub":                sub,
			"preferred_username": "johny",
			"email":              "johny@redhat.com",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)
	featureLevel := ""
	if internal {
		featureLevel = auth.InternalFeatureLevel
	}

	createUserMock(sub, featureLevel)
	return goajwt.WithJWT(context.Background(), userToken)
}

func createUserMock(tenantId string, featureLevel string) {
	gock.New("http://authservice").
		Get("/api/users/" + tenantId).
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
		Reply(200).
		BodyString(fmt.Sprintf(`{
           	  "data": {
           		"attributes": {
                  "identityID": "%s",
           		  "cluster": "%s",
           		  "email": "johny@redhat.com",
                  "featureLevel": "%s"
           		}
           	  }
           	}`, tenantId, test.Normalize(test.ClusterURL), featureLevel))
}
func createTokenMock(tenantId string) {
	gock.New("http://authservice").
		Get("/api/token").
		MatchParam("for", test.Normalize(test.ClusterURL)).
		MatchParam("force_pull", "false").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub(tenantId))).
		Reply(200).
		BodyString(`{ 
      "token_type": "bearer",
      "username": "johny@redhat.com",
      "access_token": "jA0ECQMCWbHrs0GtZQlg0sDQAYMwVoNofrjMocCLv5+FR4GkCPEOiKvK6ifRVsZ6VWLcBVF5k/MFO0Y3EmE8O77xDFRvA9AVPETb7M873tGXMEmqFjgpWvppN81zgmk/enaeJbTBeYhXScyShw7G7kIbgaRy2ufPzVj7f2muM0PHRS334xOVtWZIuaq4lP7EZvW4u0JinSVT0oIHBoCKDFlMlNS1sTygewyI3QOX1quLEEhaDr6/eTG66aTfqMYZQpM4B+m78mi02GLPx3Z24DpjzgshagmGQ8f2kj49QA0LbbFaCUvpqlyStkXNwFm7z+Vuefpp+XYGbD+8MfOKsQxDr7S6ziEdjs+zt/QAr1ZZyoPsC4TaE6kkY1JHIIcrdO5YoX6mbxDMdkLY1ybMN+qMNKtVW4eV9eh34fZKUJ6sjTfdaZ8DjN+rGDKMtZDqwa1h+YYz938jl/bRBEQjK479o7Y6Iu/v4Rwn4YjM4YGjlXs/T/rUO1uye3AWmVNFfi6GtqNpbsKEbkr80WKOOWiSuYeZHbXA7pWMit17U9LtUA=="
    }`)
}
