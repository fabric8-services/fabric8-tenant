package environment_test

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"regexp"
	"sort"
	"testing"
)

var processTemplate = `
- apiVersion: v1
  kind: Project
  metadata:
    annotations:
      openshift.io/description: ${PROJECT_DESCRIPTION}
      openshift.io/display-name: ${PROJECT_DISPLAYNAME}
      openshift.io/requester: ${PROJECT_REQUESTING_USER}
    labels:
      provider: fabric8
      project: fabric8-tenant-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: ${PROJECT_NAME}
    credentials.xml.tpl: |-
      <?xml version='1.0' encoding='UTF-8'?>
      <com.cloudbees.plugins.credentials.SystemCredentialsProvider plugin="credentials@1.23">
      </com.cloudbees.plugins.credentials.SystemCredentialsProvider>
`

var parseNamespaceTemplate = `
---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-tenant-team-environments
    version: 1.0.58
    group: io.fabric8.online.packages
  name: fabric8-tenant-team-envi
objects:
- apiVersion: v1
  kind: Namespace
  metadata:
    annotations:
      openshift.io/description: Test-Project-Description
      openshift.io/display-name: Test-Project-Name
      openshift.io/requester: Aslak-User
    labels:
      provider: fabric8
      project: fabric8-tenant-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: aslak-test
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    labels:
      app: fabric8-tenant-che-mt
      provider: fabric8
      version: 2.0.85
      group: io.fabric8.tenant.packages
    name: dsaas-user-access
  spec:
    userrestriction:
      users:
      - ${PROJECT_USER}
`
var processTemplateVariables = `
- apiVersion: v1
  kind: Project
  metadata:
    labels:
      provider: fabric8
      project: fabric8-tenant-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    credentials.xml.tpl: |-
      <?xml version='1.0' encoding='UTF-8'?>
      <com.cloudbees.plugins.credentials.SystemCredentialsProvider plugin="credentials@1.23">
        <domainCredentialsMap class="hudson.util.CopyOnWriteMap$Hash">
          ${KUBERNETES_CREDENTIALS}
        </domainCredentialsMap>
      </com.cloudbees.plugins.credentials.SystemCredentialsProvider>
`
var sortTemplate = `
---
apiVersion: v1
kind: Template
objects:
- apiVersion: v1
  kind: Secret
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: ServiceAccount
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: RoleBinding
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    name: aslak-test
- apiVersion: v1
  kind: LimitRange
  metadata:
    name: aslak-test
`

var templateWithParams = `
apiVersion: v1
kind: Template
objects:
- apiVersion: v1
  kind: Project
  metadata:
    labels:
      user_name: ${USER_NAME}
      project_user: ${PROJECT_USER}
      project_displayname: ${PROJECT_DISPLAYNAME}
      commit: ${COMMIT}
`

var templateParams = `
parameters:
- name: USER_NAME
  value: developer
- name: JENKINS_OPENSHIFT_VERSION
  value: 9865421
- name: PROJECT_USER
  value: developer
- name: PROJECT_DESCRIPTION
- name: PROJECT_DISPLAYNAME
- name: JENKINS_ROOT_URL
- name: COMMIT
  value: abc
`

func TestSort(t *testing.T) {
	// given
	data, reset := test.LoadTestConfig(t)
	defer reset()

	template := environment.Template{Content: sortTemplate}
	objects, err := template.Process(environment.CollectVars("developer", "developer", "master", data))

	// when
	sort.Sort(environment.ByKind(objects))

	// then
	require.NoError(t, err)

	assert.Equal(t, "ProjectRequest", kind(objects[0]))
	assert.Equal(t, "RoleBindingRestriction", kind(objects[1]))
	assert.Equal(t, "LimitRange", kind(objects[2]))
	assert.Equal(t, "ResourceQuota", kind(objects[3]))
}

func TestParseNamespace(t *testing.T) {
	// given
	data, reset := test.LoadTestConfig(t)
	defer reset()

	template := environment.Template{Content: parseNamespaceTemplate}

	// when
	objects, err := template.Process(environment.CollectVars("developer", "developer", "master", data))

	// then
	require.NoError(t, err)

	assert.Equal(t, "Namespace", kind(objects[0]))
	assert.Equal(t, "RoleBindingRestriction", kind(objects[1]))
}

func kind(object environment.Object) string {
	return object["kind"].(string)
}

func TestProcess(t *testing.T) {
	// given
	vars := map[string]string{
		"PROJECT_DESCRIPTION":     "Test-Project-Description",
		"PROJECT_DISPLAYNAME":     "Test-Project-Name",
		"PROJECT_REQUESTING_USER": "Aslak-User",
		"PROJECT_NAME":            "Aslak-Test",
	}
	template := environment.Template{Content: processTemplate}

	// when
	processed, err := template.ReplaceVars(vars)

	// then
	require.Nil(t, err, "error processing templateDef")

	t.Run("verify no templateDef markers in output", func(t *testing.T) {
		assert.False(t, regexp.MustCompile(`\${([A-Z_]+)}`).MatchString(processed))
	})
	t.Run("verify markers were replaced", func(t *testing.T) {
		assert.Contains(t, processed, vars["PROJECT_DESCRIPTION"], "missing")
		assert.Contains(t, processed, vars["PROJECT_DISPLAYNAME"], "missing")
		assert.Contains(t, processed, vars["PROJECT_REQUESTING_USER"], "missing")
		assert.Contains(t, processed, vars["PROJECT_NAME"], "missing")
	})

	t.Run("Verify not escaping xml/html values", func(t *testing.T) {
		assert.Contains(t, processed, `<?xml version='1.0' encoding='UTF-8'?>`)
	})
}

func TestProcessVariables(t *testing.T) {
	// given
	vars := map[string]string{}

	template := environment.Template{Content: processTemplateVariables}

	// when
	processed, err := template.ReplaceVars(vars)

	// then
	require.Nil(t, err, "error processing templateDef")

	t.Run("Verify non replaced markers are left", func(t *testing.T) {
		assert.Contains(t, processed, "${KUBERNETES_CREDENTIALS}", "missing")
	})
}

func TestUseTemplateParams(t *testing.T) {
	// given
	vars := map[string]string{
		"USER_NAME":           "Aslak",
		"COMMIT":              "12345",
		"PROJECT_DISPLAYNAME": "Test-Project-Name",
	}
	template := environment.Template{Content: templateWithParams + templateParams}

	// when
	objects, err := template.Process(vars)

	// then
	require.NoError(t, err)

	assert.Len(t, objects, 1)
	assert.Equal(t, "Aslak", environment.GetLabel(objects[0], "user_name"))
	assert.Equal(t, "12345", environment.GetLabel(objects[0], "commit"))
	assert.Equal(t, "Test-Project-Name", environment.GetLabel(objects[0], "project_displayname"))

}

func TestRetrieveUserName(t *testing.T) {
	assertName(t, "some", "some@email.com")
	assertName(t, "so-me", "so-me@email.com")
	assertName(t, "some", "some")
	assertName(t, "so-me", "so-me")
	assertName(t, "so-me", "so_me")
	assertName(t, "so-me", "so me")
	assertName(t, "so-me", "so me@email.com")
	assertName(t, "so-me", "so.me")
	assertName(t, "so-me", "so?me")
	assertName(t, "so-me", "so:me")
	assertName(t, "some1", "some1")
	assertName(t, "so1me1", "so1me1")
	assertName(t, "os-me", "-me")
	assertName(t, "os-me", "_me")
	assertName(t, "me-io", "me-")
	assertName(t, "me-io", "me_")
	assertName(t, "os-me-io", "_me_")
	assertName(t, "os-me-io", "-me-")
	assertName(t, "os-12345-io", "12345")
}

func assertName(t *testing.T, expected, username string) {
	assert.Regexp(t, dnsRegExp, environment.RetrieveUserName(username))
	assert.Equal(t, expected, environment.RetrieveUserName(username))
}
