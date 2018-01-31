package controller

import (
	"testing"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/stretchr/testify/assert"
)

func TestTenantOverride(t *testing.T) {
	internalFeatureLevel := "internal"
	otherFeatureLevel := "producation"
	openshiftConfig := openshift.Config{
		CheVersion:     "che-version",
		JenkinsVersion: "jenkins-version",
		MavenRepoURL:   "maven-url",
		TeamVersion:    "team-version",
	}

	t.Run("override disabled", func(t *testing.T) {

		t.Run("external user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: map[string]interface{}{
					"tenantConfig": map[string]interface{}{
						"cheVersion":     "another-che-version",
						"jenkinsVersion": "another-jenkins-version",
						"teamVersion":    "another-team-version",
						"mavenRepo":      "another-maven-url",
					},
				},
				FeatureLevel: &otherFeatureLevel,
			}
			// when
			resultConfig := overrideTemplateVersions(user, openshiftConfig)
			// then
			assert.Equal(t, openshiftConfig, resultConfig)
		})

		t.Run("external user without config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{}
			// when
			resultConfig := overrideTemplateVersions(user, openshiftConfig)
			// then
			assert.Equal(t, openshiftConfig, resultConfig)
		})
	})

	t.Run("override enabled", func(t *testing.T) {

		t.Run("internal user with config", func(t *testing.T) {
			// given
			user := &authclient.UserDataAttributes{
				ContextInformation: map[string]interface{}{
					"tenantConfig": map[string]interface{}{
						"cheVersion":     "another-che-version",
						"jenkinsVersion": "another-jenkins-version",
						"teamVersion":    "another-team-version",
						"mavenRepo":      "another-maven-url",
					},
				},
				FeatureLevel: &internalFeatureLevel,
			}
			// when
			resultConfig := overrideTemplateVersions(user, openshiftConfig)
			// then
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
			user := &authclient.UserDataAttributes{
				FeatureLevel: &internalFeatureLevel,
			}
			// when
			resultConfig := overrideTemplateVersions(user, openshiftConfig)
			// then
			assert.Equal(t, openshiftConfig, resultConfig)
		})
	})

}
