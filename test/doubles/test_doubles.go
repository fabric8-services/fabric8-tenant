package testdoubles

import (
	vcrrecorder "github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/stretchr/testify/require"
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
	environment.VersionFabric8TenantCheFile = "123abc"
	environment.VersionFabric8TenantCheMtFile = "234bcd"
	environment.VersionFabric8TenantCheQuotasFile = "zyx098"
	environment.VersionFabric8TenantUserFile = "345cde"
	environment.VersionFabric8TenantDeployFile = "456def"
	environment.VersionFabric8TenantJenkinsFile = "567efg"
	environment.VersionFabric8TenantJenkinsQuotasFile = "yxw987"
}

func SetTemplateSameVersion(version string) {
	environment.VersionFabric8TenantCheFile = version
	environment.VersionFabric8TenantCheMtFile = version
	environment.VersionFabric8TenantCheQuotasFile = version
	environment.VersionFabric8TenantUserFile = version
	environment.VersionFabric8TenantDeployFile = version
	environment.VersionFabric8TenantJenkinsFile = version
	environment.VersionFabric8TenantJenkinsQuotasFile = version
}

func GetMappedVersions(envTypes ...string) map[string]string {
	mappedTemplates := environment.RetrieveMappedTemplates()
	typesWithVersion := map[string]string{}
	for _, envType := range envTypes {
		typesWithVersion[envType] = mappedTemplates[envType].ConstructCompleteVersion()
	}
	return typesWithVersion
}
