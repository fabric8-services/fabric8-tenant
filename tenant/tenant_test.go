package tenant_test

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetNamespaceType(t *testing.T) {

	t.Run("should detect ns as user type when has same name", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("account-for-test", "account-for-test")

		// then
		assert.Equal(t, namespaceType, environment.TypeUser)
	})

	t.Run("should detect ns as run type when ends with run", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("account-for-test-run", "account-for-test")

		// then
		assert.Equal(t, namespaceType, environment.TypeRun)
	})

	t.Run("should detect ns as stage type when ends with stage", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("account-for-test-stage-stage", "account-for-stage")

		// then
		assert.Equal(t, namespaceType, environment.TypeStage)
	})

	t.Run("should detect ns as che type when ends with che", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("che-che", "che")

		// then
		assert.Equal(t, namespaceType, environment.TypeChe)
	})

	t.Run("should detect ns as jenkins type when ends with jenkins", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("any-run-stage-jenkins", "any-run-stage")

		// then
		assert.Equal(t, namespaceType, environment.TypeJenkins)
	})

	t.Run("should detect ns as custom type when ends with unknown suffix", func(t *testing.T) {
		// when
		namespaceType := tenant.GetNamespaceType("any-run-stage-custom", "any-run-stage")

		// then
		assert.Equal(t, namespaceType, environment.TypeCustom)
	})
}
