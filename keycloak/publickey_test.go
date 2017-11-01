package keycloak_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"github.com/stretchr/testify/assert"
)

// ignore for now, require vcr recording
func TestPublicKeys(t *testing.T) {
	u, err := keycloak.GetPublicKeys("https://auth.prod-preview.openshift.io")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(u))
}
