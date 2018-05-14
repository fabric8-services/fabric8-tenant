package configuration

import (
	"crypto/tls"
	"net/http"
)

// HTTPClientOption options passed to the HTTP Client
type HTTPClientOption func(client *http.Client)

// WithRoundTripper configures the client's transport with the given round-tripper
func WithRoundTripper(r http.RoundTripper) HTTPClientOption {
	return func(client *http.Client) {
		client.Transport = r
	}
}

func WithInsecureSkipTLSVerify() HTTPClientOption {
	var insecureSkipVerify bool

	config, err := NewData()
	if err == nil {
		insecureSkipVerify = config.APIServerInsecureSkipTLSVerify()
	}

	return func(client *http.Client) {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify,
			},
		}
	}
}
