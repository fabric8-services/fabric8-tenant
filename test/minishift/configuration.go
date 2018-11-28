package minishift

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"strings"

	"github.com/spf13/viper"
)

const (
	// Constants for viper variable names. Will be used to set
	// default values as well as to get each value
	varMinishiftURL        = "minishift.url"
	varMinishiftUserName   = "minishift.user.name"
	varMinishiftUserToken  = "minishift.user.token"
	varMinishiftAdminName  = "minishift.admin.name"
	varMinishiftAdminToken = "minishift.admin.token"
)

// Data encapsulates the Viper configuration object which stores the configuration data in-memory.
type Data struct {
	v *viper.Viper
}

// NewData creates a configuration reader object using a configurable configuration file path
func NewData() (*Data, error) {
	c := Data{
		v: viper.New(),
	}
	c.v.SetEnvPrefix("F8")
	c.v.AutomaticEnv()
	c.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	c.v.SetTypeByDefaultValue(true)
	c.setConfigDefaults()

	return &c, nil
}

// String returns the current configuration as a string
func (c *Data) String() string {
	allSettings := c.v.AllSettings()
	y, err := yaml.Marshal(&allSettings)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"settings": allSettings,
			"err":      err,
		}).Panicln("Failed to marshall config to string")
	}
	return fmt.Sprintf("%s\n", y)
}

func (c *Data) setConfigDefaults() {
	c.v.SetTypeByDefaultValue(true)
	c.v.SetDefault(varMinishiftAdminName, "admin")
	c.v.SetDefault(varMinishiftUserName, "developer")
}

// GetMinishiftURL returns the Minishift URL
func (c *Data) GetMinishiftURL() string {
	return c.v.GetString(varMinishiftURL)
}

// GetMinishiftUserName returns the Minishift user name
func (c *Data) GetMinishiftUserName() string {
	return c.v.GetString(varMinishiftUserName)
}

// GetMinishiftUserToken returns the Minishift user token
func (c *Data) GetMinishiftUserToken() string {
	return c.v.GetString(varMinishiftUserToken)
}

// GetMinishiftAdminName returns the Minishift admin name
func (c *Data) GetMinishiftAdminName() string {
	return c.v.GetString(varMinishiftAdminName)
}

// GetMinishiftAdminToken returns the Minishift admin token
func (c *Data) GetMinishiftAdminToken() string {
	return c.v.GetString(varMinishiftAdminToken)
}
