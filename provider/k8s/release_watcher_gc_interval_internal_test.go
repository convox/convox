package k8s

import (
	"testing"
	"time"
)

// Internal tests for applyReleaseWatcherGCIntervalEnv. Helper is package-
// private and mutates the package-level releasePromoteWatchGCTickInterval
// var, so the tests live in package k8s rather than k8s_test.
//
// Operators set the
// `release_watcher_gc_interval` rack param; the TF module plumbs it as
// `RELEASE_WATCHER_GC_INTERVAL` env var on the api Deployment; the
// provider reads it ONCE at Initialize and assigns to the package-level
// var. Range 60s-1h. Out-of-range clamps; invalid parses fall back to
// the existing default with a warn log.
//
// Each test must save/restore releasePromoteWatchGCTickInterval so a
// failure or panic doesn't leak across tests. t.Setenv handles env-var
// restore automatically; the var save/restore is manual.

// withGCIntervalReset captures the current package-level var and
// restores it via t.Cleanup so each test starts from a known baseline
// and a panic in mid-test doesn't taint subsequent tests.
func withGCIntervalReset(t *testing.T) {
	t.Helper()
	prev := releasePromoteWatchGCTickInterval
	t.Cleanup(func() { releasePromoteWatchGCTickInterval = prev })
}

// TestReleaseWatcherGCIntervalEnvVar_Default — env unset OR empty:
// helper returns false; package var unchanged from production default
// (5m). This is the no-op path that production must hit when an
// operator hasn't set the rack param.
func TestReleaseWatcherGCIntervalEnvVar_Default(t *testing.T) {
	withGCIntervalReset(t)
	t.Setenv("RELEASE_WATCHER_GC_INTERVAL", "")
	// Pin to a known value so we can assert no mutation.
	releasePromoteWatchGCTickInterval = 5 * time.Minute

	if got := applyReleaseWatcherGCIntervalEnv(); got {
		t.Errorf("applyReleaseWatcherGCIntervalEnv() = true; want false (empty env must be a no-op)")
	}
	if releasePromoteWatchGCTickInterval != 5*time.Minute {
		t.Errorf("releasePromoteWatchGCTickInterval = %s; want 5m (empty env must not mutate)",
			releasePromoteWatchGCTickInterval)
	}
}

// TestReleaseWatcherGCIntervalEnvVar_ValidApplied — env set to a
// duration in-range: helper assigns it verbatim. Verifies the happy
// path operators hit when they set 2m, 10m, 30m, etc.
func TestReleaseWatcherGCIntervalEnvVar_ValidApplied(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want time.Duration
	}{
		{name: "lower-edge_60s", env: "60s", want: 60 * time.Second},
		{name: "midrange_2m", env: "2m", want: 2 * time.Minute},
		{name: "midrange_10m", env: "10m", want: 10 * time.Minute},
		{name: "upper-edge_1h", env: "1h", want: 1 * time.Hour},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withGCIntervalReset(t)
			t.Setenv("RELEASE_WATCHER_GC_INTERVAL", tc.env)
			releasePromoteWatchGCTickInterval = 5 * time.Minute // pin baseline

			if got := applyReleaseWatcherGCIntervalEnv(); !got {
				t.Errorf("applyReleaseWatcherGCIntervalEnv() = false; want true (non-empty env must signal observation)")
			}
			if releasePromoteWatchGCTickInterval != tc.want {
				t.Errorf("releasePromoteWatchGCTickInterval = %s; want %s",
					releasePromoteWatchGCTickInterval, tc.want)
			}
		})
	}
}

// TestReleaseWatcherGCIntervalEnvVar_BelowMin_Clamps verifies that an
// env var set to a value below the 60s lower bound clamps UP to 60s.
// A sub-60s sweep at 200 apps inflates apiserver QPS without
// observable UX benefit; steady-state watchers already poll at 3s
// per-app for in-flight promotes.
func TestReleaseWatcherGCIntervalEnvVar_BelowMin_Clamps(t *testing.T) {
	cases := []struct {
		name string
		env  string
	}{
		{name: "30s", env: "30s"},
		{name: "10s", env: "10s"},
		{name: "1s", env: "1s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withGCIntervalReset(t)
			t.Setenv("RELEASE_WATCHER_GC_INTERVAL", tc.env)
			releasePromoteWatchGCTickInterval = 5 * time.Minute // pin baseline

			if got := applyReleaseWatcherGCIntervalEnv(); !got {
				t.Errorf("applyReleaseWatcherGCIntervalEnv() = false; want true (env was set, must signal observation even when clamped)")
			}
			if releasePromoteWatchGCTickInterval != 60*time.Second {
				t.Errorf("releasePromoteWatchGCTickInterval = %s; want 60s (sub-min input must clamp UP to lower bound)",
					releasePromoteWatchGCTickInterval)
			}
		})
	}
}

// TestReleaseWatcherGCIntervalEnvVar_AboveMax_Clamps — env set to a
// value above the 1h upper bound: helper clamps DOWN to 1h. Operators
// who try to set 24h get 1h, with a warn log so they see the override.
func TestReleaseWatcherGCIntervalEnvVar_AboveMax_Clamps(t *testing.T) {
	cases := []struct {
		name string
		env  string
	}{
		{name: "2h", env: "2h"},
		{name: "24h", env: "24h"},
		{name: "100h", env: "100h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withGCIntervalReset(t)
			t.Setenv("RELEASE_WATCHER_GC_INTERVAL", tc.env)
			releasePromoteWatchGCTickInterval = 5 * time.Minute // pin baseline

			if got := applyReleaseWatcherGCIntervalEnv(); !got {
				t.Errorf("applyReleaseWatcherGCIntervalEnv() = false; want true (env was set, must signal observation even when clamped)")
			}
			if releasePromoteWatchGCTickInterval != 1*time.Hour {
				t.Errorf("releasePromoteWatchGCTickInterval = %s; want 1h (above-max input must clamp DOWN to upper bound)",
					releasePromoteWatchGCTickInterval)
			}
		})
	}
}

// TestReleaseWatcherGCIntervalEnvVar_InvalidString_FallsBackToDefault —
// env set to something that ParseDuration can't parse: helper leaves
// the existing value untouched and emits a warn log (visible in
// api-pod stdout). Operator typos and copy-paste errors fall through
// safely without crashing the api pod.
func TestReleaseWatcherGCIntervalEnvVar_InvalidString_FallsBackToDefault(t *testing.T) {
	cases := []string{"abc", "5", "five-minutes", "60", "5m30", "not-a-duration"}
	for _, env := range cases {
		t.Run(env, func(t *testing.T) {
			withGCIntervalReset(t)
			t.Setenv("RELEASE_WATCHER_GC_INTERVAL", env)
			// Pin to default 5m so we can assert it stays put.
			releasePromoteWatchGCTickInterval = 5 * time.Minute

			if got := applyReleaseWatcherGCIntervalEnv(); !got {
				t.Errorf("applyReleaseWatcherGCIntervalEnv() = false; want true (env was set, must signal observation even when rejected)")
			}
			if releasePromoteWatchGCTickInterval != 5*time.Minute {
				t.Errorf("releasePromoteWatchGCTickInterval = %s; want 5m (invalid env must not mutate the package var)",
					releasePromoteWatchGCTickInterval)
			}
		})
	}
}

// TestReleaseWatcherGCIntervalEnvVar_NegativeDuration_ClampsToMin —
// explicit pin of the negative-duration semantics. ParseDuration
// accepts "-5m"; our clamp branch then routes it to the lower-bound.
// This is the documented behavior so a typo of "-5m" doesn't
// produce a non-tick'ing watcher.
func TestReleaseWatcherGCIntervalEnvVar_NegativeDuration_ClampsToMin(t *testing.T) {
	withGCIntervalReset(t)
	t.Setenv("RELEASE_WATCHER_GC_INTERVAL", "-5m")
	releasePromoteWatchGCTickInterval = 5 * time.Minute

	if got := applyReleaseWatcherGCIntervalEnv(); !got {
		t.Errorf("applyReleaseWatcherGCIntervalEnv() = false; want true")
	}
	if releasePromoteWatchGCTickInterval != 60*time.Second {
		t.Errorf("releasePromoteWatchGCTickInterval = %s; want 60s (negative duration must clamp to lower bound, not produce non-tick'ing watcher)",
			releasePromoteWatchGCTickInterval)
	}
}

// TestReleaseWatcherGCIntervalEnvVar_Idempotent — calling helper
// twice with the same env produces the same result both times.
// Defense-in-depth: even if Initialize were called twice (it
// shouldn't be in production), the package var stays at the same
// value rather than drifting.
func TestReleaseWatcherGCIntervalEnvVar_Idempotent(t *testing.T) {
	withGCIntervalReset(t)
	t.Setenv("RELEASE_WATCHER_GC_INTERVAL", "2m")
	releasePromoteWatchGCTickInterval = 5 * time.Minute

	_ = applyReleaseWatcherGCIntervalEnv()
	first := releasePromoteWatchGCTickInterval
	_ = applyReleaseWatcherGCIntervalEnv()
	second := releasePromoteWatchGCTickInterval

	if first != second {
		t.Errorf("idempotent invocation drifted: first=%s second=%s", first, second)
	}
	if first != 2*time.Minute {
		t.Errorf("first invocation = %s; want 2m", first)
	}
}
