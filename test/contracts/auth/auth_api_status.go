// Package contracts contains a runnable Consumer Pact test example.
package contracts

import (
	"fmt"
	"log"
	"net/http"
	"testing"

	"github.com/pact-foundation/pact-go/dsl"
)

// AuthAPIStatus defines contract of /api/status endpoint
func AuthAPIStatus(t *testing.T, pact *dsl.Pact) {
	// Pass in test case
	var test = func() error {
		u := fmt.Sprintf("http://localhost:%d/api/status", pact.Server.Port)
		req, err := http.NewRequest("GET", u, nil)

		req.Header.Set("Content-Type", "application/json")
		if err != nil {
			return err
		}

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		return err
	}

	type STATUS struct {
		buildTime           string `json:"buildTime" pact:"example=2018-10-05T10:03:04Z"`
		commit              string `json:"commit" pact:"example=0f9921980549b2baeb43f6f16cbe794f430f498c"`
		configurationStatus string `json:"configurationStatus" pact:"example=OK"`
		databaseStatus      string `json:"databaseStatus" pact:"example=OK"`
		startTime           string `json:"startTime" pact:"example=2018-10-09T15:04:50Z"`
	}

	// Set up our expected interactions.
	pact.
		AddInteraction().
		UponReceiving("A request to get status").
		WithRequest(dsl.Request{
			Method:  "GET",
			Path:    dsl.String("/api/status"),
			Headers: dsl.MapMatcher{"Content-Type": dsl.String("application/json")},
		}).
		WillRespondWith(dsl.Response{
			Status:  200,
			Headers: dsl.MapMatcher{"Content-Type": dsl.String("application/vnd.status+json")},
			Body:    dsl.Match(STATUS{}),
		})

	// Verify
	if err := pact.Verify(test); err != nil {
		log.Fatalf("Error on Verify: %v", err)
	}
}
