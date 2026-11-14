package build

import (
	"context"
	"io"
	"testing"
	"time"
)

// StubCommands replaces the command runner of the per-architecture build path and shortens its waits.
func StubCommands(t *testing.T, run func(ctx context.Context, w io.Writer, name string, args ...string) error) {
	t.Helper()

	r, ready, interval, watch, probe := runContext, helperReadyTimeout, helperReadyInterval, helperWatchInterval, helperProbeTimeout
	t.Cleanup(func() {
		runContext, helperReadyTimeout, helperReadyInterval, helperWatchInterval, helperProbeTimeout = r, ready, interval, watch, probe
	})

	runContext = run
	helperReadyTimeout = 100 * time.Millisecond
	helperReadyInterval = 10 * time.Millisecond
	helperWatchInterval = 10 * time.Millisecond
	helperProbeTimeout = 50 * time.Millisecond
}

// SetHelperReadyTimeout changes how long a build waits for its helper, until the StubCommands cleanup runs.
func SetHelperReadyTimeout(d time.Duration) {
	helperReadyTimeout = d
}
