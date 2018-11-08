package controller_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/errors"
	goatest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
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
	defer gock.Off()
	mockCommunicationWithAuth()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		defer gock.Off()
		fxt := tf.NewTestFixture(t, s.Repo, tf.Tenants(1), tf.Namespaces(1))
		// when
		_, tenant := goatest.ShowTenantOK(t, createAndMockUserAndToken(s.T(), fxt.Tenants[0].ID.String(), false), svc, ctrl)
		// then
		assert.Equal(t, fxt.Tenants[0].ID, *tenant.Data.ID)
		assert.Equal(t, 1, len(tenant.Data.Attributes.Namespaces))
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.ShowTenantUnauthorized(t, context.Background(), svc, ctrl)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.Off()

			// when/then
			goatest.ShowTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.ShowTenantNotFound(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenNoTenantExists() {
	// given
	// given
	defer gock.Off()
	mockCommunicationWithAuth()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, "https://api.cluster1/")
	// when
	goatest.SetupTenantAccepted(s.T(), createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	// then
	assert.Equal(s.T(), testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config), calls)

}

func (s *TenantControllerTestSuite) TestSetupTenantOKWhenAlreadyExists() {
	// given
	defer gock.Off()
	mockCommunicationWithAuth()
	id := uuid.NewV4()
	s.createFixtures(id, environment.TypeChe)
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()
	calls := 0
	testdoubles.MockPostRequestsToOS(&calls, "https://api.cluster1/")

	// when
	goatest.SetupTenantAccepted(s.T(), createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
	// then
	totalNumber := testdoubles.ExpectedNumberOfCallsWhenPost(s.T(), config)
	cheObjects := testdoubles.SingleTemplatesObjects(s.T(), config, environment.TypeChe)
	numberOfGetChecksForChe := testdoubles.NumberOfGetChecks(cheObjects)
	assert.Equal(s.T(), totalNumber-(len(cheObjects)+numberOfGetChecksForChe), calls)
}

func (s *TenantControllerTestSuite) TestSetupUnauthorizedFailures() {

	defer gock.Off()
	mockCommunicationWithAuth()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("Unauhorized - no token", func(t *testing.T) {
		defer gock.Off()
		// when/then
		goatest.SetupTenantUnauthorized(t, context.Background(), svc, ctrl)
	})

	s.T().Run("Unauhorized - invalid token", func(t *testing.T) {
		// given
		defer gock.Off()

		// when/then
		goatest.SetupTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
	})

	s.T().Run("Internal error because of 500 returned from OS", func(t *testing.T) {
		// given
		defer gock.Off()
		mockCommunicationWithAuth()
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()
		calls := 0
		gock.New("https://api.cluster1/").
			Post("/oapi/v1/namespaces/johny-jenkins/rolebindingrestrictions").
			Reply(500)
		testdoubles.MockPostRequestsToOS(&calls, "https://api.cluster1/")
		// when
		goatest.SetupTenantInternalServerError(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
	})
}
func (s *TenantControllerTestSuite) TestSetupConflictFailure() {

	defer gock.Off()
	mockCommunicationWithAuth()
	id := uuid.NewV4()
	s.createFixtures(id, environment.DefaultEnvTypes...)
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	// when/then
	goatest.SetupTenantConflict(s.T(), createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
}

func (s *TenantControllerTestSuite) TestDeleteTenantOK() {
	// given
	defer gock.Off()
	id := uuid.NewV4()

	s.T().Run("with existing namespaces", func(t *testing.T) {
		s.createFixtures(id, environment.DefaultEnvTypes...)
		mockCommunicationWithAuth()
		svc, ctrl, config, reset := s.newTestTenantController()
		defer reset()

		t.Run("only clean namespaces", func(t *testing.T) {
			// given
			defer gock.Off()
			calls := 0
			testdoubles.MockCleanRequestsToOS(&calls, "https://api.cluster1/")
			// when
			goatest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl, false)
			// then
			objects := testdoubles.AllTemplatesObjects(s.T(), config)
			assert.Equal(s.T(), testdoubles.NumberOfObjectsToClean(objects), calls)
			_, err := s.Repo.GetTenant(id)
			assert.NoError(t, err)
			namespaces, err := s.Repo.GetNamespaces(id)
			assert.NoError(t, err)
			assert.Len(t, namespaces, 5)
		})

		t.Run("remove namespaces and tenant", func(t *testing.T) {
			// given
			defer gock.Off()
			calls := 0
			testdoubles.MockRemoveRequestsToOS(&calls, "https://api.cluster1/")
			// when
			goatest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
			// then
			objects := testdoubles.AllTemplatesObjects(s.T(), config)
			assert.Equal(s.T(), testdoubles.NumberOfObjectsToRemove(objects), calls)
			_, err := s.Repo.GetTenant(id)
			test.AssertError(t, err, test.IsOfType(errors.NotFoundError{}))
			namespaces, err := s.Repo.GetNamespaces(id)
			assert.NoError(t, err)
			assert.Len(t, namespaces, 0)
		})
	})

	s.T().Run("remove namespaces and tenant is ok even when one namespace was already removed", func(t *testing.T) {
		// given
		defer gock.Off()
		calls := 0
		s.createFixtures(id, environment.DefaultEnvTypes...)
		mockCommunicationWithAuth()
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()
		gock.New("https://api.cluster1").
			Delete("/oapi/v1/projects/johny1-che").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
			Reply(404)
		testdoubles.MockRemoveRequestsToOS(&calls, "https://api.cluster1/")
		// when
		goatest.CleanTenantNoContent(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
		// then
		_, err := s.Repo.GetTenant(id)
		test.AssertError(t, err, test.IsOfType(errors.NotFoundError{}))
		namespaces, err := s.Repo.GetNamespaces(id)
		assert.NoError(t, err)
		assert.Len(t, namespaces, 0)
	})
}

func (s *TenantControllerTestSuite) TestDeleteTenantFailures() {
	// given
	defer gock.Off()
	mockCommunicationWithAuth()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.CleanTenantUnauthorized(t, context.Background(), svc, ctrl, false)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.Off()

			// when/then
			goatest.CleanTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl, false)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.CleanTenantNotFound(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl, false)
		})
	})

	s.T().Run("clean tenant fails when one namespace removal fails", func(t *testing.T) {
		// given
		defer gock.Off()
		calls := 0
		id := uuid.NewV4()
		s.createFixtures(id, environment.DefaultEnvTypes...)
		mockCommunicationWithAuth()
		svc, ctrl, _, reset := s.newTestTenantController()
		defer reset()

		t.Run("clean tenant fails when one namespace removal fails", func(t *testing.T) {
			// given
			defer gock.Off()
			gock.New("https://api.cluster1").
				Delete("/api/v1/namespaces/johny1-che/configmaps").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			testdoubles.MockCleanRequestsToOS(&calls, "https://api.cluster1/")
			// when
			goatest.CleanTenantInternalServerError(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, false)
			// then
			_, err := s.Repo.GetTenant(id)
			assert.NoError(t, err)
			namespaces, err := s.Repo.GetNamespaces(id)
			assert.NoError(t, err)
			assert.Len(t, namespaces, 5)
		})

		t.Run("remove tenant fails when one namespace removal fails", func(t *testing.T) {
			// given
			defer gock.Off()
			gock.New("https://api.cluster1").
				Delete("/oapi/v1/projects/johny1-che").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			testdoubles.MockRemoveRequestsToOS(&calls, "https://api.cluster1/")
			// when
			goatest.CleanTenantInternalServerError(s.T(), createAndMockUserAndToken(s.T(), id.String(), true), svc, ctrl, true)
			// then
			_, err := s.Repo.GetTenant(id)
			assert.NoError(t, err)
			namespaces, err := s.Repo.GetNamespaces(id)
			assert.NoError(t, err)
			assert.Len(t, namespaces, 1)
			assert.Equal(t, namespaces[0].Name, "johny1-che")
		})
	})
}

func (s *TenantControllerTestSuite) TestUpdateTenant() {
	// given
	defer gock.Off()
	mockCommunicationWithAuth()
	id := uuid.NewV4()
	s.createFixtures(id, environment.DefaultEnvTypes...)
	mockCommunicationWithAuth()
	svc, ctrl, config, reset := s.newTestTenantController()
	defer reset()

	s.T().Run("OK", func(t *testing.T) {
		// given
		defer gock.Off()
		calls := 0
		testdoubles.MockPatchRequestsToOS(&calls, "https://api.cluster1/")
		// when
		goatest.UpdateTenantAccepted(t, createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
		// then
		objects := testdoubles.AllTemplatesObjects(t, config)
		// get and patch requests for all objects but ProjectRequest
		assert.Equal(t, (len(objects)-5)*2, calls)
	})

	s.T().Run("Failures", func(t *testing.T) {

		t.Run("Unauhorized - no token", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.UpdateTenantUnauthorized(t, context.Background(), svc, ctrl)
		})

		t.Run("Unauhorized - invalid token", func(t *testing.T) {
			// given
			defer gock.Off()

			// when/then
			goatest.UpdateTenantUnauthorized(t, createAndMockUser(t, uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("Not found - non existing user", func(t *testing.T) {
			defer gock.Off()
			// when/then
			goatest.UpdateTenantNotFound(t, createAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)
		})

		t.Run("fails when an update of one object fails", func(t *testing.T) {
			// given
			defer gock.Off()
			gock.New("https://api.cluster1").
				Path("/api/v1/namespaces/johny1-che/configmaps").
				SetMatcher(test.ExpectRequest(test.HasJWTWithSub("devtools-sre"))).
				Reply(500)
			calls := 0
			testdoubles.MockPatchRequestsToOS(&calls, "https://api.cluster1/")
			// when/then
			goatest.UpdateTenantInternalServerError(t, createAndMockUserAndToken(s.T(), id.String(), false), svc, ctrl)
		})
	})
}

func (s *TenantControllerTestSuite) newTestTenantController() (*goa.Service, *controller.TenantController, *configuration.Data, func()) {
	clusterService, authService, config, reset := prepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, s.Repo, clusterService, authService, config)
	return svc, ctrl, config, reset
}

func createAndMockUserAndToken(t *testing.T, sub string, internal bool) context.Context {
	createTokenMock(sub)
	return createAndMockUser(t, sub, internal)
}

func (s *TenantControllerTestSuite) createFixtures(tenantId uuid.UUID, envTypes ...environment.Type) {
	tf.NewTestFixture(s.T(), s.Repo, tf.Tenants(1, func(fxt *tf.TestFixture, idx int) error {
		fxt.Tenants[0].ID = tenantId
		fxt.Tenants[0].OSUsername = "johny"
		fxt.Tenants[0].NsBaseName = "johny1"
		return nil
	}), tf.Namespaces(len(envTypes), func(fxt *tf.TestFixture, idx int) error {
		fxt.Namespaces[idx].TenantID = tenantId
		fxt.Namespaces[idx].Name = constuctNamespaceName("johny1", envTypes[idx])
		fxt.Namespaces[idx].Type = envTypes[idx]
		return nil
	}))
}

func constuctNamespaceName(nsBaseName string, envType environment.Type) string {
	if envType != environment.TypeUser {
		return nsBaseName + "-" + envType.String()
	}
	return nsBaseName
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
		featureLevel = "internal"
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
           		  "cluster": "https://api.cluster1/",
           		  "email": "johny@redhat.com",
                  "featureLevel": "%s"
           		}
           	  }
           	}`, tenantId, featureLevel))
}
func createTokenMock(tenantId string) {
	gock.New("http://authservice").
		Get("/api/token").
		MatchParam("for", "https://api.cluster1").
		MatchParam("force_pull", "false").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub(tenantId))).
		Reply(200).
		BodyString(`{ 
      "token_type": "bearer",
      "username": "johny@redhat.com",
      "access_token": "jA0ECQMCWbHrs0GtZQlg0sDQAYMwVoNofrjMocCLv5+FR4GkCPEOiKvK6ifRVsZ6VWLcBVF5k/MFO0Y3EmE8O77xDFRvA9AVPETb7M873tGXMEmqFjgpWvppN81zgmk/enaeJbTBeYhXScyShw7G7kIbgaRy2ufPzVj7f2muM0PHRS334xOVtWZIuaq4lP7EZvW4u0JinSVT0oIHBoCKDFlMlNS1sTygewyI3QOX1quLEEhaDr6/eTG66aTfqMYZQpM4B+m78mi02GLPx3Z24DpjzgshagmGQ8f2kj49QA0LbbFaCUvpqlyStkXNwFm7z+Vuefpp+XYGbD+8MfOKsQxDr7S6ziEdjs+zt/QAr1ZZyoPsC4TaE6kkY1JHIIcrdO5YoX6mbxDMdkLY1ybMN+qMNKtVW4eV9eh34fZKUJ6sjTfdaZ8DjN+rGDKMtZDqwa1h+YYz938jl/bRBEQjK479o7Y6Iu/v4Rwn4YjM4YGjlXs/T/rUO1uye3AWmVNFfi6GtqNpbsKEbkr80WKOOWiSuYeZHbXA7pWMit17U9LtUA=="
    }`)
}
