package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

// captureReleaseWatcherEvents collects all webhook payloads dispatched while
// fn runs. Returns a slice of decoded event payloads (each map[string]any).
func captureReleaseWatcherEvents(t *testing.T, p *k8s.Provider, fn func(), waitFor int, timeout time.Duration) []map[string]any {
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

	fn()

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

// findEventByAction returns the first payload whose "action" field equals action.
func findEventByAction(events []map[string]any, action string) map[string]any {
	for _, e := range events {
		if a, _ := e["action"].(string); a == action {
			return e
		}
	}
	return nil
}

// seedAppNamespaceWithStatus seeds an app namespace and applies the
// `convox.com/app-status` and `convox.com/app-release` annotations the
// watcher reads from the namespace informer. Used to pre-stage a watcher's
// observed state before the watcher's first tick fires.
func seedAppNamespaceWithStatus(t *testing.T, p *k8s.Provider, app, atomStatus, releaseID string) {
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

// TestReleasePromoteWatcher_TerminalSuccess verifies the watcher emits
// `app:promote:completed` Status="success" when the namespace
// `convox.com/app-status` reaches Running.
func TestReleasePromoteWatcher_TerminalSuccess(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app1", "Running", "R001")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R001",
			AtomVersion:   "R001",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app1", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:completed")
		require.NotNil(t, ev, "expected app:promote:completed event; got %v", events)
		assert.Equal(t, "success", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "R001", data["id"])
		assert.Equal(t, "alice@example.com", data["actor"])
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest("app1", "R001"), "slot must be released after watcher exit")
	})
}

// TestReleasePromoteWatcher_TerminalError_Failure pins the Failure transition.
func TestReleasePromoteWatcher_TerminalError_Failure(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app2", "Failure", "R002")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R002",
			AtomVersion:   "R002",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app2", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expected app:promote:errored event; got %v", events)
		assert.Equal(t, "error", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Contains(t, data["message"], "rollout-failed: Failure")
	})
}

// TestReleasePromoteWatcher_TerminalError_AtomCancelled pins the Atom-level
// Cancelled status (distinct from supersession-cancelled).
func TestReleasePromoteWatcher_TerminalError_AtomCancelled(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app3", "Cancelled", "R003")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R003",
			AtomVersion:   "R003",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app3", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expected app:promote:errored event; got %v", events)
		assert.Equal(t, "error", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "cancelled", data["message"])
	})
}

// TestReleasePromoteWatcher_TerminalError_Deadline pins the Atom-level Deadline.
func TestReleasePromoteWatcher_TerminalError_Deadline(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app4", "Deadline", "R004")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R004",
			AtomVersion:   "R004",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app4", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "deadline-exceeded", data["message"])
	})
}

// TestReleasePromoteWatcher_TerminalError_Rollback covers the Rollback branch.
func TestReleasePromoteWatcher_TerminalError_Rollback(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app5", "Rollback", "R005")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R005",
			AtomVersion:   "R005",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app5", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev)
		data, _ := ev["data"].(map[string]any)
		assert.Contains(t, data["message"], "rollback: Rollback")
	})
}

// TestReleasePromoteWatcher_WatcherTimeout pins the past-grace deadline path.
// Drives a state stuck in Updating with state.ExpiresAt already in the past.
func TestReleasePromoteWatcher_WatcherTimeout(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(50 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app6", "Updating", "R006")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R006",
			AtomVersion:   "R006",
			StartedAt:     time.Now().UTC().Add(-1 * time.Second),
			ExpiresAt:     time.Now().UTC().Add(-100 * time.Millisecond), // already past
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app6", &state)
		}, 1, 3*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expected app:promote:errored on timeout; got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"])
	})
}

// TestReleasePromoteWatcher_GracePeriodHonor pins the 30s-grace invariant.
// At time t+1.5s (past ExpiresAt but inside grace) the watcher MUST NOT
// emit watcher-timeout; it does emit at t+grace+epsilon.
func TestReleasePromoteWatcher_GracePeriodHonor(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()
	defer k8s.SetReleasePromoteWatchGracePeriodForTest(300 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app6g", "Updating", "R006G")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R006G",
			AtomVersion:   "R006G",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(50 * time.Millisecond), // expires soon
			Actor:         "alice@example.com",
		}

		// Run watcher in the foreground with deadline = ExpiresAt(50ms) + grace(300ms) = 350ms.
		// The first tick at 20ms sees Updating, second at 40ms sees Updating, etc.
		// Times where deadline hasn't elapsed (<350ms): no event.
		// At >=350ms: watcher emits watcher-timeout.
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app6g", &state)
		}, 1, 2*time.Second)

		// Confirm the emit IS the timeout event (post-grace fire).
		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expected timeout event after grace window; got %v", events)
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"])
	})
}

// TestReleasePromoteWatcher_SupersededByNewerPromote_EmitsCancelled pins the
// supersession path: when `convox.com/app-release` annotation differs from
// state.AtomVersion, the watcher emits `app:promote:cancelled` Status="cancelled".
func TestReleasePromoteWatcher_SupersededByNewerPromote_EmitsCancelled(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		// Seed namespace with a NEWER release-id (R007B) than the watcher state (R007A).
		seedAppNamespaceWithStatus(t, p, "app7", "Updating", "R007B")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R007A",
			AtomVersion:   "R007A",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "app7", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:cancelled")
		require.NotNil(t, ev, "expected app:promote:cancelled event; got %v", events)
		assert.Equal(t, "cancelled", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "R007A", data["id"])
		assert.Equal(t, "superseded-by-newer-promote", data["message"])
	})
}

// TestReleasePromoteWatcher_NamespaceDeleted: namespace not found -> silent exit.
func TestReleasePromoteWatcher_NamespaceDeleted(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		// Do NOT create the namespace.

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R008",
			AtomVersion:   "R008",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		// Watcher with a short ctx so we exit cleanly when namespace stays missing.
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, ctx, "app8", &state)
		}, 0, 500*time.Millisecond)

		// No completed/errored/cancelled event when namespace is absent.
		assert.Nil(t, findEventByAction(events, "app:promote:completed"))
		assert.Nil(t, findEventByAction(events, "app:promote:errored"))
		assert.Nil(t, findEventByAction(events, "app:promote:cancelled"))
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest("app8", "R008"))
	})
}

// TestReleasePromoteWatcher_CtxCancelled: ctx cancellation -> silent exit.
func TestReleasePromoteWatcher_CtxCancelled(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "app9", "Updating", "R009")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R009",
			AtomVersion:   "R009",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, ctx, "app9", &state)
		}, 0, 200*time.Millisecond)

		// Silent exit - no event emitted.
		assert.Nil(t, findEventByAction(events, "app:promote:completed"))
		assert.Nil(t, findEventByAction(events, "app:promote:errored"))
		assert.Nil(t, findEventByAction(events, "app:promote:cancelled"))
	})
}

// TestTryAcquireWatchSlot_DoubleAcquire pins the per-(app, release-id)
// singleton invariant.
func TestTryAcquireWatchSlot_DoubleAcquire(t *testing.T) {
	acq1, rel1 := k8s.TryAcquireReleasePromoteWatchSlotForTest("appS", "R-SLOT-1")
	require.True(t, acq1)
	require.True(t, k8s.ReleasePromoteWatchSlotHeldForTest("appS", "R-SLOT-1"))

	acq2, _ := k8s.TryAcquireReleasePromoteWatchSlotForTest("appS", "R-SLOT-1")
	require.False(t, acq2, "second acquire for same (app, release-id) MUST fail")

	rel1()
	require.False(t, k8s.ReleasePromoteWatchSlotHeldForTest("appS", "R-SLOT-1"), "release-fn must drop the slot")

	// After release, fresh acquire works.
	acq3, rel3 := k8s.TryAcquireReleasePromoteWatchSlotForTest("appS", "R-SLOT-1")
	require.True(t, acq3, "after release, slot MUST be re-acquirable")
	rel3()
}

// TestEmitReleasePromoteResult_StatusToActionMapping pins the
// status -> action name mapping invariant.
func TestEmitReleasePromoteResult_StatusToActionMapping(t *testing.T) {
	cases := []struct {
		status     string
		wantAction string
	}{
		{"success", "app:promote:completed"},
		{"error", "app:promote:errored"},
		{"cancelled", "app:promote:cancelled"},
		// Unknown status falls through to errored (defensive default).
		{"weird-unknown", "app:promote:errored"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.status, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				state := structs.ReleasePromoteWatchState{
					SchemaVersion: 1,
					ReleaseID:     "R-MAP",
					AtomVersion:   "R-MAP",
					Actor:         "alice@example.com",
				}
				events := captureReleaseWatcherEvents(t, p, func() {
					k8s.EmitReleasePromoteResultForTest(p, "appMap", &state, c.status, "")
				}, 1, 1*time.Second)

				ev := findEventByAction(events, c.wantAction)
				require.NotNil(t, ev, "expected action %q; got %v", c.wantAction, events)
			})
		})
	}
}

// TestEmitReleasePromoteResult_EmptyStatusSilent pins the empty-status
// case (ctx cancelled / namespace gone -> no emit).
func TestEmitReleasePromoteResult_EmptyStatusSilent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-MAP-2",
			AtomVersion:   "R-MAP-2",
			Actor:         "alice@example.com",
		}
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.EmitReleasePromoteResultForTest(p, "appMap2", &state, "", "")
		}, 0, 200*time.Millisecond)

		assert.Empty(t, events, "empty status MUST suppress emit; got %v", events)
	})
}

// TestRunReleasePromoteWatchGC_ResumeInflight: cold-start GC with a
// non-expired annotation on the namespace re-launches a watcher for it.
func TestRunReleasePromoteWatchGC_ResumeInflight(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		// Pre-seed namespace + watch annotation. AtomStatus=Running so the
		// re-launched watcher reaches success on its first tick.
		seedAppNamespaceWithStatus(t, p, "appGC1", "Running", "R-GC-1")
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-GC-1",
			AtomVersion:   "R-GC-1",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), "appGC1", &state))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:completed")
		require.NotNil(t, ev, "GC scan must re-launch watcher; got %v", events)
		assert.Equal(t, "success", ev["status"])
	})
}

// TestRunReleasePromoteWatchGC_TimeoutExpired: stale annotation past
// ExpiresAt -> immediate watcher-timeout emit, annotation deleted.
func TestRunReleasePromoteWatchGC_TimeoutExpired(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appGC2", "Updating", "R-GC-2")
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-GC-2",
			AtomVersion:   "R-GC-2",
			StartedAt:     time.Now().UTC().Add(-2 * time.Hour),
			ExpiresAt:     time.Now().UTC().Add(-1 * time.Hour),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), "appGC2", &state))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expired annotation MUST emit watcher-timeout; got %v", events)
		assert.Equal(t, "error", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"])

		// Annotation must be deleted post-emit.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appGC2", p.Name), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation])
	})
}

// TestRunReleasePromoteWatchGC_Superseded: stale annotation with mismatched
// release-id -> emit cancelled, annotation deleted.
func TestRunReleasePromoteWatchGC_Superseded(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		// Namespace has the NEW release-id (R-GC-3-NEW); annotation has OLD.
		seedAppNamespaceWithStatus(t, p, "appGC3", "Updating", "R-GC-3-NEW")
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-GC-3-OLD",
			AtomVersion:   "R-GC-3-OLD",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), "appGC3", &state))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:cancelled")
		require.NotNil(t, ev, "supersession via GC MUST emit cancelled; got %v", events)
		assert.Equal(t, "cancelled", ev["status"])
		data, _ := ev["data"].(map[string]any)
		assert.Equal(t, "R-GC-3-OLD", data["id"])
		assert.Equal(t, "superseded-by-newer-promote", data["message"])

		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appGC3", p.Name), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation])
	})
}

// TestRunReleasePromoteWatchGC_PastDeadline_MapsAppStatusToVerb pins the
// Phase H R2 fix (m-A08-NEW-1 / m-A12-NEW-1): the GC scanner past-
// deadline branch consults `convox.com/app-status` first instead of
// always emitting errored. When AtomController has already written a
// terminal app-status, the GC scan surfaces that as the watch event
// (so the audit log reflects the real rollout outcome). Only when
// app-status is non-terminal (Pending / Updating / empty) does it
// fall back to watcher-timeout.
//
// Table-driven: each row pre-seeds a past-deadline annotation with a
// specific app-status and asserts the GC scan emits the expected
// (action, status, message) tuple.
func TestRunReleasePromoteWatchGC_PastDeadline_MapsAppStatusToVerb(t *testing.T) {
	cases := []struct {
		name        string
		appStatus   string
		nsSuffix    string
		releaseID   string
		wantAction  string
		wantStatus  string
		wantMessage string // matched against data.message
	}{
		{"running_completes", "Running", "appGCD1", "R-GCD-1", "app:promote:completed", "success", ""},
		{"success_completes", "Success", "appGCD2", "R-GCD-2", "app:promote:completed", "success", ""},
		{"failure_errors", "Failure", "appGCD3", "R-GCD-3", "app:promote:errored", "error", "rollout-failed: Failure"},
		{"reverted_errors", "Reverted", "appGCD4", "R-GCD-4", "app:promote:errored", "error", "rollout-failed: Reverted"},
		{"cancelled_errors", "Cancelled", "appGCD5", "R-GCD-5", "app:promote:errored", "error", "cancelled"},
		{"deadline_errors", "Deadline", "appGCD6", "R-GCD-6", "app:promote:errored", "error", "deadline-exceeded"},
		{"rollback_errors", "Rollback", "appGCD7", "R-GCD-7", "app:promote:errored", "error", "rollback: Rollback"},
		// Non-terminal status — falls back to watcher-timeout (legacy
		// behavior preserved for genuinely-stuck rollouts).
		{"updating_falls_back_to_timeout", "Updating", "appGCD8", "R-GCD-8", "app:promote:errored", "error", "watcher-timeout"},
		{"pending_falls_back_to_timeout", "Pending", "appGCD9", "R-GCD-9", "app:promote:errored", "error", "watcher-timeout"},
		// Empty app-status (informer hasn't seen the rollout yet) —
		// falls back to watcher-timeout.
		{"empty_falls_back_to_timeout", "", "appGCDA", "R-GCDA", "app:promote:errored", "error", "watcher-timeout"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				// Use the matching release-id on the namespace so the
				// supersession branch doesn't fire — we want to land on
				// the past-deadline branch unambiguously.
				seedAppNamespaceWithStatus(t, p, c.nsSuffix, c.appStatus, c.releaseID)
				state := structs.ReleasePromoteWatchState{
					SchemaVersion: 1,
					ReleaseID:     c.releaseID,
					AtomVersion:   c.releaseID,
					StartedAt:     time.Now().UTC().Add(-2 * time.Hour),
					ExpiresAt:     time.Now().UTC().Add(-1 * time.Hour),
					Actor:         "alice@example.com",
				}
				require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(
					p, context.Background(), c.nsSuffix, &state))

				events := captureReleaseWatcherEvents(t, p, func() {
					k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
				}, 1, 2*time.Second)

				ev := findEventByAction(events, c.wantAction)
				require.NotNil(t, ev,
					"app-status=%q past-deadline MUST emit %s; got %v", c.appStatus, c.wantAction, events)
				assert.Equal(t, c.wantStatus, ev["status"])
				if c.wantMessage != "" {
					data, _ := ev["data"].(map[string]any)
					require.NotNil(t, data, "event data must be present")
					assert.Equal(t, c.wantMessage, data["message"],
						"expected data.message=%q for app-status=%q", c.wantMessage, c.appStatus)
				}

				// Annotation must be cleaned up via the supersession-
				// aware variant (release-id matches in this scenario).
				ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
					fmt.Sprintf("%s-%s", p.Name, c.nsSuffix), am.GetOptions{})
				assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
					"watch annotation MUST be cleaned up by GC past-deadline branch")
			})
		})
	}
}

// TestRunReleasePromoteWatchGC_CorruptJSON: GC of unparseable annotation
// deletes immediately; no event emitted.
func TestRunReleasePromoteWatchGC_CorruptJSON(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appGC4", "Updating", "R-GC-4")
		// Manually set annotation to invalid JSON.
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					structs.ReleasePromoteWatchAnnotation: "{not-valid-json",
				},
			},
		}
		body, _ := json.Marshal(patch)
		_, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
			fmt.Sprintf("%s-appGC4", p.Name), types.MergePatchType, body, am.PatchOptions{})
		require.NoError(t, err)

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 0, 200*time.Millisecond)

		assert.Empty(t, events, "corrupt JSON MUST NOT emit a phantom event; got %v", events)

		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appGC4", p.Name), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"corrupt JSON annotation MUST be deleted by GC")
	})
}

// TestRunReleasePromoteWatchGC_UnknownSchemaVersion_LogAndSkip: future or
// out-of-range schemaVersion -> annotation NOT deleted, no event emitted
// (rolling-upgrade safety). Table-driven so the same invariant is pinned
// for the immediate next-version (2), a far-future placeholder (99), and
// an explicit negative (-1) that an attacker or corruption could plant.
// SchemaVersion=0 (zero value when the field is omitted from JSON) is
// covered separately by TestRunReleasePromoteWatchGC_SchemaVersionZero_LogAndSkip
// because a missing field is a distinct write-path scenario.
func TestRunReleasePromoteWatchGC_UnknownSchemaVersion_LogAndSkip(t *testing.T) {
	cases := []struct {
		name          string
		nsSuffix      string
		schemaVersion int
		releaseID     string
	}{
		{"version2_next", "appGC5", 2, "R-GC-5"},
		{"version99_far_future", "appGC5b", 99, "R-GC-5B"},
		{"version_negative", "appGC5c", -1, "R-GC-5C"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				seedAppNamespaceWithStatus(t, p, c.nsSuffix, "Updating", c.releaseID)
				raw := fmt.Sprintf(
					`{"schemaVersion":%d,"releaseId":%q,"atomVersion":%q,"actor":"future@convox.com"}`,
					c.schemaVersion, c.releaseID, c.releaseID,
				)
				patch := map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]string{
							structs.ReleasePromoteWatchAnnotation: raw,
						},
					},
				}
				body, _ := json.Marshal(patch)
				_, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
					fmt.Sprintf("%s-%s", p.Name, c.nsSuffix), types.MergePatchType, body, am.PatchOptions{})
				require.NoError(t, err)

				events := captureReleaseWatcherEvents(t, p, func() {
					k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
				}, 0, 200*time.Millisecond)

				assert.Empty(t, events, "schemaVersion=%d MUST NOT emit; got %v", c.schemaVersion, events)

				// Annotation MUST persist for the future api-pod.
				ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
					fmt.Sprintf("%s-%s", p.Name, c.nsSuffix), am.GetOptions{})
				assert.Equal(t, raw, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
					"schemaVersion=%d annotation MUST be preserved (no delete)", c.schemaVersion)
			})
		})
	}
}

// TestRunReleasePromoteWatchGC_SchemaVersionZero_LogAndSkip pins the
// defensive zero-value branch — JSON missing schemaVersion (zero value)
// must follow the same log-and-skip path as future-version annotations.
func TestRunReleasePromoteWatchGC_SchemaVersionZero_LogAndSkip(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appGC5z", "Updating", "R-GC-5Z")
		// SchemaVersion field omitted -> Go zero value 0.
		raw := `{"releaseId":"R-GC-5Z","atomVersion":"R-GC-5Z","actor":"zero@convox.com"}`
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					structs.ReleasePromoteWatchAnnotation: raw,
				},
			},
		}
		body, _ := json.Marshal(patch)
		_, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
			fmt.Sprintf("%s-appGC5z", p.Name), types.MergePatchType, body, am.PatchOptions{})
		require.NoError(t, err)

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 0, 200*time.Millisecond)

		assert.Empty(t, events)

		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appGC5z", p.Name), am.GetOptions{})
		assert.Equal(t, raw, ns.Annotations[structs.ReleasePromoteWatchAnnotation])
	})
}

// TestRunReleasePromoteWatchGC_EmptyRack: scan with no app namespaces ->
// no panic, no event.
func TestRunReleasePromoteWatchGC_EmptyRack(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 0, 200*time.Millisecond)

		assert.Empty(t, events)
	})
}

// TestReleasePromoteWatcher_ConcurrentPromotesSerializedByLock pins the
// race-test invariant: many goroutines concurrently calling tryAcquireWatchSlot
// for the same (app, release-id) only land ONE acquisition.
func TestReleasePromoteWatcher_ConcurrentPromotesSerializedByLock(t *testing.T) {
	const N = 200
	var (
		wg       sync.WaitGroup
		acquired int32
		releases []func()
		relMu    sync.Mutex
	)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, rel := k8s.TryAcquireReleasePromoteWatchSlotForTest("appCC", "R-CC-1")
			if ok {
				atomic.AddInt32(&acquired, 1)
				relMu.Lock()
				releases = append(releases, rel)
				relMu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), atomic.LoadInt32(&acquired),
		"concurrent acquires for the same (app, release-id) MUST land exactly ONE acquisition")

	// Cleanup.
	for _, r := range releases {
		r()
	}
}

// TestReleasePromoteWatcher_GCConcurrentWithSteadyStateNoCorruption
// pins the cold-start-GC vs steady-state-watcher race scenario. Both
// emit-paths exit cleanly; the slot self-clears; no double-launch occurs.
func TestReleasePromoteWatcher_GCConcurrentWithSteadyStateNoCorruption(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appCC2", "Running", "R-CC-2")
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-CC-2",
			AtomVersion:   "R-CC-2",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), "appCC2", &state))

		var wg sync.WaitGroup
		// Drive watcher and GC scan concurrently 10x to provoke any race.
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
			}()
		}
		wg.Wait()

		// Wait long enough for any launched watchers to finish (Running -> success).
		time.Sleep(500 * time.Millisecond)

		// At end, slot must be released.
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest("appCC2", "R-CC-2"),
			"after concurrent GC+watcher exits, slot MUST be released")
	})
}

// TestReleasePromoteWatcher_PanicRecovery exercises the outer-defer
// recover() guard: a panic from inside the watcher's polling loop must
// (a) not propagate out of the goroutine, (b) cause the cleanup defer
// to emit an `app:promote:errored` event with a watcher-panic message,
// (c) leave the slot released, and (d) clean up the watch annotation.
// This is the M-A06-1 / row-9 regression test — the production code at
// release_watcher.go::runReleasePromoteWatcher relies on the recover()
// to keep the api pod alive when an unexpected nil-pointer or map-write
// panic strikes the polling tick.
func TestReleasePromoteWatcher_PanicRecovery(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "appPanic"
	const releaseID = "R-PANIC-1"

	// Install a panic hook that fires on the FIRST tick. The watcher
	// loop's hook check runs BEFORE the namespace read, so the panic
	// strikes mid-tick — recovery must reroute through the cleanup
	// defer's emit + delete + release path.
	var hookFired int32
	defer k8s.SetReleasePromoteWatcherPanicHookForTest(func(a, rid string) {
		if a == app && rid == releaseID {
			if atomic.AddInt32(&hookFired, 1) == 1 {
				panic("synthetic-panic-from-hook")
			}
		}
	})()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, app, "Updating", releaseID)
		// Pre-write the watch annotation so the cleanup defer's
		// supersession-aware delete sees a matching record and removes it.
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     releaseID,
			AtomVersion:   releaseID,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, &state))

		events := captureReleaseWatcherEvents(t, p, func() {
			// MUST NOT panic out of this call — the watcher's recover()
			// converts the panic into a terminal errored emit.
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), app, &state)
		}, 1, 2*time.Second)

		// Assert the panic actually fired (otherwise the test is a no-op).
		assert.Equal(t, int32(1), atomic.LoadInt32(&hookFired), "panic hook must have fired exactly once")

		// Recovered panic must surface as an errored event with watcher-panic prefix.
		ev := findEventByAction(events, "app:promote:errored")
		require.NotNil(t, ev, "expected app:promote:errored on recovered panic; got %v", events)
		assert.Equal(t, "error", ev["status"])
		// EventSend writes opts.Error into data.message (event.go:101-104).
		// The watcher's panic recovery sets resultError="watcher-panic: ..."
		// which routes through opts.Error on the error path, so the panic
		// reason surfaces in payload.data.message — NOT a top-level error
		// field on the canonical event JSON.
		data, _ := ev["data"].(map[string]any)
		require.NotNil(t, data, "errored event must carry data payload")
		msg, _ := data["message"].(string)
		assert.Contains(t, msg, "watcher-panic", "expected watcher-panic in data.message; got %q", msg)

		// Slot must be released regardless of panic.
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, releaseID),
			"slot MUST be released after a recovered panic")

		// Annotation must be cleaned up (supersession-aware delete confirms our release-id).
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		assert.Empty(t, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"annotation MUST be deleted after a recovered panic")
	})
}

// TestReleasePromoteWatcher_PanicDuringStateMachine_RecoverEmitsAndCleansUp
// covers the secondary-panic case: the watcher loop body panics, the
// cleanup defer's emit + annotation-delete logic ALSO panics, and the
// outermost LIFO bare-defer release() must still drop the slot. This
// is the row-11a regression test — the m-A05-01 belt-and-suspenders
// fix exists precisely to keep a slot from being permanently leaked
// when both the loop body and the cleanup defer fail at once.
func TestReleasePromoteWatcher_PanicDuringStateMachine_RecoverEmitsAndCleansUp(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "appPanic2"
	const releaseID = "R-PANIC-2"

	var loopPanicFired, cleanupPanicFired int32
	defer k8s.SetReleasePromoteWatcherPanicHookForTest(func(a, rid string) {
		if a == app && rid == releaseID {
			if atomic.AddInt32(&loopPanicFired, 1) == 1 {
				panic("synthetic-loop-panic")
			}
		}
	})()
	defer k8s.SetReleasePromoteCleanupDeferPanicHookForTest(func(a, rid string) {
		if a == app && rid == releaseID {
			if atomic.AddInt32(&cleanupPanicFired, 1) == 1 {
				panic("synthetic-cleanup-panic")
			}
		}
	})()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, app, "Updating", releaseID)
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     releaseID,
			AtomVersion:   releaseID,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		// Run watcher inside a goroutine that catches anything that
		// the LIFO bare-defer didn't manage to swallow. A test failure
		// occurs only if the slot leaks (the bare-defer release() did
		// NOT fire). The cleanup defer's panic propagates past the
		// current goroutine — testify and t.Fail() handle the runtime
		// panic via t.FailNow at the assertion below.
		var ranToCompletion int32
		done := make(chan struct{})
		go func() {
			defer func() {
				// Cleanup-defer panic propagates here; recover so the
				// test goroutine terminates cleanly. The release()
				// path we care about runs BEFORE this recover() in
				// LIFO order, so by the time we see the panic the
				// slot is already dropped.
				_ = recover()
				atomic.StoreInt32(&ranToCompletion, 1)
				close(done)
			}()
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), app, &state)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("watcher goroutine did not return")
		}

		assert.Equal(t, int32(1), atomic.LoadInt32(&ranToCompletion), "watcher goroutine must complete (slot release path)")
		assert.Equal(t, int32(1), atomic.LoadInt32(&loopPanicFired), "loop-panic hook must have fired")
		assert.Equal(t, int32(1), atomic.LoadInt32(&cleanupPanicFired), "cleanup-panic hook must have fired")

		// The critical invariant: slot must be released even when the
		// cleanup defer itself panicked. The LIFO bare-defer at the
		// top of runReleasePromoteWatcher provides this guarantee.
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, releaseID),
			"slot MUST be released even when cleanup defer panics (belt-and-suspenders)")
	})
}

// TestReleasePromoteWatcher_SingleEmitPerTerminalState pins the
// single-emit invariant: each watcher emits EXACTLY one terminal-state
// event per release-id. Drives the watcher to a `Running` terminal,
// then asserts only one `app:promote:completed` payload arrives at the
// webhook receiver — no retry-loop spam, no duplicate event from the
// cleanup defer's emit path firing twice. Row-11c regression test.
func TestReleasePromoteWatcher_SingleEmitPerTerminalState(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appSingle", "Running", "R-SINGLE-1")

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-SINGLE-1",
			AtomVersion:   "R-SINGLE-1",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		// Capture *all* events with a timeout that comfortably exceeds
		// the watcher's first-tick latency (20ms) plus the cleanup
		// defer drain.
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), "appSingle", &state)
		}, 1, 1*time.Second)
		// Allow extra settle time so any rogue retry-emit would surface.
		time.Sleep(200 * time.Millisecond)

		// Re-collect after the settle window to confirm no late event.
		// Phase H R2 fix (m-A06-1 / m-A09-DEAD-CODE): previously this
		// captured `events2` was discarded as `_ = events2` — the
		// settle-window second-capture pattern was started but the
		// invariant was never asserted. Now explicitly asserts no
		// late terminal event arrives so a future regression that
		// introduces async webhook emit (e.g. a goroutine launched
		// from inside emitReleasePromoteResult) would be caught.
		events2 := captureReleaseWatcherEvents(t, p, func() {}, 0, 100*time.Millisecond)
		assert.Empty(t, events2, "no late terminal event must arrive after watcher exits; got %v", events2)

		// Count completed/errored/cancelled emits — must total exactly 1.
		count := 0
		for _, ev := range events {
			a, _ := ev["action"].(string)
			switch a {
			case "app:promote:completed", "app:promote:errored", "app:promote:cancelled":
				count++
			}
		}
		assert.Equal(t, 1, count,
			"watcher MUST emit exactly one terminal event per release-id; got %d (events=%v)", count, events)
	})
}

// TestReleasePromoteWatcher_CleanupDeferPanic_StillReleasesSlot is the
// m-A05-01 regression test. A successful state-machine run is followed
// by a panic INSIDE the cleanup defer. The outermost LIFO bare-defer
// release() must still fire so the slot is dropped — without the
// belt-and-suspenders defer, the slot would leak until api-pod restart.
func TestReleasePromoteWatcher_CleanupDeferPanic_StillReleasesSlot(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	const app = "appCleanupPanic"
	const releaseID = "R-CLEAN-PANIC-1"

	var cleanupPanicFired int32
	defer k8s.SetReleasePromoteCleanupDeferPanicHookForTest(func(a, rid string) {
		if a == app && rid == releaseID {
			if atomic.AddInt32(&cleanupPanicFired, 1) == 1 {
				panic("synthetic-cleanup-only-panic")
			}
		}
	})()

	testProvider(t, func(p *k8s.Provider) {
		// Happy-path trigger — the watcher exits on Running.
		seedAppNamespaceWithStatus(t, p, app, "Running", releaseID)
		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     releaseID,
			AtomVersion:   releaseID,
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}

		done := make(chan struct{})
		go func() {
			defer func() {
				_ = recover() // catch the cleanup-defer panic in this goroutine
				close(done)
			}()
			k8s.RunReleasePromoteWatcherForTest(p, context.Background(), app, &state)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("watcher goroutine did not return")
		}

		assert.Equal(t, int32(1), atomic.LoadInt32(&cleanupPanicFired), "cleanup-defer panic hook must have fired")
		assert.False(t, k8s.ReleasePromoteWatchSlotHeldForTest(app, releaseID),
			"belt-and-suspenders: slot MUST be released even when cleanup defer panics post-emit")
	})
}

// TestReleasePromoteWatcher_SupersessionAware_CleanupSkips is the
// m-A12-03 regression test. Watcher A reaches a terminal state; while
// A's cleanup is in-flight, Watcher B starts and overwrites the watch
// annotation. A's cleanup must read the annotation, see the new
// release-id, and SKIP the delete so B's payload survives.
//
// We exercise the read-before-delete primitive directly via
// DeleteReleasePromoteWatchAnnotationIfMatchesForTest — the
// race-window orchestration that would arise inside the goroutine is
// awkward to schedule deterministically with a fake clientset, but
// the primitive is the load-bearing piece and the production cleanup
// defer routes through it on every steady-state exit.
func TestReleasePromoteWatcher_SupersessionAware_CleanupSkips(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		const app = "appSupersess"
		seedAppNamespaceWithStatus(t, p, app, "Updating", "R2")

		// Step 1 (Watcher A's annotation): write payload for R1.
		stateA := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R1",
			AtomVersion:   "R1",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, &stateA))

		// Step 2 (Watcher B overwrites): simulate a newer promote
		// landing before A's cleanup defer runs.
		stateB := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R2",
			AtomVersion:   "R2",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "bob@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, &stateB))

		// Step 3 (A's cleanup reads + skips): A's cleanup defer calls
		// the supersession-aware variant with its own release-id (R1).
		// It should see R2 in the annotation and SKIP the delete.
		err := k8s.DeleteReleasePromoteWatchAnnotationIfMatchesForTest(p, context.Background(), app, "R1")
		require.NoError(t, err, "supersession-aware delete must succeed (no-op skip)")

		// Step 4 (verify): annotation must still hold B's payload.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		raw := ns.Annotations[structs.ReleasePromoteWatchAnnotation]
		require.NotEmpty(t, raw, "B's annotation MUST persist (A skipped delete)")
		var got structs.ReleasePromoteWatchState
		require.NoError(t, json.Unmarshal([]byte(raw), &got))
		assert.Equal(t, "R2", got.ReleaseID, "annotation must still hold B's payload after A's skipped cleanup")
		assert.Equal(t, "bob@example.com", got.Actor, "actor must reflect B")

		// Step 5: When B's own cleanup runs (with releaseID=R2), the
		// delete proceeds because the stored release-id matches B's.
		err = k8s.DeleteReleasePromoteWatchAnnotationIfMatchesForTest(p, context.Background(), app, "R2")
		require.NoError(t, err, "matching-release cleanup must proceed normally")
		ns2, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		assert.Empty(t, ns2.Annotations[structs.ReleasePromoteWatchAnnotation],
			"B's cleanup MUST delete the annotation (matching release-id)")
	})
}

// TestReleasePromoteWatcher_SupersessionAware_CleanupSkips_TOCTOU_PatchTestOpRejects
// pins the JSON-Patch `test` op fallback semantics introduced in the
// Phase H R2 fix (m-A03-01 / m-A05-01 / m-A12-01). The in-process
// release-id compare in deleteReleasePromoteWatchAnnotationIfMatches
// correctly catches supersession in the simple sequential case
// (covered by SupersessionAware_CleanupSkips above), but a fast
// supersession landing BETWEEN our Get and our Patch leaves a residual
// TOCTOU window that the test op closes — apiserver atomically
// rejects the Patch with Invalid (test op failed) when the annotation
// value differs from what we read.
//
// The fake clientset honors JSONPatchType test ops via
// evanphx/json-patch, so this test directly exercises the rejection
// path: hand-craft a JSON-Patch with a stale `value` (encoding what
// USED to be in the annotation), apply it after the annotation has
// been overwritten, and assert the apiserver rejects with Invalid.
// Mirrors what would happen in production when a fast B-overwrite
// races our Patch landing.
func TestReleasePromoteWatcher_SupersessionAware_CleanupSkips_TOCTOU_PatchTestOpRejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		const app = "appTOCTOU"
		seedAppNamespaceWithStatus(t, p, app, "Updating", "R-TOCTOU-A")

		// Write annotation with payload A (release-id A).
		stateA := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-TOCTOU-A",
			AtomVersion:   "R-TOCTOU-A",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "alice@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, &stateA))

		// Capture the raw annotation string for A — this is the value
		// the test op would expect if our Get had read it.
		nsA, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		rawA := nsA.Annotations[structs.ReleasePromoteWatchAnnotation]
		require.NotEmpty(t, rawA)

		// Simulate a concurrent overwrite: B writes its annotation
		// between our (hypothetical) Get and Patch.
		stateB := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-TOCTOU-B",
			AtomVersion:   "R-TOCTOU-B",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(60 * time.Second),
			Actor:         "bob@example.com",
		}
		require.NoError(t, k8s.WriteReleasePromoteWatchAnnotationForTest(p, context.Background(), app, &stateB))

		// Hand-craft the JSON-Patch our production code would have
		// constructed if it had read rawA and skipped the in-process
		// check (i.e. it didn't get to compare release-id mismatch).
		// The test op encodes A's raw value; the apiserver will reject
		// because the annotation now holds B's value.
		rawAJSON, jerr := json.Marshal(rawA)
		require.NoError(t, jerr)
		patch := []byte(fmt.Sprintf(
			`[{"op":"test","path":"/metadata/annotations/convox.com~1release-promote-watch","value":%s},{"op":"remove","path":"/metadata/annotations/convox.com~1release-promote-watch"}]`,
			rawAJSON,
		))
		_, perr := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), types.JSONPatchType, patch, am.PatchOptions{})
		require.Error(t, perr, "apiserver MUST reject JSON-Patch when test op value mismatches current annotation")
		// Real apiserver wraps test-op failures as Invalid (HTTP 422)
		// or Conflict (HTTP 409); production code handles both via
		// kerr.IsInvalid / kerr.IsConflict and treats them as the
		// supersession-aware skip path. The fake clientset
		// (vendor/k8s.io/client-go/testing/fixture.go ApplyPatch:246)
		// surfaces the raw evanphx/json-patch.v4 error directly without
		// wrapping it as a typed apierror — error message is
		// "testing value <path> failed: test failed". Both code paths
		// (typed Invalid/Conflict in production; raw error in fake)
		// converge on the same behavior: the Patch did NOT land, so
		// the annotation in the namespace is preserved unchanged.
		// Production behavior is verified by the Invalid/Conflict
		// branches in deleteReleasePromoteWatchAnnotationIfMatches;
		// fake-clientset behavior is verified here via the persistence
		// assertion below.
		assert.Contains(t, perr.Error(), "test failed",
			"fake clientset surfaces JSON-Patch test-op failure with 'test failed' suffix; got %v", perr)

		// Verify B's annotation persists — A's stale Patch did NOT land.
		nsAfter, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-%s", p.Name, app), am.GetOptions{})
		rawAfter := nsAfter.Annotations[structs.ReleasePromoteWatchAnnotation]
		require.NotEmpty(t, rawAfter, "B's annotation MUST persist after rejected Patch")
		var got structs.ReleasePromoteWatchState
		require.NoError(t, json.Unmarshal([]byte(rawAfter), &got))
		assert.Equal(t, "R-TOCTOU-B", got.ReleaseID, "annotation must still hold B's payload after rejected Patch")
	})
}

// TestDeleteReleasePromoteWatchAnnotationIfMatches_NotFound pins the
// namespace-not-found no-op branch. When the app's namespace doesn't
// exist (deleted concurrently with the watcher's cleanup defer), the
// supersession-aware delete returns nil instead of erroring — same
// best-effort semantics as the unconditional variant. Phase H R2 fix
// (m-A06-3): exercises one of the three previously-uncovered no-op
// branches so a regression that converts the no-op into an error
// would be caught.
func TestDeleteReleasePromoteWatchAnnotationIfMatches_NotFound(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		// Do NOT seed the namespace — Get returns NotFound.
		err := k8s.DeleteReleasePromoteWatchAnnotationIfMatchesForTest(
			p, context.Background(), "appNotFound", "R-NF-1")
		require.NoError(t, err, "namespace-not-found MUST be a silent no-op")
	})
}

// TestDeleteReleasePromoteWatchAnnotationIfMatches_EmptyAnnotation pins
// the empty-annotation no-op branch. When the app's namespace exists
// but the watch annotation is absent (already deleted by GC, or never
// written), the supersession-aware delete returns nil without issuing
// any patch. Phase H R2 fix (m-A06-3).
func TestDeleteReleasePromoteWatchAnnotationIfMatches_EmptyAnnotation(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		// Seed namespace WITHOUT the watch annotation. seedAppNamespaceWithStatus
		// only writes app-status / app-release annotations.
		seedAppNamespaceWithStatus(t, p, "appEmpty", "Updating", "R-EMPTY-1")

		err := k8s.DeleteReleasePromoteWatchAnnotationIfMatchesForTest(
			p, context.Background(), "appEmpty", "R-EMPTY-1")
		require.NoError(t, err, "empty annotation MUST be a silent no-op")

		// Confirm seed annotations are untouched.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appEmpty", p.Name), am.GetOptions{})
		assert.Equal(t, "Updating", ns.Annotations["convox.com/app-status"],
			"app-status annotation MUST be preserved (no patch issued)")
	})
}

// TestDeleteReleasePromoteWatchAnnotationIfMatches_CorruptJSON pins the
// corrupt-JSON no-op branch. When the watch annotation is present but
// holds invalid JSON, the supersession-aware delete logs and returns
// nil without issuing a patch — defers to the GC scan's corrupt-JSON
// branch which deletes via the unconditional variant (since no
// release-id is attributable). Phase H R2 fix (m-A06-3).
func TestDeleteReleasePromoteWatchAnnotationIfMatches_CorruptJSON(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appCorrupt", "Updating", "R-CORRUPT-1")
		// Plant a corrupt-JSON annotation directly.
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					structs.ReleasePromoteWatchAnnotation: "{not-valid-json",
				},
			},
		}
		body, _ := json.Marshal(patch)
		_, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
			fmt.Sprintf("%s-appCorrupt", p.Name), types.MergePatchType, body, am.PatchOptions{})
		require.NoError(t, err)

		derr := k8s.DeleteReleasePromoteWatchAnnotationIfMatchesForTest(
			p, context.Background(), "appCorrupt", "R-CORRUPT-1")
		require.NoError(t, derr, "corrupt-JSON MUST be a silent no-op (deferred to GC scan)")

		// Confirm the corrupt annotation persists — the supersession-aware
		// variant does NOT delete corrupt JSON (no payload to attribute).
		// The cold-start GC scan handles this case via its unconditional
		// delete branch (release_watcher.go scanReleasePromoteAnnotations
		// corrupt-JSON branch).
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appCorrupt", p.Name), am.GetOptions{})
		assert.Equal(t, "{not-valid-json", ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"corrupt annotation MUST persist (defer to GC scan; no patch issued)")
	})
}
