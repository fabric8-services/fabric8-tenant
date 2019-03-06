package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func singleObjectResolver(obj environment.Object) objectsForVersionRetrieval {
	return func(version string) (data *environment.EnvData, objects environment.Objects, e error) {
		return &environment.EnvData{}, environment.Objects{obj}, nil
	}
}

func slowObjectResolver(duration time.Duration, obj environment.Object) objectsForVersionRetrieval {
	return func(version string) (data *environment.EnvData, objects environment.Objects, e error) {
		time.Sleep(duration)
		return &environment.EnvData{}, environment.Objects{obj}, nil
	}
}

func errorResolver() objectsForVersionRetrieval {
	return func(version string) (data *environment.EnvData, objects environment.Objects, e error) {
		return nil, nil, fmt.Errorf("some error")
	}
}

func TestCacheReturnCachedDataSingleThread(t *testing.T) {
	// given
	c := RemovedObjectsCache{}

	toReturn := NewObject(environment.ValKindRoleBinding, "ns-name", "rb-name")
	otherObj := NewObject(environment.ValKindRoleBinding, "ns-name", "rb-name")

	// when
	first, err := c.GetResolver(environment.TypeChe, "123abc", singleObjectResolver(toReturn), environment.Objects{}).Resolve()
	require.NoError(t, err)
	second, err := c.GetResolver(environment.TypeChe, "123abc", singleObjectResolver(otherObj), environment.Objects{}).Resolve()
	require.NoError(t, err)

	// then
	require.Len(t, first, 1)
	assertThatObjectsAreSame(t, toReturn, first[0].toObject("ns-name"))
	require.Len(t, second, 1)
	assertThatObjectsAreSame(t, toReturn, second[0].toObject("ns-name"))
}

func TestCacheReturnCachedDataMultiThread(t *testing.T) {
	// given
	c := RemovedObjectsCache{}

	wg := sync.WaitGroup{}
	trigger := sync.WaitGroup{}
	trigger.Add(1)

	fetch := 10

	// when
	for _, envType := range environment.DefaultEnvTypes {

		obj := NewObject(environment.ValKindRole, "ns-name", fmt.Sprintf("r-%s", envType))

		for i := 0; i < fetch; i++ {
			wg.Add(1)
			go func(env environment.Type, toReturn environment.Object) {
				defer wg.Done()
				trigger.Wait()
				objs, err := c.GetResolver(env, "123abc", slowObjectResolver(1*time.Second, toReturn), environment.Objects{}).Resolve()

				// then
				require.NoError(t, err)
				require.Len(t, objs, 1)
				assertThatObjectsAreSame(t, toReturn, objs[0].toObject("ns-name"))
			}(envType, obj)

		}
		for i := 0; i < fetch; i++ {
			wg.Add(1)
			go func(env environment.Type, index int) {
				defer wg.Done()
				version := fmt.Sprintf("%d-version", index)
				versioned := NewObject(environment.ValKindRole, "ns-name", fmt.Sprintf("r-%s-%d", env, index))

				trigger.Wait()
				objs, err := c.GetResolver(env, version, slowObjectResolver(1*time.Second, versioned), environment.Objects{}).Resolve()

				// then
				require.NoError(t, err)
				require.Len(t, objs, 1)
				assert.Equal(t, versioned, objs[0].toObject("ns-name"))
			}(envType, i)
		}
	}
	trigger.Done()
	wg.Wait()
}

func TestCacheWhenResolverReturnsErrorForTheSecondCall(t *testing.T) {
	// given
	c := RemovedObjectsCache{}

	toReturn := NewObject(environment.ValKindRoleBinding, "ns-name", "rb-name")

	// when
	first, err := c.GetResolver(environment.TypeChe, "123abc", singleObjectResolver(toReturn), environment.Objects{}).Resolve()
	require.NoError(t, err)
	second, err := c.GetResolver(environment.TypeChe, "123abc", errorResolver(), environment.Objects{}).Resolve()
	require.NoError(t, err)

	// then
	require.Len(t, first, 1)
	assertThatObjectsAreSame(t, toReturn, first[0].toObject("ns-name"))
	require.Len(t, second, 1)
	assertThatObjectsAreSame(t, toReturn, second[0].toObject("ns-name"))
}

func TestCacheWhenResolverReturnsErrorForTheFirstCallAndIsRemoved(t *testing.T) {
	// given
	c := RemovedObjectsCache{}

	toReturn := NewObject(environment.ValKindRoleBinding, "ns-name", "rb-name")

	// when
	first, err := c.GetResolver(environment.TypeChe, "123abc", errorResolver(), environment.Objects{}).Resolve()

	// then
	assert.Error(t, err, test.HasMessage("some error"))
	assert.Nil(t, first)

	// and when
	c.RemoveResolver(environment.TypeChe, "123abc")
	second, err := c.GetResolver(environment.TypeChe, "123abc", singleObjectResolver(toReturn), environment.Objects{}).Resolve()
	require.NoError(t, err)

	// then
	require.Len(t, second, 1)
	assertThatObjectsAreSame(t, toReturn, second[0].toObject("ns-name"))
}

func TestRemovedObject(t *testing.T) {
	// given
	rmObject := removedObject{
		kind: environment.ValKindRole,
		name: "my-role",
	}

	// when
	obj := rmObject.toObject("my-namespace")

	// then
	require.Len(t, obj, 2)
	assert.Equal(t, environment.ValKindRole, environment.GetKind(obj))
	require.Contains(t, obj, "metadata")
	require.Len(t, obj["metadata"], 2)
	assert.Equal(t, "my-namespace", environment.GetNamespace(obj))
	assert.Equal(t, "my-role", environment.GetName(obj))
}

func assertThatObjectsAreSame(t *testing.T, expected environment.Object, actual environment.Object) {
	assert.Equal(t, environment.GetKind(expected), environment.GetKind(actual))
	assert.Equal(t, environment.GetName(expected), environment.GetName(actual))
	assert.Equal(t, environment.GetNamespace(expected), environment.GetNamespace(actual))
}
