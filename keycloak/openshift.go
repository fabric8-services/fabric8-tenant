package keycloak

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"unsafe"

	yaml "gopkg.in/yaml.v2"
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

// OpenshiftToken fetches the Openshift token defined for the current user in Keycloak
func OpenshiftToken(config Config, token string) (string, error) {
	ut, err := get(config.BrokerTokenURL(), token)
	if err != nil {
		return "", err
	}

	return ut.AccessToken, nil
}

type usertoken struct {
	AccessToken string `yaml:"access_token"`
}

func get(url, token string) (*usertoken, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// for debug only
	rb, _ := httputil.DumpRequest(req, true)
	if false {
		fmt.Println(string(rb))
	}

	client := createHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unknown response:\n%v\n%v", *(*string)(unsafe.Pointer(&b)), string(rb))
	}

	var u usertoken
	err = yaml.Unmarshal(b, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func createHttpClient() *http.Client {
	// when running on minishift there is usually no certs on the HTTPS endpoint for KeyCloak
	// so lets allow host verification to be disabled
	flag := os.Getenv("KEYCLOAK_SKIP_HOST_VERIFY")
	if strings.ToLower(flag) == "true" {
		return &http.Client{
			Transport: &http.Transport{
				// we need to disable TLS verify on minishift
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return http.DefaultClient
}
