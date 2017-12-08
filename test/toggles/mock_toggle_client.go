package toggles

import (
	unleash "github.com/Unleash/unleash-client-go"
	"github.com/fabric8-services/fabric8-wit/log"
)

// NewMockUnleashClient returns a new MockUnleashClient initialized with the given features and their strategies
func NewMockUnleashClient(features ...MockFeature) *MockUnleashClient {
	return &MockUnleashClient{
		Features: features,
	}
}

// MockUnleashClient a mock unleash client
type MockUnleashClient struct {
	Features []MockFeature
}

// IsEnabled mimicks the behaviour of the real client, always returns true
func (c *MockUnleashClient) IsEnabled(featureName string, opts ...unleash.FeatureOption) (enabled bool) {
	log.Info(nil, map[string]interface{}{"feature_name": featureName}, "checking if feature is enabled")
	for _, f := range c.Features {
		if f.name == featureName {
			return f.enabled
		}
	}
	log.Info(nil, map[string]interface{}{"feature_name": featureName}, "no matching feature configured in the client")
	return false
}

func (m *MockUnleashClient) Close() error {
	return nil
}

func (m *MockUnleashClient) Ready() <-chan bool {
	return nil
}
