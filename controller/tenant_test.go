package controller_test

import (
	"context"
	"net/http"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-tenant/user"
	"github.com/fabric8-services/fabric8-wit/resource"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type TenantControllerTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantController(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TenantControllerTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantControllerTestSuite) TestSetupTenant() {
	// given
	r, err := recorder.New("../test/data/controller/setup_tenant", recorder.WithJWTMatcher())
	require.NoError(s.T(), err)
	defer r.Stop()
	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(s.T(), err)
	svc, ctrl, err := newTestTenantController(saToken, s.DB, r.Transport)
	require.NoError(s.T(), err)

	s.T().Run("accepted", func(t *testing.T) {

		t.Run("no namespace already exists on tenant cluster", func(t *testing.T) {
			// given
			tenantID := "83fdcae2-634f-4a52-958a-f723cb621700" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_foo@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantAccepted(t, ctx, svc, ctrl)
		})
	})

	s.T().Run("fail", func(t *testing.T) {

		t.Run("tenant already exists in DB", func(t *testing.T) {
			// given a user that already exists in the tenant DB
			tenantID := uuid.NewV4()
			tenant.NewDBService(s.DB).SaveTenant(&tenant.Tenant{ID: tenantID})
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID.String(),
				"email": "user_known@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantConflict(t, ctx, svc, ctrl)
		})

		t.Run("missing token", func(t *testing.T) {
			// when using default context with no JWT
			test.SetupTenantUnauthorized(t, context.Background(), svc, ctrl)
		})

		t.Run("cluster not found", func(t *testing.T) {
			// given
			tenantID := "526ea9ac-0cf7-4e12-a835-0b76eab45517"
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_unknown_cluster@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantInternalServerError(t, ctx, svc, ctrl)
		})

		t.Run("namespace already exists on OpenShift", func(t *testing.T) {
			// given an account that already has a namespace with a different name on OpenShift
			tenantID := "02a6474c-3b04-4dc4-bfd2-4867102581e0"
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_ns_exists@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantConflict(t, ctx, svc, ctrl)
		})

		t.Run("quotad execeeded on tenant cluster", func(t *testing.T) {
			// given
			tenantID := "38b33b8b-996d-4ba4-b565-f32a526de85c" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_foo2@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantForbidden(t, ctx, svc, ctrl)
		})

	})

}

func newTestTenantController(saToken *jwt.Token, db *gorm.DB, rt http.RoundTripper) (*goa.Service, *controller.TenantController, error) {
	authURL := "http://authservice"
	resolveToken := token.NewResolve(authURL, configuration.WithRoundTripper(rt))
	clusterService := cluster.NewService(
		authURL,
		saToken.Raw,
		resolveToken,
		token.NewGPGDecypter("foo"),
		configuration.WithRoundTripper(rt),
	)
	clusters, err := clusterService.GetClusters(context.Background())
	if err != nil {
		return nil, nil, errs.Wrapf(err, "unable to initialize tenant controller")
	}
	resolveCluster := cluster.NewResolve(clusters)
	resolveTenant := func(ctx context.Context, target, userToken string) (user, accessToken string, err error) {
		// log.Debug(ctx, map[string]interface{}{"user_token": userToken}, "attempting to resolve tenant for user...")
		return resolveToken(ctx, target, userToken, false, token.PlainText) // no need to use "forcePull=true" to validate the user's token on the target.
	}
	tenantService := tenant.NewDBService(db)
	userService := user.NewService(
		authURL,
		saToken.Raw,
		configuration.WithRoundTripper(rt),
	)
	defaultOpenshiftConfig := openshift.Config{}
	templateVars := make(map[string]string)
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenantService, userService, resolveTenant, resolveCluster, defaultOpenshiftConfig, templateVars)
	return svc, ctrl, nil
}

func createValidUserContext(claims map[string]interface{}) (context.Context, error) {
	tok, err := testsupport.NewToken(jwt.MapClaims(claims), "../test/private_key.pem")
	if err != nil {
		return nil, errs.Wrapf(err, "failed to create token")
	}
	return goajwt.WithJWT(context.Background(), tok), nil
}
