package testdoubles

import (
	vcrrecorder "github.com/dnaeon/go-vcr/recorder"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
)

func LoadTestConfig() (*configuration.Data, error) {
	data, err := configuration.GetData()
	return data, err
}

func NewAuthClientService(cassetteFile, authURL string, recorderOptions ...recorder.Option) (*auth.Service, *vcrrecorder.Recorder, error) {
	var options []configuration.HTTPClientOption
	var r *vcrrecorder.Recorder
	var err error
	if cassetteFile != "" {
		r, err = recorder.New(cassetteFile, recorderOptions...)
		if err != nil {
			return nil, r, err
		}
		options = append(options, configuration.WithRoundTripper(r))
	}
	config, err := LoadTestConfig()
	if err != nil {
		return nil, r, err
	}
	config.Set("auth.url", authURL)
	authService := &auth.Service{
		Config:        config,
		ClientOptions: options,
	}
	return authService, r, nil
}
