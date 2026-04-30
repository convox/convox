package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

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
