package openshift_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
	"strings"
	"testing"
)

func TestEnvironmentTypeService(t *testing.T) {
	// given
	config, reset := test.LoadTestConfig(t)
	defer reset()

	clusterMapping := map[environment.Type]cluster.Cluster{}
	for _, envType := range environment.DefaultEnvTypes {
		url := fmt.Sprintf("http://starter-for-type-%s.com", envType.String())
		clusterMapping[envType] = cluster.Cluster{
			APIURL: url,
			Token:  "clusterToken",
		}
	}
	client := openshift.NewClient(nil, "https://starter.com", tokenProducer)

	ctx := openshift.NewServiceContext(
		context.Background(), config, cluster.ForTypeMapping(clusterMapping), "developer", "developer1", func(cluster cluster.Cluster) string {
			return "userToken"
		})
	envService := environment.NewService()

	t.Run("test service behavior common for all types", func(t *testing.T) {
		for _, envType := range environment.DefaultEnvTypes {
			// when
			service := openshift.NewEnvironmentTypeService(envType, ctx, envService)

			// then
			assert.Equal(t, envType, service.GetType())
			assert.Equal(t, fmt.Sprintf("http://starter-for-type-%s.com", envType.String()), service.GetCluster().APIURL)
			envData, objects, err := service.GetEnvDataAndObjects(func(objects environment.Object) bool {
				return true
			})
			assert.NoError(t, err)
			assert.Equal(t, envType, envData.EnvType)
			assert.NotEmpty(t, envData.Templates)
			assert.NotEmpty(t, objects)

			if envType != environment.TypeChe {
				object, add := service.AdditionalObject()
				assert.Empty(t, object)
				assert.True(t, add)
			} else {
				object, add := service.AdditionalObject()
				assert.NotEmpty(t, object)
				assert.False(t, add)
			}

			if envType != environment.TypeUser {
				assert.Equal(t, "clusterToken", service.GetTokenProducer(false)(false))
				assert.Equal(t, "clusterToken", service.GetTokenProducer(true)(true))
				assert.NoError(t, service.AfterCallback(client, "POST"))
				assert.Equal(t, "developer1-"+envType.String(), service.GetNamespaceName())
			}
		}
	})
	t.Run("test service behavior specific for user type", func(t *testing.T) {
		// when
		service := openshift.NewEnvironmentTypeService(environment.TypeUser, ctx, envService)

		// then
		assert.Equal(t, "clusterToken", service.GetTokenProducer(true)(true))
		assert.Equal(t, "clusterToken", service.GetTokenProducer(true)(false))
		assert.Equal(t, "clusterToken", service.GetTokenProducer(false)(true))
		assert.Equal(t, "userToken", service.GetTokenProducer(false)(false))
		assert.Equal(t, "developer1", service.GetNamespaceName())

		t.Run("when action is post then sends request to remove admin rolebinding", func(t *testing.T) {
			// given
			defer gock.OffAll()
			gock.New("https://starter.com").
				Delete("/oapi/v1/namespaces/developer1/rolebindings/admin").
				Reply(200)
			// when
			err := service.AfterCallback(client, "POST")
			// then
			assert.NoError(t, err)
		})
		t.Run("when action is post then sends request to remove admin rolebinding", func(t *testing.T) {
			// given
			defer gock.OffAll()
			gock.New("https://starter.com").
				Delete("/oapi/v1/namespaces/developer1/rolebindings/admin").
				Reply(505)
			// when
			err := service.AfterCallback(client, "POST")
			// then
			test.AssertError(t, err,
				test.HasMessageContaining("unable to remove admin rolebinding in developer1 namespace"),
				test.HasMessageContaining("server responded with status: 505 for the DELETE request"))
		})
		t.Run("when action is other than post then it does nothing", func(t *testing.T) {
			// when
			err := service.AfterCallback(client, "PATCH")
			// then
			assert.NoError(t, err)
		})
	})
}

func TestPresenceOfTemplateObjects(t *testing.T) {
	data, reset := test.LoadTestConfig(t)
	defer reset()
	templateObjects := testdoubles.AllDefaultObjects(t, data)

	t.Run("verify jenkins deployment config", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindDeploymentConfig,
				withNamespace("developer-jenkins")))
	})
	t.Run("verify jenkins deployment config", func(t *testing.T) {
		assert.Error(t,
			contain(templateObjects,
				environment.ValKindDeploymentConfig,
				withNamespace("developer-che")))
	})
	t.Run("verify jenkins deployment config", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindPersistentVolumeClaim,
				withNamespace("developer-che")))
	})

	t.Run("verify che project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer-che")))
	})
	t.Run("verify jenkins project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer-jenkins")))
	})
	t.Run("verify run project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer-run")))
	})
	t.Run("verify stage project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer-stage")))
	})
	t.Run("verify user project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer")))
	})
	t.Run("verify user project request", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindProjectRequest,
				withName("developer")))
	})

	t.Run("verify resource quotas", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindResourceQuota,
				withNamespace("developer-che")))
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindResourceQuota,
				withNamespace("developer-jenkins")))
	})

	t.Run("verify resource quotas are not present when DISABLE_OSO_QUOTAS is true", func(t *testing.T) {
		resetEnv := test.SetEnvironments(test.Env("DISABLE_OSO_QUOTAS", "true"))
		defer resetEnv()
		data, reset := test.LoadTestConfig(t)
		defer reset()
		templateObjects := testdoubles.AllDefaultObjects(t, data)

		assert.Error(t,
			contain(templateObjects,
				environment.ValKindResourceQuota,
				withNamespace("developer-che")))
		assert.Error(t,
			contain(templateObjects,
				environment.ValKindResourceQuota,
				withNamespace("developer-jenkins")))
	})

	t.Run("verify all variables are replaced", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects, "", withoutNotReplacedVariable()))
	})
}

func contain(templates environment.Objects, kind string, checks ...func(environment.Object) error) error {
	var err error
	for _, tmpl := range templates {
		if environment.GetKind(tmpl) == kind || kind == "" {
			err = nil
			for _, check := range checks {
				if e := check(tmpl); e != nil {
					err = e
				}
			}
			if err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("no template of kind %v found, cause %v", kind, err)
}

func withNamespace(name string) func(environment.Object) error {
	return func(temp environment.Object) error {
		val := environment.GetNamespace(temp)
		if val == name {
			return nil
		}
		return fmt.Errorf("no namespace match for %v found", name)
	}
}

func withName(name string) func(environment.Object) error {
	return func(temp environment.Object) error {
		val := environment.GetName(temp)
		if val == name {
			return nil
		}
		return fmt.Errorf("no name match for %v found", name)
	}
}

func withoutNotReplacedVariable() func(environment.Object) error {
	return func(temp environment.Object) error {
		if strings.Contains(fmt.Sprint(temp), "${") {
			return fmt.Errorf("object %s contains not replaced variable", temp)
		}
		return nil
	}
}
