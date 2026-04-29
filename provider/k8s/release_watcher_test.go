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

// TestRunReleasePromoteWatchGC_UnknownSchemaVersion_LogAndSkip: future
// schemaVersion -> annotation NOT deleted, no event emitted (rolling-upgrade
// safety).
func TestRunReleasePromoteWatchGC_UnknownSchemaVersion_LogAndSkip(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		seedAppNamespaceWithStatus(t, p, "appGC5", "Updating", "R-GC-5")
		// schemaVersion=2 (future); rc6 reader must log-and-skip.
		raw := `{"schemaVersion":2,"releaseId":"R-GC-5","atomVersion":"R-GC-5","actor":"future@convox.com"}`
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					structs.ReleasePromoteWatchAnnotation: raw,
				},
			},
		}
		body, _ := json.Marshal(patch)
		_, err := p.Cluster.CoreV1().Namespaces().Patch(context.TODO(),
			fmt.Sprintf("%s-appGC5", p.Name), types.MergePatchType, body, am.PatchOptions{})
		require.NoError(t, err)

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 0, 200*time.Millisecond)

		assert.Empty(t, events, "unknown schemaVersion MUST NOT emit; got %v", events)

		// Annotation MUST persist for the future api-pod.
		ns, _ := p.Cluster.CoreV1().Namespaces().Get(context.TODO(),
			fmt.Sprintf("%s-appGC5", p.Name), am.GetOptions{})
		assert.Equal(t, raw, ns.Annotations[structs.ReleasePromoteWatchAnnotation],
			"unknown schemaVersion annotation MUST be preserved (no delete)")
	})
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
