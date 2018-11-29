package test

import (
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func LoadTestConfig(t *testing.T) (*configuration.Data, func()) {
	reset := SetEnvironments(
		Env("F8_TEMPLATE_RECOMMENDER_EXTERNAL_NAME", "recommender.api.prod-preview.openshift.io"),
		Env("F8_TEMPLATE_RECOMMENDER_API_TOKEN", "xxxx"),
		Env("F8_TEMPLATE_DOMAIN", "d800.free-int.openshiftapps.com"),
		Env("F8_API_SERVER_INSECURE_SKIP_TLS_VERIFY", "true"))
	data, err := configuration.GetData()
	require.NoError(t, err)
	return data, reset
}

func Env(key, value string) Environment {
	return Environment{key: key, value: value}
}

type Environment struct {
	key, value string
}

func SetEnvironments(environments ...Environment) func() {
	originalValues := make([]Environment, 0, len(environments))

	for _, env := range environments {
		originalValues = append(originalValues, Env(env.key, os.Getenv(env.key)))
		os.Setenv(env.key, env.value)
	}
	return func() {
		for _, env := range originalValues {
			os.Setenv(env.key, env.value)
		}
	}
}
