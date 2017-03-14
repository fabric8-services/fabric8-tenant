package openshift_test

import (
	"testing"

	"github.com/fabric8io/fabric8-init-tenant/openshift"
	"github.com/stretchr/testify/assert"
)

var applyTemplate = `
---
apiVersion: v1
kind: Template
metadata:
  labels:
    provider: fabric8
    project: fabric8-online-team-environments
    version: 1.0.58
    group: io.fabric8.online.packages
  name: fabric8-online-team-envi
objects:
- apiVersion: v1
  kind: Project
  metadata:
    annotations:
      openshift.io/description: Test-Project-Description
      openshift.io/display-name: Test-Project-Name
      openshift.io/requester: Aslak-User
    labels:
      provider: fabric8
      project: fabric8-online-team-environments
      version: 1.0.58
      group: io.fabric8.online.packages
    name: aslak-test
`

// Ignore for now. Need vcr setup to record openshift interactions
func xTestApply(t *testing.T) {
	opts := openshift.ApplyOptions{
		Config: openshift.Config{
			MasterURL: "https://tsrv.devshift.net:8443",
			Token:     "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8",
		},
	}

	t.Run("apply single project", func(t *testing.T) {
		result := openshift.Apply(applyTemplate, opts)
		assert.NoError(t, result, "apply error")
	})

}
