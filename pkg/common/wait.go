package common

import (
	"context"
	"errors"
	"time"
)

// ErrWaitTimeout lets callers replace the bare "timeout" with something that
// names the app and the commands that help.
var ErrWaitTimeout = errors.New("timeout")

func Wait(interval time.Duration, timeout time.Duration, times int, fn func() (bool, error)) error {
	return WaitContext(context.Background(), interval, timeout, times, fn)
}

func WaitContext(ctx context.Context, interval time.Duration, timeout time.Duration, times int, fn func() (bool, error)) error {
	successes := 0
	failures := 0
	start := time.Now().UTC()

	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
			if start.Add(timeout).Before(time.Now().UTC()) {
				return ErrWaitTimeout
			}

			success, err := fn()
			if err != nil {
				failures += 1
			} else {
				failures = 0
			}

			if failures >= times {
				return err
			}

			if success {
				successes += 1
			} else {
				successes = 0
			}

			if successes >= times {
				return nil
			}
		}
	}
}
