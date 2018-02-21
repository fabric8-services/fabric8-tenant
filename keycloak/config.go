package keycloak

import (
	"fmt"
)

// Config contains basic configuration data for Keycloak
type Config struct {
	BaseURL string
	Realm   string
	Broker  string
}

// RealmAuthURL return endpoint for realm auth config "{BaseURL}/auth/realms/{Realm}/broker/{Broker}/token"
func (c Config) RealmAuthURL() string {
	return fmt.Sprintf("%v/auth/realms/%v", c.BaseURL, c.Realm)
}

// BrokerTokenURL return endpoint to fetch Brokern token "{BaseURL}/auth/realms/{Realm}/broker/{Broker}/token"
func (c Config) BrokerTokenURL() string {
	return c.CustomBrokerTokenURL(c.Broker)
}

// CustomBrokerTokenURL return endpoint to fetch Brokern token "{BaseURL}/auth/realms/{Realm}/broker/{Broker}/token"
func (c Config) CustomBrokerTokenURL(broker string) string {
	return fmt.Sprintf("%v/auth/realms/%v/broker/%v/token", c.BaseURL, c.Realm, broker)
}
