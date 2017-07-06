package openshift

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// GetOrCreateKubeToken will try to load the ServiceAccount for the given user name
// and return its token otherwise if allowed it will lazily create a new ServiceAccount for the username
func GetOrCreateKubeToken(config Config, openshiftUsername string) (string, error) {
	serviceAccountT, err := loadTemplate(config, "fabric8-kubernetes-user-sa.yml")
	if err != nil {
		return "", err
	}
	vars := map[string]string{
		"NAME": openshiftUsername,
	}
	serviceAcccountNamespace := os.Getenv("KUBERNETES_NAMESPACE")
	if serviceAcccountNamespace == "" {
		serviceAcccountNamespace = "fabric8"
	}
	userOpts := ApplyOptions{Config: config, Namespace: serviceAcccountNamespace, Callback: kubeTokenCallback}

	serviceAccountUrl := fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/%s", serviceAcccountNamespace, openshiftUsername)

	sa, err := getResource(config, serviceAccountUrl)
	if err != nil {
		// TODO lets add some check based on the current username in KeyCloak to see if we are allowed to create a new ServiceAccount for them?

		// we maybe don't have a ServiceAccount yet so lets try create it
		err = executeNamespaceSync(string(serviceAccountT), vars, userOpts)
		if err != nil {
			return "", fmt.Errorf("Failed to create a ServiceAccount for user %s due to %v", openshiftUsername, err)
		}
	}
	sa, err = getResource(config, serviceAccountUrl)
	if err != nil {
		return "", fmt.Errorf("Failed to load ServiceAccount %s due to %v", openshiftUsername, err)
	}
	secretName := ""
	secretsArray, ok := sa["secrets"].([]interface{})
	if ok {
		for _, el := range secretsArray {
			m, ok := el.(map[interface{}]interface{})
			if ok {
				name, ok := m["name"].(string)
				if ok && len(name) > 0 {
					secretName = name
					break
				}
			}
		}
	}
	if len(secretName) == 0 {
		return "", fmt.Errorf("Failed to find Secret name in ServiceAccount %s", openshiftUsername)
	}
	secret, err := getResource(config, fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", serviceAcccountNamespace, secretName))
	if err != nil {
		return "", fmt.Errorf("Failed to load Secret %s due to %v", secretName, err)
	}
	token := ""
	data, ok := secret["data"].(map[interface{}]interface{})
	if ok {
		text, ok := data["token"].(string)
		if ok && len(text) > 0 {
			token = text
		}

	}
	if len(token) == 0 {
		return "", fmt.Errorf("No Token found inside Secret %s for ServiceAccount %s", secretName, openshiftUsername)
	}
	bytes, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("Failed to base64 decode the token for secret %s due to %v", secretName, err)
	}
	return string(bytes), nil
}

func getResource(config Config, url string) (map[interface{}]interface{}, error) {
	var body []byte
	fullUrl := strings.TrimSuffix(config.MasterURL, "/") + url
	req, err := http.NewRequest("GET", fullUrl, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/yaml")
	req.Header.Set("Authorization", "Bearer "+config.Token)

	opts := ApplyOptions{Config: config}

	client := opts.CreateHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	status := resp.StatusCode
	if status < 200 || status > 300 {
		return nil, fmt.Errorf("Failed to GET url %s due to status code %d", fullUrl, status)
	}
	var respType map[interface{}]interface{}
	err = yaml.Unmarshal(b, &respType)
	if err != nil {
		return nil, err
	}
	return respType, nil
}

func kubeTokenCallback(statusCode int, method string, request, response map[interface{}]interface{}) (string, map[interface{}]interface{}) {
	//fmt.Printf("CreateKubeToken Got status code %s method %s request %v response %v\n", statusCode, method, request, response)
	return method, response
}
