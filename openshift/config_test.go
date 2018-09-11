package openshift

import (
	"testing"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/stretchr/testify/assert"
)

var contextInfo = map[string]interface{}{
	"tenantConfig": map[string]interface{}{
		"templatesRepo":     "http://my.own.repo",
		"templatesRepoBlob": "12345",
		"templatesRepoDir":  "my/own/dir",
	},
}

func TestTenantOverride(t *testing.T) {
	internalFeatureLevel := "internal"
	otherFeatureLevel := "producation"
	config := Config{}

	t.Run("override disabled", func(t *testing.T) {

		t.Run("external user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: contextInfo,
				FeatureLevel:       &otherFeatureLevel,
			}
			// when
			resultConfig := setTemplateRepoInfo(user, config)
			// then
			assert.Equal(t, config, resultConfig)
		})

		t.Run("external user without config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{}
			// when
			resultConfig := setTemplateRepoInfo(user, config)
			// then
			assert.Equal(t, config, resultConfig)
		})
	})

	t.Run("override enabled", func(t *testing.T) {

		t.Run("internal user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: contextInfo,
				FeatureLevel:       &internalFeatureLevel,
			}
			// when
			resultConfig := setTemplateRepoInfo(user, config)
			// then
			assert.Equal(t, resultConfig.TemplatesRepo, "http://my.own.repo")
			assert.Equal(t, resultConfig.TemplatesRepoBlob, "12345")
			assert.Equal(t, resultConfig.TemplatesRepoDir, "my/own/dir")
		})

		t.Run("internal user without config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				FeatureLevel: &internalFeatureLevel,
			}
			// when
			resultConfig := setTemplateRepoInfo(user, config)
			// then
			assert.Equal(t, config, resultConfig)
		})
	})

}
