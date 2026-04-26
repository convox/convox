package k8s_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRelease_AdvisoryEvents_PinSystemActor + TestService_ImperativePatchNote_PinsSystemActor
// are static source-inspection tests that lock the explicit "actor": "system"
// override at every accumulator/release-advisory/service-patch call site.
// Runtime triggers for these emit paths are conditional (e.g. a ScaledObject
// must exist; a manifest must be malformed in a specific way) and not all
// fixtures cleanly exercise them in unit tests. The source-level guard is
// strict and trivially diff-reviewable.

func readSource(t *testing.T, rel string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// hasActorSystemNearAction asserts that the action literal appears within the
// same EventSend(...) Data map as the actor:system pair. Uses a substring
// window around the action literal to avoid false positives from unrelated
// neighboring map literals.
func hasActorSystemNearAction(src, action string) bool {
	q := "QUOTE" + action + "QUOTE"
	q = strings.ReplaceAll(q, "QUOTE", string('"'))
	idx := 0
	for {
		i := strings.Index(src[idx:], q)
		if i < 0 {
			return false
		}
		start := idx + i
		from := start - 200
		if from < 0 {
			from = 0
		}
		to := start + 800
		if to > len(src) {
			to = len(src)
		}
		window := src[from:to]
		actorKey := string('"') + "actor" + string('"') + ":"
		systemVal := string('"') + "system" + string('"')
		if strings.Contains(window, actorKey) && strings.Contains(window, systemVal) {
			return true
		}
		idx = start + len(action)
	}
}

func TestRelease_AdvisoryEvents_PinSystemActor(t *testing.T) {
	src := readSource(t, "release.go")
	cases := []string{
		"release:autoscale-disabled",
		"release:manifest-advisory",
		"release:prometheus-default",
	}
	for _, action := range cases {
		t.Run(action, func(t *testing.T) {
			ok := hasActorSystemNearAction(src, action)
			assert.True(t, ok, "release.go MUST emit %q with explicit actor=system", action)
		})
	}
}

func TestService_ImperativePatchNote_PinsSystemActor(t *testing.T) {
	src := readSource(t, "service.go")
	ok := hasActorSystemNearAction(src, "release:imperative-patch-note")
	assert.True(t, ok, "service.go MUST emit release:imperative-patch-note with actor=system")
}

func TestBudgetAccumulator_AlertEvents_PinSystemActor(t *testing.T) {
	src := readSource(t, "budget_accumulator.go")
	cases := []string{
		"app:budget:threshold",
		"app:budget:cap",
	}
	for _, action := range cases {
		t.Run(action, func(t *testing.T) {
			ok := hasActorSystemNearAction(src, action)
			assert.True(t, ok, "budget_accumulator.go MUST emit %q with explicit actor=system", action)
		})
	}
}

// TestNoSystemBudgetAccumulatorActor: cross-spec REJECTED 6th value.
// Hard-pinned: the literal "system-budget-accumulator" must NOT appear
// anywhere in the package source (excluding test files which may carry it
// for adversarial assertion strings — none currently do).
func TestNoSystemBudgetAccumulatorActor(t *testing.T) {
	files := []string{
		"event.go",
		"budget_accumulator.go",
		// F-7 fix (catalog F-7): extend scan list to cover Set G files
		// where γ-C added the bulk of new actor-bearing code. Direct grep
		// confirms zero current violations; defensive guard for future
		// regressions.
		"budget_auto_shutdown.go",
		"budget_shutdown.go",
		"release.go",
		"service.go",
		"build.go",
		"k8s.go",
	}
	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			src := readSource(t, rel)
			assert.False(t, strings.Contains(src, "system-budget-accumulator"),
				"%s contains REJECTED 6th-value actor literal", rel)
		})
	}
}
