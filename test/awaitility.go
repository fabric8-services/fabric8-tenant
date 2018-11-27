package test

import (
	"time"
)

type Awaitility struct {
	timeout time.Duration
	before  time.Time
}

func WaitWithTimeout(timeout time.Duration) Awaitility {
	return Awaitility{timeout: timeout, before: time.Now()}
}

func (a Awaitility) Until(do func() error) error {
	err := do()
	if err == nil {
		return nil
	} else if a.before.Before(time.Now().Add(-a.timeout)) {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	return a.Until(do)
}
