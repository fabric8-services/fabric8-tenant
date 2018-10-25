package openshift_test

import (
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	. "github.com/fabric8-services/fabric8-tenant/test"
	. "github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"os"
	"testing"
)

var templateHeader = `
---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-tenant-team-environments
    version: 1.0.58
    group: io.fabric8.online.packages
  name: fabric8-tenant-team
objects:
`
var projectRequestObject = `
- apiVersion: v1
  kind: ProjectRequest
  metadata:
    annotations:
      openshift.io/description: run-Project-Description
      openshift.io/display-name: run-Project-Name
      openshift.io/requester: Aslak-User
    labels:
      provider: fabric8
      project: fabric8-tenant-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: ${USER_NAME}-run
`
var roleBindingRestrictionObject = `
- apiVersion: v1
  kind: RoleBindingRestriction
  metadata:
    labels:
      app: fabric8-tenant-che-mt
      provider: fabric8
      version: 2.0.85
      group: io.fabric8.tenant.packages
    name: dsaas-user-access
    namespace: ${USER_NAME}-run
  spec:
    userrestriction:
      users:
      - ${PROJECT_USER}
`

func TestMain(m *testing.M) {
	defer gock.Off()
	retCode := m.Run()
	os.Exit(retCode)
}

func TestInvokePostAndGetCallsForAllObjects(t *testing.T) {
	// given
	config, reset := LoadTestConfig(t)
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject + roleBindingRestrictionObject)
	gock.New("http://starter.com").
		Post("/oapi/v1/projectrequests").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/projects/aslak-run").
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)

	service, db := NewOSService(
		config,
		WithTenant(uuid.NewV4()),
		SingleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"))

	// when
	err := service.WithPostMethod().ApplyAll(environment.TypeRun)

	// then
	require.NoError(t, err)
	require.Len(t, db.Namespaces, 1)
	require.Equal(t, "aslak-run", db.Namespaces[0].Name)
	require.Equal(t, tenant.Ready.String(), db.Namespaces[0].State.String())
}

func TestDeleteIfThereIsConflict(t *testing.T) {
	// given
	config, reset := LoadTestConfig(t)
	defer reset()

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + roleBindingRestrictionObject)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(409)
	gock.New("http://starter.com").
		Delete("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-run/rolebindingrestrictions/dsaas-user-access").
		Reply(200).
		BodyString(roleBindingRestrictionObject)

	service, db := NewOSService(
		config,
		WithTenant(uuid.NewV4()),
		SingleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"))

	// when
	err := service.WithPostMethod().ApplyAll(environment.TypeRun)

	// then
	require.NoError(t, err)
	require.Len(t, db.Namespaces, 1)
	require.Equal(t, "aslak-run", db.Namespaces[0].Name)
	require.Equal(t, tenant.Ready.String(), db.Namespaces[0].State.String())
}

func TestDeleteAndGet(t *testing.T) {
	// given
	config, reset := LoadTestConfig(t)
	defer reset()

	tok, err := NewToken(
		map[string]interface{}{
			"sub": "clusterUser",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	gock.New("https://raw.githubusercontent.com").
		Get("fabric8-services/fabric8-tenant/12345/environment/templates/fabric8-tenant-deploy.yml").
		Reply(200).
		BodyString(templateHeader + projectRequestObject)
	gock.New("http://starter.com").
		Delete("/oapi/v1/projects/aslak-run").
		SetMatcher(ExpectRequest(HasJWTWithSub("clusterUser"))).
		Reply(200)

	tenantId := uuid.NewV4()
	service, db := NewOSService(
		config,
		WithTenant(tenantId, Ns("aslak-run", environment.TypeRun)),
		SingleClusterMapping("http://starter.com", "clusterUser", tok.Raw),
		WithUser(NewUserDataWithTenantConfig("", "12345", ""), "aslak", "abc123"))

	// when
	err = service.WithDeleteMethod(db.Namespaces, true).ApplyAll()

	// then
	require.NoError(t, err)
	require.Empty(t, db.Namespaces)
	require.Empty(t, db.Tenants)
}

func TestNumberOfCallsToCluster(t *testing.T) {
	// given
	config, reset := LoadTestConfig(t)
	defer reset()
	SetTemplateVersions()

	calls := 0
	gock.New("http://my.cluster").
		Post("").
		SetMatcher(SpyOnCalls(&calls)).
		Times(78).
		Reply(200).
		BodyString("{}")

	gock.New("http://my.cluster").
		Get("").
		Times(11).
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)

	gock.New("http://my.cluster").
		Delete("").
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)

	service, db := NewOSService(
		config,
		WithTenant(uuid.NewV4()),
		SingleClusterMapping("http://my.cluster", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"),
		WithUser(&authclient.UserDataAttributes{}, "developer", "12345"))

	objectsInTemplates := tmplObjects(t, config)

	// when
	err := service.WithPostMethod().ApplyAll(environment.DefaultEnvTypes...)

	// then
	require.NoError(t, err)
	assert.Equal(t, len(objectsInTemplates), calls)
	assert.Len(t, db.Namespaces, 5)
}

// SpyOnCalls checks the number of calls
func SpyOnCalls(counter *int) gock.Matcher {
	matcher := gock.NewBasicMatcher()
	matcher.Add(func(req *http.Request, _ *gock.Request) (bool, error) {
		*counter++
		return true, nil
	})
	return matcher
}
