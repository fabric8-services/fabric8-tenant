package controller

import (
	"context"
	"crypto/rsa"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	jwtrequest "github.com/dgrijalva/jwt-go/request"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/fabric8-services/fabric8-tenant/openshift"
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
	openshiftConfig := openshift.Config{
		CheVersion:     "che-version",
		JenkinsVersion: "jenkins-version",
		MavenRepoURL:   "maven-url",
		TeamVersion:    "team-version",
	}

	s.T().Run("override disabled", func(t *testing.T) {

		t.Run("external user with config", func(t *testing.T) {
			// given
			ctrl := newTenantController(t, openshiftConfig)
			ctx := createValidContext(s.T(), "external_user_with_config")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, openshiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, openshiftConfig, resultConfig)
		})

		t.Run("external user without config", func(t *testing.T) {
			// given
			ctrl := newTenantController(t, openshiftConfig)
			ctx := createValidContext(s.T(), "external_user_without_config")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, openshiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, openshiftConfig, resultConfig)
		})
	})

	s.T().Run("override enabled", func(t *testing.T) {

		t.Run("internal user with config", func(t *testing.T) {
			// given
			ctrl := newTenantController(t, openshiftConfig)
			ctx := createValidContext(s.T(), "internal_user_with_config")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, openshiftConfig)
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

		t.Run("internal user without config", func(t *testing.T) {
			// given
			ctrl := newTenantController(t, openshiftConfig)
			ctx := createValidContext(s.T(), "internal_user_without_config")
			// when
			resultConfig, err := ctrl.loadUserTenantConfiguration(ctx, openshiftConfig)
			// then
			require.NoError(t, err)
			assert.Equal(t, openshiftConfig, resultConfig)
		})
	})

}

func newTenantController(t *testing.T, defaultConfig openshift.Config) *TenantController {
	svc := goa.New("Tenants-service")
	authURL := "http://auth-test"
	templateVars := make(map[string]string)
	tenantService := mockTenantService{ID: uuid.NewV4()}
	r, err := recorder.New("../test/data/auth/auth_get_user")
	require.Nil(t, err)
	r.SetMatcher(jwtMatcher())
	defer r.Stop()
	mockHTTPClient := &http.Client{
		Transport: r.Transport,
	}
	return NewTenantController(svc, tenantService, mockHTTPClient, keycloak.Config{}, defaultConfig, templateVars, authURL)
}

func jwtMatcher() cassette.Matcher {
	log.Println("Using a custom cassette matcher...")
	return func(httpRequest *http.Request, cassetteRequest cassette.Request) bool {
		if httpRequest.URL != nil && httpRequest.URL.String() != cassetteRequest.URL {
			log.Printf("Request URL does not match with cassette: %s vs %s\n", httpRequest.URL.String(), cassetteRequest.URL)
			return false
		}
		if httpRequest.Method != cassetteRequest.Method {
			log.Printf("Request Method does not match with cassette: %s vs %s\n", httpRequest.Method, cassetteRequest.Method)
			return false
		}

		// look-up the JWT's "sub" claim and compare with the request
		token, err := jwtrequest.ParseFromRequest(httpRequest, jwtrequest.AuthorizationHeaderExtractor, func(*jwt.Token) (interface{}, error) {
			return PublicKey()
		})
		if err != nil {
			log.Panic(nil, map[string]interface{}{"error": err.Error()}, "failed to parse token from request")
		}
		claims := token.Claims.(jwt.MapClaims)
		if sub, found := cassetteRequest.Headers["sub"]; found {
			log.Printf("Comparing subs: %s vs %s\n", sub[0], claims["sub"])
			return sub[0] == claims["sub"]
		}
		log.Printf("Request token does not match with cassette")
		return false
	}
}

func createValidContext(t *testing.T, userID string) context.Context {
	claims := jwt.MapClaims{}
	claims["sub"] = userID
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	// use the test private key to sign the token
	key, err := PrivateKey()
	require.NoError(t, err)
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	token.Raw = signed
	return goajwt.WithJWT(context.Background(), token)
}

func PrivateKey() (*rsa.PrivateKey, error) {
	rsaPrivateKey, err := ioutil.ReadFile("../test/private_key.pem")
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPrivateKeyFromPEM(rsaPrivateKey)
}

func PublicKey() (*rsa.PublicKey, error) {
	rsaPublicKey, err := ioutil.ReadFile("../test/public_key.pem")
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(rsaPublicKey)
}
