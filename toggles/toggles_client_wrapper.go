package toggles

import (
	"os"
	"time"

	"github.com/Unleash/unleash-client-go"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-wit/log"
)

// UnleashClient the interface for the underlying Unleash client
type UnleashClient interface {
	Ready() <-chan bool
	IsEnabled(feature string, options ...unleash.FeatureOption) (enabled bool)
	Close() error
}

// Client the wrapper for the toggle client
type Client struct {
	ready         bool
	UnleashClient UnleashClient
}

// NewClient returns a new client to the toggle feature service including the default underlying unleash client initialized
func NewClient(serviceName, hostURL string) (*Client, error) {
	l := clientListener{}
	unleashclient, err := unleash.NewClient(
		unleash.WithAppName(serviceName),
		unleash.WithInstanceId(os.Getenv("HOSTNAME")),
		unleash.WithUrl(hostURL),
		unleash.WithMetricsInterval(1*time.Minute),
		unleash.WithRefreshInterval(10*time.Second),
		unleash.WithListener(l),
	)
	if err != nil {
		return nil, err
	}
	result := NewCustomClient(unleashclient, false)
	l.client = result
	return result, nil
}

// NewCustomClient returns a new client to the toggle feature service with a pre-initialized unleash client
func NewCustomClient(unleashclient UnleashClient, ready bool) *Client {
	result := &Client{
		UnleashClient: unleashclient,
		ready:         ready,
	}
	return result
}

// Close closes the underlying Unleash client
func (c *Client) Close() error {
	return c.UnleashClient.Close()
}

// Ready returns `true` if the client is ready
func (c *Client) Ready() bool {
	return c.ready
}

// IsEnabled returns a boolean to specify whether on feature is enabled given the user's context (in particular, his/her token claims)
func (c *Client) IsEnabled(token *jwt.Token, feature string, fallback bool) bool {
	log.Info(nil, map[string]interface{}{"feature_name": feature, "ready": c.Ready()}, "checking if feature is enabled")
	if !c.Ready() {
		return fallback
	}
	return c.UnleashClient.IsEnabled(feature, WithToken(token), unleash.WithFallback(fallback))
}
