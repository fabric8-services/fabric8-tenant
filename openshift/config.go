package openshift

import (
	"fmt"
	"net/http"
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

type LogCallback func(message string)

func (c Config) WithToken(token string) Config {
	return Config{MasterURL: c.MasterURL, MasterUser: c.MasterUser, Token: token, HttpTransport: c.HttpTransport, TemplateDir: c.TemplateDir, MavenRepoURL: c.MavenRepoURL, TeamVersion: c.TeamVersion}
}

// WithUserSettings overrides the user settings with the given values (if not nil).
func (c Config) WithUserSettings(cheVersion, jenkinsVersion, teamVersion, mavenRepoURL *string) Config {
	copy := c
	if cheVersion != nil {
		copy.CheVersion = *cheVersion
	}
	if jenkinsVersion != nil {
		copy.JenkinsVersion = *jenkinsVersion
	}
	if teamVersion != nil {
		copy.TeamVersion = *teamVersion
	}
	if mavenRepoURL != nil {
		copy.MavenRepoURL = *mavenRepoURL
	}
	return copy
}

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
