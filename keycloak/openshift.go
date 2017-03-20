package keycloak

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"unsafe"

	yaml "gopkg.in/yaml.v2"
)

// Config contains basic configuration data for Keycloak
type Config struct {
	BaseURL string
	Realm   string
	Broker  string
}

// OpenshiftToken fetches the Openshift token defined for the current user in Keycloak
func OpenshiftToken(config Config, token string) (string, error) {
	tokenURL := fmt.Sprintf("%v/auth/realms/%v/broker/%v/token", config.BaseURL, config.Realm, config.Broker)
	ut, err := get(tokenURL, token)
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
	req.Header.Set("Accept", "application/yaml")
	req.Header.Set("Content-Type", "application/yaml")
	req.Header.Set("Authorization", "Bearer "+token)

	// for debug only
	rb, _ := httputil.DumpRequest(req, true)
	if false {
		fmt.Println(string(rb))
	}

	client := http.DefaultClient
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
