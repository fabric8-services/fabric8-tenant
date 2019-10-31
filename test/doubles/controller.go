package testdoubles

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"testing"
	"time"
)

func CreateAndMockUserAndToken(t *testing.T, sub string, internal bool) context.Context {
	createTokenMock(sub)
	return CreateAndMockUser(t, sub, internal)
}

func CreateAndMockUser(t *testing.T, sub string, internal bool) context.Context {
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
		MatchParam("for", test.ClusterURL).
		MatchParam("force_pull", "false").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub(tenantId))).
		Reply(200).
		BodyString(`{ 
      "token_type": "bearer",
      "username": "johny@redhat.com",
      "access_token": "jA0ECQMCWbHrs0GtZQlg0sDQAYMwVoNofrjMocCLv5+FR4GkCPEOiKvK6ifRVsZ6VWLcBVF5k/MFO0Y3EmE8O77xDFRvA9AVPETb7M873tGXMEmqFjgpWvppN81zgmk/enaeJbTBeYhXScyShw7G7kIbgaRy2ufPzVj7f2muM0PHRS334xOVtWZIuaq4lP7EZvW4u0JinSVT0oIHBoCKDFlMlNS1sTygewyI3QOX1quLEEhaDr6/eTG66aTfqMYZQpM4B+m78mi02GLPx3Z24DpjzgshagmGQ8f2kj49QA0LbbFaCUvpqlyStkXNwFm7z+Vuefpp+XYGbD+8MfOKsQxDr7S6ziEdjs+zt/QAr1ZZyoPsC4TaE6kkY1JHIIcrdO5YoX6mbxDMdkLY1ybMN+qMNKtVW4eV9eh34fZKUJ6sjTfdaZ8DjN+rGDKMtZDqwa1h+YYz938jl/bRBEQjK479o7Y6Iu/v4Rwn4YjM4YGjlXs/T/rUO1uye3AWmVNFfi6GtqNpbsKEbkr80WKOOWiSuYeZHbXA7pWMit17U9LtUA=="
    }`)
}

func PrepareConfigClusterAndAuthService(t *testing.T) (cluster.Service, auth.Service, *configuration.Data, func()) {
	return PrepareConfigClusterAndAuthServiceWithRefreshInt(time.Hour, t)
}

func PrepareConfigClusterAndAuthServiceWithRefreshInt(refreshInt time.Duration, t *testing.T) (cluster.Service, auth.Service, *configuration.Data, func()) {
	saToken, err := test.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	resetVars := test.SetEnvironments(test.Env("F8_AUTH_TOKEN_KEY", "foo"), test.Env("F8_API_SERVER_USE_TLS", "false"))
	authService, _, cleanup :=
		NewAuthServiceWithRecorder(t, "", "http://authservice", saToken.Raw, recorder.WithJWTMatcher)
	config, resetConf := test.LoadTestConfig(t)
	reset := func() {
		resetVars()
		cleanup()
		resetConf()
	}

	clusterService := cluster.NewClusterService(refreshInt, authService)
	err = clusterService.Start()
	require.NoError(t, err)
	return clusterService, authService, config, reset
}
