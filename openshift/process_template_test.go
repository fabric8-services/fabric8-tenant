package openshift_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
)

func TestPresenceOfTemplateObjects(t *testing.T) {
	templateObjects := tmplObjects(t)

	t.Run("verify jenkins deployment config", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindDeploymentConfig,
				withNamespace("developer-jenkins")))
	})
	t.Run("verify jenkins deployment config", func(t *testing.T) {
		assert.NoError(t,
			contain(templateObjects,
				environment.ValKindDeploymentConfig,
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
		os.Setenv("DISABLE_OSO_QUOTAS", "true")
		templateObjects := tmplObjects(t)

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

func tmplObjects(t *testing.T) environment.Objects {
	data, err := testdoubles.LoadTestConfig()
	assert.NoError(t, err)
	config := openshift.Config{OriginalConfig: data, MasterUser: "master"}
	templs, err := openshift.LoadProcessedTemplates(context.Background(), config, "developer")
	assert.NoError(t, err)
	return templs
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
