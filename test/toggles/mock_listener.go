package toggles

import "github.com/fabric8-services/fabric8-wit/log"

type MockListener struct {
	ready bool
}

// OnReady prints to the console when the repository is ready.
func (l MockListener) OnReady() {
	l.ready = true
	log.Info(nil, map[string]interface{}{}, "toggles ready")
}
