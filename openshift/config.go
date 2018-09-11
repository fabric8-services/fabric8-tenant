package openshift

import (
	"fmt"
	"net/http"

	"crypto/tls"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
)

// Config the configuration for the connection to Openshift and for the templates to apply
// TODO: split the config in 2 parts to distinguish connection settings vs template settings ?
type Config struct {
	OriginalConfig    *configuration.Data
	MasterURL         string
	MasterUser        string
	Token             string
	HTTPTransport     http.RoundTripper
	ConsoleURL        string
	LogCallback       LogCallback
	Commit            string
	TemplatesRepo     string
	TemplatesRepoBlob string
	TemplatesRepoDir  string
}

// NewConfig builds openshift config for every user request depending on the user profile
func NewConfig(config *configuration.Data, user *authclient.UserDataAttributes, clusterUser, clusterToken, clusterURL, commit string) Config {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.APIServerInsecureSkipTLSVerify(),
		},
	}

	conf := Config{
		OriginalConfig: config,
		ConsoleURL:     config.GetConsoleURL(),
		HTTPTransport:  tr,
		MasterUser:     clusterUser,
		Token:          clusterToken,
		MasterURL:      clusterURL,
		Commit:         commit,
	}
	return setTemplateRepoInfo(user, conf)
}

// setTemplateRepoInfo returns a new config in which the template repo info set
func setTemplateRepoInfo(user *authclient.UserDataAttributes, config Config) Config {
	if user.FeatureLevel != nil && *user.FeatureLevel != "internal" {
		return config
	}
	userContext := user.ContextInformation
	if tc, found := userContext["tenantConfig"]; found {
		if tenantConfig, ok := tc.(map[string]interface{}); ok {
			find := func(key string) string {
				if rawValue, found := tenantConfig[key]; found {
					if value, ok := rawValue.(string); ok {
						return value
					}
				}
				return ""
			}
			config.TemplatesRepo = find("templatesRepo")
			config.TemplatesRepoBlob = find("templatesRepoBlob")
			config.TemplatesRepoDir = find("templatesRepoDir")
		}
	}
	return config
}

type LogCallback func(message string)

// CreateHTTPClient returns an HTTP client with the options settings,
// or a default HTTP client if nothing was specified
func (c *Config) CreateHTTPClient() *http.Client {
	if c.HTTPTransport != nil {
		return &http.Client{
			Transport: c.HTTPTransport,
		}
	}
	return http.DefaultClient
}

// WithToken returns a new config with an override of the token
func (c Config) WithToken(token string) Config {
	c.Token = token
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
