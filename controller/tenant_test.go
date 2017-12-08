package controller

import (
	"context"
	"net/http"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	testtoggles "github.com/fabric8-services/fabric8-tenant/test/toggles"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TenantControllerTestSuite struct {
	suite.Suite
}

func TestTenantController(t *testing.T) {
	// resource.Require(t, resource.Database)
	suite.Run(t, &TenantControllerTestSuite{})
}

func (s *TenantControllerTestSuite) TestLoadTenantConfiguration() {

	// given
	svc := goa.New("Tenants-service")
	inputOpenShiftConfig := openshift.Config{
		CheVersion:     "che-version",
		JenkinsVersion: "jenkins-version",
		MavenRepoURL:   "maven-url",
		TeamVersion:    "team-version",
	}
	tenantID := uuid.NewV4()
	tenantService := mockTenantService{ID: tenantID}
	ctx := createValidUserContext(tenantID.String())

	s.T().Run("override disabled", func(t *testing.T) {

		// given
		userUpdateTenantFeature := testtoggles.NewMockFeature(toggles.UserUpdateTenantFeature, false)
		toggleClient := toggles.NewCustomClient(testtoggles.NewMockUnleashClient(*userUpdateTenantFeature), true)
		// ensure that the "override config" feature is disabled for the user
		require.False(t, toggleClient.IsEnabled(goajwt.ContextJWT(ctx), toggles.UserUpdateTenantFeature, false))

		t.Run("user has config in profile", func(t *testing.T) {
			// given
			r, err := recorder.New("../test/data/tenant/auth_get_user.withconfig")
			require.Nil(t, err)
			defer r.Stop()
			mockHTTPClient := &http.Client{
				Transport: r.Transport,
			}
			ctrl := NewTenantController(svc, tenantService, mockHTTPClient, toggleClient, keycloak.Config{}, inputOpenShiftConfig, map[string]string{}, "http://auth")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, inputOpenShiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, inputOpenShiftConfig, resultConfig)
		})

		t.Run("user has no config in profile", func(t *testing.T) {
			// given
			r, err := recorder.New("../test/data/tenant/auth_get_user.withoutconfig")
			require.Nil(t, err)
			defer r.Stop()
			mockHTTPClient := &http.Client{
				Transport: r.Transport,
			}
			ctrl := NewTenantController(svc, tenantService, mockHTTPClient, toggleClient, keycloak.Config{}, inputOpenShiftConfig, map[string]string{}, "http://auth")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, inputOpenShiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, inputOpenShiftConfig, resultConfig)
		})
	})

	s.T().Run("override enabled", func(t *testing.T) {
		// given
		userUpdateTenantFeature := testtoggles.NewMockFeature(toggles.UserUpdateTenantFeature, true)
		toggleClient := toggles.NewCustomClient(testtoggles.NewMockUnleashClient(*userUpdateTenantFeature), true)
		// ensure that the "override config" feature is enabled for the user
		require.True(t, toggleClient.IsEnabled(goajwt.ContextJWT(ctx), toggles.UserUpdateTenantFeature, false))

		t.Run("user has config in profile", func(t *testing.T) {
			// given
			r, err := recorder.New("../test/data/tenant/auth_get_user.withconfig")
			require.Nil(t, err)
			defer r.Stop()
			mockHTTPClient := &http.Client{
				Transport: r.Transport,
			}
			ctrl := NewTenantController(svc, tenantService, mockHTTPClient, toggleClient, keycloak.Config{}, inputOpenShiftConfig, map[string]string{}, "http://auth")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, inputOpenShiftConfig)
			// then
			require.NoError(t, err)
			expectedOpenshiftConfig := openshift.Config{
				CheVersion:     "another-che-version",
				JenkinsVersion: "another-jenkins-version",
				MavenRepoURL:   "another-maven-url",
				TeamVersion:    "another-team-version",
			}
			assert.Equal(t, expectedOpenshiftConfig, resultConfig)
		})

		t.Run("user has no config in profile", func(t *testing.T) {
			// given
			r, err := recorder.New("../test/data/tenant/auth_get_user.withoutconfig")
			require.Nil(t, err)
			defer r.Stop()
			mockHTTPClient := &http.Client{
				Transport: r.Transport,
			}
			ctrl := NewTenantController(svc, tenantService, mockHTTPClient, toggleClient, keycloak.Config{}, inputOpenShiftConfig, map[string]string{}, "http://auth")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, inputOpenShiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, inputOpenShiftConfig, resultConfig)
		})
	})

}

func createValidUserContext(userID string) context.Context {
	claims := jwt.MapClaims{}
	claims["sub"] = userID
	claims["session_state"] = "foo_session_id"
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}
