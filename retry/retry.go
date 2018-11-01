package retry

import (
	"time"
)

// RetryFunc is a function type which wraps actual logic to be retried and returns error if that needs to happen
type RetryFunc func() error // nolint: golint

// Do invokes a function and if invocation fails retries defined amount of time with sleep in between
// Returns accumulated errors if all attempts failed or empty slice otherwise
func Do(retries int, sleep time.Duration, toRetry RetryFunc) []error {
	errs := make([]error, 0, retries)

	err := toRetry()

	for i := 0; i < retries-1 && err != nil; i++ {
		errs = append(errs, err)
		time.Sleep(sleep)
		err = toRetry()
	}

	if err != nil {
		errs = append(errs, err)
		return errs
	}

	return make([]error, 0)
}
