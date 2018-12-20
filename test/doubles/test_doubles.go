package testdoubles

import (
	"fmt"
	vcrrecorder "github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"testing"
)

func NewAuthService(t *testing.T, cassetteFile, authURL, saToken string, options ...recorder.Option) (auth.Service, func()) {
	authService, _, cleanup := NewAuthServiceWithRecorder(t, cassetteFile, authURL, saToken, options...)
	return authService, cleanup
}

func NewAuthServiceWithRecorder(t *testing.T, cassetteFile, authURL, saToken string, options ...recorder.Option) (auth.Service, *vcrrecorder.Recorder, func()) {
	var clientOptions []configuration.HTTPClientOption
	var r *vcrrecorder.Recorder
	var err error
	if cassetteFile != "" {
		r, err = recorder.New(cassetteFile, options...)
		require.NoError(t, err)
		clientOptions = append(clientOptions, configuration.WithRoundTripper(r))
	}
	resetBack := test.SetEnvironments(test.Env("F8_AUTH_URL", authURL))
	config, err := configuration.GetData()
	require.NoError(t, err)

	authService := auth.NewAuthServiceWithToken(config, saToken, clientOptions...)

	return authService, r, func() {
		if r != nil {
			err := r.Stop()
			require.NoError(t, err)
		}
		resetBack()
	}
}

func SetTemplateVersions() {
	environment.VersionFabric8TenantCheMtFile = "234bcd"
	environment.VersionFabric8TenantCheQuotasFile = "zyx098"
	environment.VersionFabric8TenantUserFile = "345cde"
	environment.VersionFabric8TenantDeployFile = "456def"
	environment.VersionFabric8TenantJenkinsFile = "567efg"
	environment.VersionFabric8TenantJenkinsQuotasFile = "yxw987"
}

func SetTemplateSameVersion(version string) {
	environment.VersionFabric8TenantCheMtFile = version
	environment.VersionFabric8TenantCheQuotasFile = version
	environment.VersionFabric8TenantUserFile = version
	environment.VersionFabric8TenantDeployFile = version
	environment.VersionFabric8TenantJenkinsFile = version
	environment.VersionFabric8TenantJenkinsQuotasFile = version
}

func GetMappedVersions(envTypes ...environment.Type) map[environment.Type]string {
	mappedTemplates := environment.RetrieveMappedTemplates()
	typesWithVersion := map[environment.Type]string{}
	for _, envType := range envTypes {
		typesWithVersion[envType] = mappedTemplates[envType].ConstructCompleteVersion()
	}
	return typesWithVersion
}

func MockCommunicationWithAuth(cluster string, otherClusters ...string) {
	clusterList := ""
	var clusters []string
	clusters = append(otherClusters, cluster)
	for _, cl := range clusters {
		gock.New("http://authservice").
			Get("/api/token").
			Persist().
			MatchParam("for", cl+"/").
			MatchParam("force_pull", "false").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
			Reply(200).
			BodyString(`{ 
			"access_token": "jA0ECQMCYyjV8Zo7wgNg0sDQAUvut+valbh3k/zKDx+KPXcR7mmt7toLkc9Px7XaVMT6lQ6S7aOl6T8hpoPIWIEJuY33hZmJGmEXKkFzkU4BKcDaMnZXhiuwz4ECxOaeREpsUNCd7KSLayFGwuTuXbVwErmZau12CCCIjvlyJH89dCIkZD2hcElOhY6avEXfxQprtDF9iLddHiT+EOwZCSDOMKQbXVyAKR5FDaW8NXQpr7xsTmbe7dpoeS/uvIe2C5vEAH7dnc/TN5HmWYf0Is4ukfznKYef/+E+oSg3UkAO3i7PTFVsRuJCaN4pTIOcgeWjT7pvB49rb9UAZSfwSLDqbHgEfzjEatlC9PszMDlVckqvzg0Y0vhr+HpcvaJuu1VMy6Y5KH6NT4VlnL8tPFIcEeDJZLOreSmi43gkcl8YgTQp8G9C4h5h2nmS4E+1oU14uoBKwpjlek9r/x/o/hinYUrmSsht9FnQbbJAq7Umm/RbmanE47q86gy59UCTlW+zig8cp02pwQ7BW23YRrpZkiVB2QVmOGqB3+NCmK0pMg==",
			"token_type": "bearer",
			"username": "tenant_service"
    }`)

		gock.New(cl).
			Get("/apis/user.openshift.io/v1/users/~").
			SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
			Persist().
			Reply(200).
			BodyString(`{
     "kind":"User",
     "apiVersion":"user.openshift.io/v1",
     "metadata":{
       "name":"tenant_service",
       "selfLink":"/apis/user.openshift.io/v1/users/tenant_service",
       "uid":"bcdd0b29-123d-11e8-a8bc-b69930b94f5c",
       "resourceVersion":"814",
       "creationTimestamp":"2018-02-15T10:48:20Z"
     },
     "identities":[],
     "groups":[]
   }`)

		clusterList += fmt.Sprintf(`
        {
          "name": "cluster_name",
          "api-url": "%s/",
          "console-url": "http://console.cluster1/console/",
          "metrics-url": "http://metrics.cluster1/",
          "logging-url": "http://logging.cluster1/",
          "app-dns": "foo",
          "capacity-exhausted": false
        },`, cl)

	}

	gock.New("http://authservice").
		Get("/api/clusters/").
		SetMatcher(test.ExpectRequest(test.HasJWTWithSub("tenant_service"))).
		Persist().
		Reply(200).
		BodyString(fmt.Sprintf(`{
      "data":[
        %s
      ]
    }`, clusterList[:len(clusterList)-1]))
}
