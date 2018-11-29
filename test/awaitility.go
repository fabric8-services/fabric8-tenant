package test

import (
	"context"
	"time"
)

type Awaitility struct {
	timeout time.Duration
}

func WaitWithTimeout(timeout time.Duration) Awaitility {
	return Awaitility{timeout: timeout}
}

func (a Awaitility) Until(do func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	var err error
	for {
		select {
		case <-ctx.Done():
			return err
		case <-time.After(500 * time.Millisecond):
			if err = do(); err == nil {
				return nil
			}
		}
	}
}
