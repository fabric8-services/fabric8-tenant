package auth_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeys(t *testing.T) {

	t.Run("valid keys", func(t *testing.T) {
		//given
		authService, r, err := testdoubles.NewAuthClientService("../test/data/token/auth_get_keys", "http://authservice")
		require.NoError(t, err)
		defer r.Stop()
		// when
		result, err := authService.GetPublicKeys()
		// then
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("invalid url", func(t *testing.T) {
		//given
		authService, _, err := testdoubles.NewAuthClientService("", "http://google.com")
		assert.NoError(t, err)
		// when
		_, err = authService.GetPublicKeys()
		// then
		assert.Error(t, err)
	})
}
