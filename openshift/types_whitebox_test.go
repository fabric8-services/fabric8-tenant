package openshift

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/assert"
	"testing"
)

var userEditRb = `kind: RoleBinding
metadata:
  name: user-edit
  namespace: developer1-che
roleRef:
  name: edit
subjects:
- kind: User
  name: developer
userNames:
- developer
`

func TestAdditionalObjectForChe(t *testing.T) {
	config, reset := test.LoadTestConfig(t)
	defer reset()

	ctx := NewServiceContext(
		context.Background(), config, cluster.ForTypeMapping(map[environment.Type]cluster.Cluster{}), "developer", "developer1",
		func(cluster cluster.Cluster) string {
			return "userToken"
		})

	service := &CheNamespaceTypeService{
		CommonEnvTypeService: &CommonEnvTypeService{
			envType:    environment.TypeChe,
			context:    ctx,
			envService: environment.NewService(),
		},
	}

	t.Run("AdditionalObject for che type should return role binding and false when toggle returns false", func(t *testing.T) {
		// given
		service.isToggleEnabled = func(ctx context.Context, feature string, fallback bool) bool {
			return false
		}

		// when
		object, shouldBeCreated := service.AdditionalObject()

		// then
		assert.False(t, shouldBeCreated)
		assert.Equal(t, userEditRb, object.ToString())
	})

	t.Run("AdditionalObject for che type should return role binding and true when toggle returns true", func(t *testing.T) {
		// given
		service.isToggleEnabled = func(ctx context.Context, feature string, fallback bool) bool {
			return true
		}
		// when
		object, shouldBeCreated := service.AdditionalObject()

		// then
		assert.True(t, shouldBeCreated)
		assert.Equal(t, userEditRb, object.ToString())
	})
}
