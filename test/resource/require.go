// Package resource is used to manage which tests shall be executed.
// Tests can specify which resources they require. If such a resource
// is not available at runtime, the test will be skipped.
// The availability of resources is determined by the presence of an
// environment variable that doesn't evaluate to false (e.g. "0", "no", "false").
// See strconv.ParseBool for more information what evaluates to false.
package resource

import (
	"fmt"
	"os"
	"strconv"
)

const (
	// UnitTest refers to the name of the environment variable that is used to
	// specify that unit tests shall be run. Unless this environment variable
	// is explicitly set to evaluate to false ("0", "no", or "false"), unit
	// tests are executed all the time.
	UnitTest = "F8_RESOURCE_UNIT_TEST"
	// Database refers to the name of the environment variable that is used to
	// specify that test can be run that require a database.
	Database = "F8_RESOURCE_DATABASE"
	// StSkipReasonValueFalse is the skip message for tests when an environment variable is present but evaluates to false.
	StSkipReasonValueFalse = "Skipping test because environment variable %s evaluates to false: %s"
	// StSkipReasonNotSet is the skip message for tests when an environment is not present.
	StSkipReasonNotSet = "Skipping test because environment variable %s is no set."
	// StSkipReasonParseError is the error message for tests where we're unable to parse the required
	// environment variable as a boolean value.
	StSkipReasonParseError = "Unable to parse value of environment variable %s as bool: %s"
)

// IsReady checks if all the given environment variables ("envVars") are set
// and if one is not set it will return false and the reason. The only exception is
// that the unit test resource is always considered to be available unless
// is is explicitly set to false (e.g. "no", "0", "false").
func IsReady(envVars ...string) (bool, string) {
	for _, envVar := range envVars {
		v, isSet := os.LookupEnv(envVar)

		// If we don't explicitly opt out from unit tests
		// by specifying F8_RESOURCE_UNIT_TEST=0
		// we're going to run them
		if !isSet && envVar == UnitTest {
			continue
		}

		// Skip test if environment variable is not set.
		if !isSet {
			return false, fmt.Sprintf(StSkipReasonNotSet, envVar)
		}
		// Try to convert to boolean value
		isTrue, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Sprintf(StSkipReasonParseError, envVar, v)
		}

		if !isTrue {
			return false, fmt.Sprintf(StSkipReasonValueFalse, envVar, v)
		}
	}
	return true, ""
}
