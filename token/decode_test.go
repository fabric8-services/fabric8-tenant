package token_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptSuccess(t *testing.T) {

	testEncryptedMessage := `jA0ECQMCtCG1bfGEQbxg0kABEQ6nh/A4tMGGkHMHJtLDtFLyXh28IuLvoyGjsZtWPV0LHwN+EEsTtu90BQGbWFdBv+2Wiedk9eE3h08lwA8m`

	t.Run("success", func(t *testing.T) {
		// when
		txt, err := token.NewGPGDecypter("foo")(testEncryptedMessage)
		// then
		require.NoError(t, err)
		require.NotNil(t, txt)
		assert.Equal(t, "SuperSecret", *txt)
	})

	t.Run("fail", func(t *testing.T) {
		// when
		_, err := token.NewGPGDecypter("foo2")(testEncryptedMessage)
		// then
		require.Error(t, err)
	})

}
