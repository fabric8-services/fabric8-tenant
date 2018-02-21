package openshift

import (
	"fmt"
	"net/http"

	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/cluster"
)

type Config struct {
	MasterURL      string
	MasterUser     string
	Token          string
	HttpTransport  *http.Transport
	TemplateDir    string
	MavenRepoURL   string
	ConsoleURL     string
	TeamVersion    string
	CheVersion     string
	JenkinsVersion string
	LogCallback    LogCallback
}

// NewConfig builds openshift config for every user request depending on the user profile
func NewConfig(osTemplate Config, user *authclient.UserDataAttributes, cluster *cluster.Cluster) Config {
	return overrideTemplateVersions(user, osTemplate.WithMasterUser(cluster.User).WithToken(cluster.Token).WithMasterURL(cluster.APIURL))
}

// overrideTemplateVersions returns a new config in which the template versions have been overridden
func overrideTemplateVersions(user *authclient.UserDataAttributes, config Config) Config {
	if user.FeatureLevel != nil && *user.FeatureLevel != "internal" {
		return config
	}
	userContext := user.ContextInformation
	if tc, found := userContext["tenantConfig"]; found {
		if tenantConfig, ok := tc.(map[string]interface{}); ok {
			find := func(key, defaultValue string) string {
				if rawValue, found := tenantConfig[key]; found {
					if value, ok := rawValue.(string); ok {
						return value
					}
				}
				return defaultValue
			}
			return config.WithUserSettings(
				find("cheVersion", config.CheVersion),
				find("jenkinsVersion", config.JenkinsVersion),
				find("teamVersion", config.TeamVersion),
				find("mavenRepo", config.MavenRepoURL),
			)
		}
	}
	return config
}

type LogCallback func(message string)

// WithToken returns a new config with an override of the token
func (c Config) WithToken(token string) Config {
	c.Token = token
	return c
}

// WithUserSettings returns a new config with an override of the user settings
func (c Config) WithUserSettings(cheVersion string, jenkinsVersion string, teamVersion string, mavenRepoURL string) Config {
	if len(cheVersion) > 0 || len(jenkinsVersion) > 0 || len(teamVersion) > 0 || len(mavenRepoURL) > 0 {
		copy := c
		if cheVersion != "" {
			copy.CheVersion = cheVersion
		}
		if jenkinsVersion != "" {
			copy.JenkinsVersion = jenkinsVersion
		}
		if teamVersion != "" {
			copy.TeamVersion = teamVersion
		}
		if mavenRepoURL != "" {
			copy.MavenRepoURL = mavenRepoURL
		}
		return copy
	}
	return c
}

// WithMasterUser returns a new config with an override of the master user
func (c Config) WithMasterUser(masterUser string) Config {
	c.MasterUser = masterUser
	return c
}

// WithMasterURL returns a new config with an override of the master URL
func (c Config) WithMasterURL(masterURL string) Config {
	c.MasterURL = masterURL
	return c
}

// GetLogCallback returns the log callback function if defined in the config, otherwise a `nil log callback`
func (c Config) GetLogCallback() LogCallback {
	if c.LogCallback == nil {
		return nilLogCallback
	}
	return c.LogCallback
}

func nilLogCallback(string) {
}

type multiError struct {
	Message string
	Errors  []error
}

func (m multiError) Error() string {
	s := m.Message + "\n"
	for _, err := range m.Errors {
		s += fmt.Sprintf("%v\n", err)
	}
	return s
}

func (m *multiError) String() string {
	return m.Error()
}
