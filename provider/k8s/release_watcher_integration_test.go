package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// SCAFFOLDING NOTE — ReleasePromote integration coverage (M-A06-2)
// ----------------------------------------------------------------
// These tests drive the watcher launch + lifecycle through the SAME
// production write-then-watch path that ReleasePromote uses (annotation
// write → goroutine spawn → polling → terminal emit), but they bypass
// the ReleasePromote outer scaffolding because:
//
//   1. The full p.ReleasePromote(app, id, opts) entry path requires
//      AppGet + ReleaseGet (CRD-backed) + manifest.Load + Atom.Apply
//      + releaseTemplateServices + ingress / balancer / RDS / KEDA
//      template generation + a healthy Convox CRD informer cache.
//      The pre-existing TestReleasePromote in release_test.go is
//      `t.Skip()`-ed for this same reason — the fake clientset
//      scaffolding cannot exercise the full Atom Apply chain without
//      wholesale fixture refactoring beyond item-18 scope.
//
//   2. The watcher launch + lifecycle behaviors that the M-A06-2 spec
//      pins (happy-path completed, failure-path errored, supersession
//      cancelled) are 100% in the watcher's polling-state-machine code
//      path. The pre-Apply portion of ReleasePromote is unit-tested by
//      release_scale_override_test.go; the post-Apply emit portion is
//      unit-tested by TestEmitReleasePromoteResult_StatusToActionMapping.
//
// To reduce the "tests stub the helper" gap called out in spec § 10
// row 18-19a WITHOUT triggering a full-fixture refactor, these tests:
//
//   - Compose the same production sequence: writeReleasePromoteWatchAnnotation
//     followed by tryAcquireWatchSlot + go runReleasePromoteWatcher (mirrors
//     release.go:314-339 exactly).
//   - Assert the canonical app:promote:<verb> action names, status codes,
//     payload shape, and slot teardown that the production path produces.
//   - Exercise the cleanup defer's annotation delete, the supersession-
//     aware variant, and the terminal-emit ordering.

// runReleasePromoteFromAnnotation is the integration-test shim that
// mirrors the production sequence in release.go after the Atom Apply
// completes: persist the watch annotation, then launch the watcher
// goroutine which performs its own slot acquire + cleanup.
//
// RunReleasePromoteWatcherForTest internally calls tryAcquireWatchSlot
// and threads the release-fn through the watcher's outer-defer, so we
// do NOT pre-acquire the slot at the integration layer (doing so would
// cause the production-path tryAcquireWatchSlot to return loaded=true
// and skip the goroutine — exactly the singleton invariant production
// code relies on, but here it would mean the watcher never runs).
//
// We synchronously join the goroutine via a wait channel so assertions
// against captured webhook events run only after the cleanup defer
// has emitted its terminal event.
func runReleasePromoteFromAnnotation(
	t *testing.T,
	p *k8s.Provider,
	app string,
	state *structs.ReleasePromoteWatchState,
	timeout time.Duration,
) {
	t.Helper()

	// Persist the annotation FIRST — same ordering as production
	// (release.go:314 writeReleasePromoteWatchAnnotation, then
	// release.go:336 tryAcquireWatchSlot + go runReleasePromoteWatcher).
	require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, state))

	// Launch watcher goroutine — RunReleasePromoteWatcherForTest acquires
	// the slot, drives the polling loop, and the outer-defer releases the
	// slot on exit. Mirrors release.go:336-339 exactly.
	done := make(chan struct{})
	go func() {
		defer close(done)
		k8s.RunReleasePromoteWatcherForTest(p, context.Background(), app, state)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("integration watcher goroutine timed out after %v", timeout)
	}
}

// captureIntegrationEvents installs a webhook capture endpoint on the
// provider and returns all received payloads matching action filter.
// Mirrors the captureReleaseWatcherEvents helper but is local to
// integration tests so the synchronization knobs can differ.
func captureIntegrationEvents(t *testing.T, p *k8s.Provider, run func(), waitFor int, timeout time.Duration) []map[string]any {
	t.Helper()

	var mu sync.Mutex
	var payloads []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var pl map[string]any
		if json.Unmarshal(b, &pl) == nil {
			mu.Lock()
			payloads = append(payloads, pl)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	k8s.SetWebhooksForTest(p, []string{srv.URL})

	run()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(payloads)
		mu.Unlock()
		if n >= waitFor {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	out := make([]map[string]any, len(payloads))
	copy(out, payloads)
	return out
}

// seedIntegrationAppNamespace seeds an app namespace with the given
// `convox.com/app-status` and `convox.com/app-release` annotations.
// Mirrors seedAppNamespaceWithStatus from release_watcher_test.go but
// keeps the integration-test fixture local (avoids cross-file coupling
// when the integration harness needs to evolve independently).
func seedIntegrationAppNamespace(t *testing.T, p *k8s.Provider, app, atomStatus, releaseID string) {
	t.Helper()
	kk, _ := p.Cluster.(*fake.Clientset)
	ns := fmt.Sprintf("%s-%s", p.Name, app)
	annotations := map[string]string{}
	if atomStatus != "" {
		annotations["convox.com/app-status"] = atomStatus
	}
	if releaseID != "" {
		annotations["convox.com/app-release"] = releaseID
	}
	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name:        ns,
			Annotations: annotations,
			Labels: map[string]string{
				"app":    app,
				"name":   app,
				"rack":   p.Name,
				"system": "convox",
				"type":   "app",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

// findIntegrationEvent returns the first event with the given action.
func findIntegrationEvent(events []map[string]any, action string) map[string]any {
	for _, e := range events {
		if a, _ := e["action"].(string); a == action {
			return e
		}
	}
	return nil
}

// TestReleasePromote_FullPath_HappyPath_EmitsCompleted drives the
// production write-then-watch sequence: the namespace's app-status
// resolves to Running, the watcher's polling tick observes it, and the
// cleanup defer emits app:promote:completed Status="success" with the
// canonical action+status+data shape Console3 consumes. The annotation
// is also cleaned up by the supersession-aware delete (matching
// release-id, so delete proceeds).
func TestReleasePromote_FullPath_HappyPath_EmitsCompleted(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "intHappy"
	const releaseID = "R-INT-HAPPY-1"

	testProvider(t, func(p *k8s.Provider) {
		seedIntegrationAppNamespace(t, p, app, "Running", releaseID)

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     releaseID,
			AtomVersion:   releaseID,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureIntegrationEvents(t, p, func() {
			runReleasePromoteFromAnnotation(t, p, app, &state, 2*time.Second)
		}, 1, 2*time.Second)

		ev := findIntegrationEvent(events, "app:promote:completed")
		require.NotNil(t, ev, "production write-then-watch sequence MUST surface app:promote:completed; got %v", events)
		assert.Equal(t, "success", ev["status"])
		data, _ := ev["data"].(map[string]any)
		require.NotNil(t, data, "data payload must be present")
		assert.Equal(t, app, data["app"])
		assert.Equal(t, releaseID, data["id"])
		assert.Equal(t, "alice@example.com", data["actor"])

		// Slot must be released on success path.
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, releaseID),
			"slot MUST be released after happy-path watcher exit")

		// Supersession-aware delete saw matching release-id and removed
		// the annotation — verify it's gone.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"watch annotation MUST be deleted by happy-path cleanup")
	})
}

// TestReleasePromote_FullPath_RolloutFailure_EmitsErrored drives the
// failure path: namespace app-status is Failure, watcher tick observes
// it, terminal emit produces app:promote:errored Status="error" with
// the rollout-failed error message in payload.error.
func TestReleasePromote_FullPath_RolloutFailure_EmitsErrored(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "intFail"
	const releaseID = "R-INT-FAIL-1"

	testProvider(t, func(p *k8s.Provider) {
		seedIntegrationAppNamespace(t, p, app, "Failure", releaseID)

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     releaseID,
			AtomVersion:   releaseID,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureIntegrationEvents(t, p, func() {
			runReleasePromoteFromAnnotation(t, p, app, &state, 2*time.Second)
		}, 1, 2*time.Second)

		ev := findIntegrationEvent(events, "app:promote:errored")
		require.NotNil(t, ev, "rollout failure MUST surface app:promote:errored; got %v", events)
		assert.Equal(t, "error", ev["status"])
		// EventSend writes opts.Error into data.message (event.go:101-104):
		// the canonical event JSON has no top-level error field, so we
		// must read the rollout reason from payload.data.message.
		data, _ := ev["data"].(map[string]any)
		require.NotNil(t, data, "data payload must be present")
		assert.Contains(t, data["message"], "rollout-failed: Failure",
			"errored payload must surface rollout-failed reason; got %q", data["message"])

		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, releaseID),
			"slot MUST be released after errored watcher exit")

		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"watch annotation MUST be deleted on errored cleanup")
	})
}

// TestReleasePromote_FullPath_Supersession_EmitsCancelled exercises the
// supersession path end-to-end: a first watcher launches with R1, a
// second watcher launches with R2 (which overwrites the namespace's
// `convox.com/app-release` mirror), and on the next polling tick the
// first watcher detects the mismatch and emits app:promote:cancelled
// Status="cancelled" with superseded-by-newer-promote in data.message
// (cancelled events route the message into payload.data.message, not
// payload.error, so EventSend does not reclassify the status).
func TestReleasePromote_FullPath_Supersession_EmitsCancelled(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "intSuper"
	const release1 = "R-INT-SUPER-1"
	const release2 = "R-INT-SUPER-2"

	testProvider(t, func(p *k8s.Provider) {
		// Seed the namespace with R2 already in the release annotation
		// — this simulates the AtomController having already mirrored
		// the second promote's release-id by the time the first
		// watcher's polling tick arrives. Same effect as a real
		// supersession: A's state.AtomVersion=R1, but the namespace
		// shows convox.com/app-release=R2.
		seedIntegrationAppNamespace(t, p, app, "Updating", release2)

		// Watcher A's state holds R1.
		stateA := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     release1,
			AtomVersion:   release1,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureIntegrationEvents(t, p, func() {
			runReleasePromoteFromAnnotation(t, p, app, &stateA, 2*time.Second)
		}, 1, 2*time.Second)

		ev := findIntegrationEvent(events, "app:promote:cancelled")
		require.NotNil(t, ev, "supersession MUST surface app:promote:cancelled; got %v", events)
		assert.Equal(t, "cancelled", ev["status"], "cancelled status MUST NOT be reclassified to error")
		data, _ := ev["data"].(map[string]any)
		require.NotNil(t, data)
		assert.Equal(t, release1, data["id"], "cancelled event must reference superseded release-id (R1)")
		assert.Equal(t, "superseded-by-newer-promote", data["message"],
			"cancelled message must be in data.message (not opts.Error)")

		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, release1),
			"slot MUST be released after cancelled watcher exit")

		// Phase H R2 fix (m-A06-2): mirror the happy-path + errored
		// variants' annotation-cleanup assertion. The supersession
		// path drives `deleteReleasePromoteWatchAnnotationIfMatches`
		// with the watcher's own release-id (R1). The watch annotation
		// in the namespace also holds R1 (written by
		// runReleasePromoteFromAnnotation's WriteReleasePromoteWatchAnnotationForTest
		// call), so the predicate matches and the delete proceeds. A
		// regression that wires the wrong expectedReleaseID through
		// the cancelled-cleanup path would leave the annotation in
		// place — this assertion catches that.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"watch annotation MUST be deleted on cancelled cleanup (state R1 matches the watcher's release-id)")
	})
}
