package tenant_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeys(t *testing.T) {

	t.Run("valid keys", func(t *testing.T) {
		u, err := tenant.GetPublicKeys("https://auth.prod-preview.openshift.io")
		require.NoError(t, err)
		assert.Equal(t, 2, len(u))
	})
	t.Run("invalid url", func(t *testing.T) {
		_, err := tenant.GetPublicKeys("http://google.com")
		assert.Error(t, err)
	})
}
