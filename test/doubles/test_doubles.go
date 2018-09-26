package testdoubles

import "os"

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
