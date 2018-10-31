package openshift_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"os"
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
  name: fabric8-tenant-team-envi
objects:
`
var projectRequestObject = `
- apiVersion: v1
  kind: ProjectRequest
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
    name: ${USER_NAME}-test
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
    namespace: ${USER_NAME}-test
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
	config, reset := test.LoadTestConfig(t)
	defer reset()
	objects, opts := prepareObjectsAndOpts(t, templateHeader+projectRequestObject+roleBindingRestrictionObject, config)

	gock.New("http://starter.com").
		Post("/oapi/v1/projectrequests").
		Reply(200)
	gock.New("http://starter.com").
		Get("/api/v1/namespaces/aslak-test").
		Reply(200)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(200)

	// when
	err := openshift.ApplyProcessed(objects, opts)

	// then
	require.NoError(t, err)
}

func TestDeleteIfThereIsConflict(t *testing.T) {
	// given
	config, reset := test.LoadTestConfig(t)
	defer reset()
	objects, opts := prepareObjectsAndOpts(t, templateHeader+roleBindingRestrictionObject, config)

	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions").
		Reply(409)
	gock.New("http://starter.com").
		Delete("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(404)
	gock.New("http://starter.com").
		Post("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(200)

	// when
	err := openshift.ApplyProcessed(objects, opts)

	// then
	require.NoError(t, err)
}

func TestDeleteAndGet(t *testing.T) {
	// given
	config, reset := test.LoadTestConfig(t)
	defer reset()
	objects, opts := prepareObjectsAndOpts(t, templateHeader+roleBindingRestrictionObject, config)

	gock.New("http://starter.com").
		Delete("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(200)
	gock.New("http://starter.com").
		Get("/oapi/v1/namespaces/aslak-test/rolebindingrestrictions/dsaas-user-access").
		Reply(404)

	// when
	err := openshift.ApplyProcessed(objects, opts)

	// then
	require.NoError(t, err)
}

func prepareObjectsAndOpts(t *testing.T, content string, config *configuration.Data) (environment.Objects, openshift.ApplyOptions) {
	template := environment.Template{Content: content}
	objects, err := template.Process(environment.CollectVars("aslak", "master", config))
	require.NoError(t, err)
	opts := openshift.ApplyOptions{
		Config: openshift.Config{
			MasterURL: "http://starter.com",
			Token:     "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8",
		},
	}
	return objects, opts
}
