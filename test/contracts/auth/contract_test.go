// Package contracts contains a runnable Consumer Pact test example.
package contracts

import (
	"os"
	"testing"

	"github.com/pact-foundation/pact-go/dsl"
)

// TestAuthAPI runs all user related tests
func TestAuthAPI(t *testing.T) {
	// Create Pact connecting to local Daemon
	pact := &dsl.Pact{
		Consumer:          os.Getenv("PACT_CONSUMER"),
		Provider:          os.Getenv("PACT_PROVIDER"),
		Host:              "localhost",
		PactFileWriteMode: "merge",
	}
	defer pact.Teardown()

	// Test interactions
	AuthAPIStatus(t, pact)
	AuthAPIUserByNameConsumer(t, pact)
	AuthAPIUserByIDConsumer(t, pact)

	// Negative tests
	AuthAPIUserInvalidToken(t, pact)
	AuthAPIUserNoToken(t, pact)
}
