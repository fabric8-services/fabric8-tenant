package testdoubles

import (
	vcrRecorder "github.com/dnaeon/go-vcr/recorder"
	commonConfig "github.com/fabric8-services/fabric8-common/configuration"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
)

func LoadTestConfig() (*configuration.Data, error) {
	data, err := configuration.GetData()
	return data, err
}

func NewAuthClientService(cassetteFile, authURL string, recorderOptions ...recorder.Option) (*auth.Service, *vcrRecorder.Recorder, error) {
	var options []commonConfig.HTTPClientOption
	var r *vcrRecorder.Recorder
	var err error
	if cassetteFile != "" {
		r, err = recorder.New(cassetteFile, recorderOptions...)
		if err != nil {
			return nil, r, err
		}
		options = append(options, commonConfig.WithRoundTripper(r))
	}
	config, err := LoadTestConfig()
	if err != nil {
		return nil, r, err
	}
	config.Set(configuration.VarAuthURL, authURL)
	authService := &auth.Service{
		Config:        config,
		ClientOptions: options,
	}
	return authService, r, nil
}
