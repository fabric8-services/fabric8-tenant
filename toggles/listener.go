package toggles

import (
	unleash "github.com/Unleash/unleash-client-go"
	"github.com/fabric8-services/fabric8-wit/log"
)

type clientListener struct {
	client *Client
}

// OnError prints out errors.
func (l clientListener) OnError(err error) {
	log.Error(nil, map[string]interface{}{
		"err": err.Error(),
	}, "toggles error")
}

// OnWarning prints out warning.
func (l clientListener) OnWarning(warning error) {
	log.Warn(nil, map[string]interface{}{
		"err": warning.Error(),
	}, "toggles warning")
}

// OnReady prints to the console when the repository is ready.
func (l clientListener) OnReady() {
	l.client.ready = true
	log.Info(nil, map[string]interface{}{}, "toggles ready")
}

// OnCount prints to the console when the feature is queried.
func (l clientListener) OnCount(name string, enabled bool) {
	log.Info(nil, map[string]interface{}{
		"name":    name,
		"enabled": enabled,
	}, "toggles count")
}

// OnSent prints to the console when the server has uploaded metrics.
func (l clientListener) OnSent(payload unleash.MetricsData) {
	log.Info(nil, map[string]interface{}{
		"payload": payload,
	}, "toggles sent")
}

// OnRegistered prints to the console when the client has registered.
func (l clientListener) OnRegistered(payload unleash.ClientData) {
	log.Info(nil, map[string]interface{}{
		"payload": payload,
	}, "toggles registered")
}
