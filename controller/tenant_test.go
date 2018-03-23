package controller_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

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
	"github.com/stretchr/testify/assert"
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

	s.T().Run("accepted", func(t *testing.T) {

		t.Run("no namespace already exists on tenant cluster", func(t *testing.T) {
			// given
			svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-1")
			require.NoError(t, err)
			tenantID := "83fdcae2-634f-4a52-958a-f723cb621700" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_foo@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantAccepted(t, ctx, svc, ctrl, nil)
		})
	})

	s.T().Run("fail", func(t *testing.T) {

		t.Run("tenant already exists in DB", func(t *testing.T) {
			// given a user that already exists in the tenant DB
			svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-2")
			require.NoError(t, err)
			tenantID := uuid.NewV4()
			tenant.NewDBService(s.DB).SaveTenant(&tenant.Tenant{ID: tenantID})
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID.String(),
				"email": "user_known@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantConflict(t, ctx, svc, ctrl, nil)
		})

		t.Run("missing token", func(t *testing.T) {
			// given
			svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-3")
			require.NoError(t, err)
			// when using default context with no JWT
			test.SetupTenantUnauthorized(t, context.Background(), svc, ctrl, nil)
		})

		t.Run("cluster not found", func(t *testing.T) {
			// given
			svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-4")
			require.NoError(t, err)
			tenantID := "526ea9ac-0cf7-4e12-a835-0b76eab45517"
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_unknown_cluster@bar.com",
			})
			require.NoError(t, err)
			// when/then
			test.SetupTenantInternalServerError(t, ctx, svc, ctrl, nil)
		})

		t.Run("namespace already exists on OpenShift", func(t *testing.T) {

			t.Run("without x-forwarded-path", func(t *testing.T) {
				// given an account that already has a namespace with a different name on OpenShift
				svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-5")
				require.NoError(t, err)
				tenantID := "02a6474c-3b04-4dc4-bfd2-4867102581e0"
				ctx, err := createValidUserContext(map[string]interface{}{
					"sub":   tenantID,
					"email": "user_with_namespace@bar.com",
				})
				require.NoError(t, err)
				// when
				_, jsonAPIErr := test.SetupTenantConflict(t, ctx, svc, ctrl, nil)
				// then
				require.NotEmpty(t, jsonAPIErr.Errors)
				require.NotNil(t, jsonAPIErr.Errors[0].Links)
				t.Logf("JSON-API error links: %v\n", jsonAPIErr.Errors[0].Links)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["user-with-namespace"])
				assert.Equal(t, "http:///api/tenant/namespaces/user-with-namespace", *jsonAPIErr.Errors[0].Links["user-with-namespace"].Href)
			})

			t.Run("with x-forwarded-path", func(t *testing.T) {
				// given an account that already has a namespace with a different name on OpenShift
				svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-6")
				require.NoError(t, err)
				tenantID := "0443beeb-1cfb-427f-bd7c-d22d941bea4f"
				ctx, err := createValidUserContext(map[string]interface{}{
					"sub":   tenantID,
					"email": "user_with_namespace2@bar.com",
				})
				require.NoError(t, err)
				xForwardedPath := "/api/user"
				// when
				_, jsonAPIErr := test.SetupTenantConflict(t, ctx, svc, ctrl, &xForwardedPath)
				//then
				require.NotEmpty(t, jsonAPIErr.Errors)
				require.NotNil(t, jsonAPIErr.Errors[0].Links)
				t.Logf("JSON-API error links: %v\n", jsonAPIErr.Errors[0].Links)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["user-with-namespace2"])
				assert.Equal(t, "http:///api/user/namespaces/user-with-namespace2", *jsonAPIErr.Errors[0].Links["user-with-namespace2"].Href)
			})
		})

		t.Run("quotad execeeded on tenant cluster", func(t *testing.T) {

			t.Run("without x-forwarded-path", func(t *testing.T) {
				// given
				svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-7")
				require.NoError(t, err)
				tenantID := "38b33b8b-996d-4ba4-b565-f32a526de85c" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
				ctx, err := createValidUserContext(map[string]interface{}{
					"sub":   tenantID,
					"email": "user_foo2@bar.com",
				})
				require.NoError(t, err)
				// when
				_, jsonAPIErr := test.SetupTenantForbidden(t, ctx, svc, ctrl, nil)
				//then
				require.NotEmpty(t, jsonAPIErr.Errors)
				require.NotNil(t, jsonAPIErr.Errors[0].Links)
				t.Logf("JSON-API error links: %v\n", jsonAPIErr.Errors[0].Links)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["foo1"])
				assert.Equal(t, "http:///api/tenant/namespaces/foo1", *jsonAPIErr.Errors[0].Links["foo1"].Href)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["foo2"])
				assert.Equal(t, "http:///api/tenant/namespaces/foo2", *jsonAPIErr.Errors[0].Links["foo2"].Href)
			})

			t.Run("with x-forwarded-path", func(t *testing.T) {
				// given
				svc, ctrl, err := newTestTenantController(s.DB, "setup_tenant-8")
				require.NoError(t, err)
				tenantID := "da6d50b9-0086-4aec-9fcd-2882c09ea53b" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
				ctx, err := createValidUserContext(map[string]interface{}{
					"sub":   tenantID,
					"email": "user_foo3@bar.com",
				})
				require.NoError(t, err)
				xForwardedPath := "/api/user"
				// when
				_, jsonAPIErr := test.SetupTenantForbidden(t, ctx, svc, ctrl, &xForwardedPath)
				//then
				require.NotEmpty(t, jsonAPIErr.Errors)
				require.NotNil(t, jsonAPIErr.Errors[0].Links)
				t.Logf("JSON-API error links: %v\n", jsonAPIErr.Errors[0].Links)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["foo1"])
				assert.Equal(t, "http:///api/user/namespaces/foo1", *jsonAPIErr.Errors[0].Links["foo1"].Href)
				require.NotNil(t, jsonAPIErr.Errors[0].Links["foo2"])
				assert.Equal(t, "http:///api/user/namespaces/foo2", *jsonAPIErr.Errors[0].Links["foo2"].Href)
			})
		})

	})

}

func (s *TenantControllerTestSuite) TestDeleteNamespace() {

	s.T().Run("ok", func(t *testing.T) {
		// given
		svc, ctrl, err := newTestTenantController(s.DB, "delete_namespace-1")
		require.NoError(t, err)
		tenantID := "6d603ab4-7c5e-4c5f-bba8-a3ba9d370985" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
		ctx, err := createValidUserContext(map[string]interface{}{
			"sub":   tenantID,
			"email": "user_foo@bar.com",
		})
		require.NoError(t, err)

		// when
		test.DeleteNamespaceTenantAccepted(t, ctx, svc, ctrl, "foo")
	})

	s.T().Run("fail", func(t *testing.T) {

		t.Run("not found", func(t *testing.T) {
			// given
			svc, ctrl, err := newTestTenantController(s.DB, "delete_namespace-2")
			require.NoError(t, err)
			tenantID := "3194ab60-855b-4155-9005-9dce4a05f1eb" // ok, well... we could probably use a random UUID and use it in a template based on "../test/data/controller/setup_tenant" to generate the actual cassette file to use with go-vcr...
			ctx, err := createValidUserContext(map[string]interface{}{
				"sub":   tenantID,
				"email": "user_foo@bar.com",
			})
			require.NoError(t, err)

			// when
			test.DeleteNamespaceTenantNotFound(t, ctx, svc, ctrl, "unknown")
		})
	})

}
func newTestTenantController(db *gorm.DB, filename string) (*goa.Service, *controller.TenantController, error) {
	r, err := recorder.New(fmt.Sprintf("../test/data/controller/%s", filename), recorder.WithJWTMatcher())
	if err != nil {
		return nil, nil, errs.Wrapf(err, "unable to initialize tenant controller")
	}
	defer r.Stop()

	saToken, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	if err != nil {
		fmt.Printf("error occurred: %v", err)
		return nil, nil, errs.Wrapf(err, "unable to initialize tenant controller")
	}

	authURL := "http://authservice"
	resolveToken := token.NewResolve(authURL, configuration.WithRoundTripper(r))
	clusterService := cluster.NewService(
		authURL,
		time.Hour, // don't want to interfer with the refresher here
		saToken.Raw,
		resolveToken,
		token.NewGPGDecypter("foo"),
		configuration.WithRoundTripper(r),
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
		configuration.WithRoundTripper(r),
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
	req := &http.Request{
		Host: "https://example.com",
	}
	ctx := goa.NewContext(context.Background(), nil, req, nil)
	return goajwt.WithJWT(ctx, tok), nil
}
