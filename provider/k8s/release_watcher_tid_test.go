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
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// The GC loop runs on the provider's background context, so ContextTID() is
// empty while tenant namespaces carry a tid segment. These fixtures reproduce
// that pairing: the provider has no tenant context, the namespace is
// <rack>-<tid>-<app>. A fixture that gave the provider a tenant context would
// resolve AppNamespace correctly and pass against the broken code.
func seedWatchNamespace(t *testing.T, p *k8s.Provider, namespace, tid, app, atomStatus, releaseID, watchAnnotation string) {
	t.Helper()
	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)

	annotations := map[string]string{}
	if atomStatus != "" {
		annotations["convox.com/app-status"] = atomStatus
	}
	if releaseID != "" {
		annotations["convox.com/app-release"] = releaseID
	}
	if watchAnnotation != "" {
		annotations[structs.ReleasePromoteWatchAnnotation] = watchAnnotation
	}
	labels := map[string]string{
		"app":    app,
		"name":   app,
		"rack":   p.Name,
		"system": "convox",
		"type":   "app",
	}
	if tid != "" {
		labels["tid"] = tid
	}

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name:        namespace,
			Annotations: annotations,
			Labels:      labels,
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func watchStateJSON(t *testing.T, releaseID string, expiresIn time.Duration) string {
	t.Helper()
	raw, err := json.Marshal(structs.ReleasePromoteWatchState{
		SchemaVersion: 1,
		ReleaseID:     releaseID,
		AtomVersion:   releaseID,
		StartedAt:     time.Now().UTC().Add(-time.Minute),
		ExpiresAt:     time.Now().UTC().Add(expiresIn),
		Actor:         "deploy-key:test",
	})
	require.NoError(t, err)
	return string(raw)
}

func watchAnnotationOf(t *testing.T, p *k8s.Provider, namespace string) string {
	t.Helper()
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), namespace, am.GetOptions{})
	require.NoError(t, err)
	return ns.Annotations[structs.ReleasePromoteWatchAnnotation]
}

// TestReleasePromoteWatchGC_TenantNamespace_ExpiredEmitsOnceThenSilent is the
// regression test. Before the fix the cleanup rebuilt the namespace from an
// empty tenant context, the delete hit <rack>-<app>, NotFound read as
// already-clean, and every later tick re-emitted the same promote.
func TestReleasePromoteWatchGC_TenantNamespace_ExpiredEmitsOnceThenSilent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-tenantapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "tenantapp", "Running", "R-TID-1",
			watchStateJSON(t, "R-TID-1", -time.Hour))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 2, 500*time.Millisecond)

		require.Len(t, events, 1, "expired tenant watch MUST emit exactly once; got %v", events)
		assert.Equal(t, "app:promote:completed", events[0]["action"])
		assert.Empty(t, watchAnnotationOf(t, p, namespace),
			"cleanup MUST clear the annotation on the tenant namespace it read")

		repeat := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)

		assert.Empty(t, repeat, "a second GC pass MUST be silent; got %v", repeat)
	})
}

// TestReleasePromoteWatchGC_RackNamespace_ExpiredEmitsOnceThenSilent proves the
// ordinary non-tenant path is unchanged.
func TestReleasePromoteWatchGC_RackNamespace_ExpiredEmitsOnceThenSilent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-rackapp", p.Name)
		seedWatchNamespace(t, p, namespace, "", "rackapp", "Running", "R-RACK-1",
			watchStateJSON(t, "R-RACK-1", -time.Hour))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 2, 500*time.Millisecond)

		require.Len(t, events, 1, "expired watch MUST emit exactly once; got %v", events)
		assert.Equal(t, "app:promote:completed", events[0]["action"])
		assert.Empty(t, watchAnnotationOf(t, p, namespace))

		repeat := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)

		assert.Empty(t, repeat, "a second GC pass MUST be silent; got %v", repeat)
	})
}

// TestReleasePromoteWatchGC_TenantNamespace_ResumesLiveWatch covers the watcher
// the GC spawns for a still-live annotation, which is how a promote in flight
// across an api-pod restart gets its result. It saw the wrong namespace, read
// NotFound and exited silently on every tick.
func TestReleasePromoteWatchGC_TenantNamespace_ResumesLiveWatch(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-liveapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "liveapp", "Running", "R-TID-2",
			watchStateJSON(t, "R-TID-2", time.Minute))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:completed")
		require.NotNil(t, ev, "GC MUST resume a live tenant watch; got %v", events)
		assert.Equal(t, "success", ev["status"])

		assert.Eventually(t, func() bool {
			return watchAnnotationOf(t, p, namespace) == ""
		}, 2*time.Second, 20*time.Millisecond,
			"the resumed watcher MUST clear the annotation it was resumed from")
	})
}

// TestReleasePromoteWatchGC_CorruptJSON_LeavesRackNamespaceAlone pins the
// unguarded branch: the corrupt-JSON delete carries no release-id compare, so a
// derived namespace let a tenant's corrupt annotation strip a same-named rack
// app's live one.
func TestReleasePromoteWatchGC_CorruptJSON_LeavesRackNamespaceAlone(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		tenantNamespace := fmt.Sprintf("%s-2bzcs9-shared", p.Name)
		rackNamespace := fmt.Sprintf("%s-shared", p.Name)
		// schemaVersion 2 is an annotation the scan must leave untouched, so
		// anything missing from it afterwards came from the tenant's branch.
		rackWatch := `{"schemaVersion":2,"releaseId":"R-RACK-2","atomVersion":"R-RACK-2","actor":"future@convox.com"}`

		seedWatchNamespace(t, p, tenantNamespace, "2bzcs9", "shared", "Running", "R-TID-3", "{not-valid-json")
		seedWatchNamespace(t, p, rackNamespace, "", "shared", "Running", "R-RACK-2", rackWatch)

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)

		assert.Empty(t, events, "corrupt JSON MUST NOT emit; got %v", events)
		assert.Empty(t, watchAnnotationOf(t, p, tenantNamespace),
			"corrupt annotation MUST be cleared on the namespace it was read from")
		assert.Equal(t, rackWatch, watchAnnotationOf(t, p, rackNamespace),
			"a same-named rack app's annotation MUST NOT be touched")
	})
}

// TestReleasePromoteWatchGC_CleanupFailureIsLogged: a cleanup error that is not
// NotFound has no observable trace in the namespace, so a surviving annotation
// is indistinguishable from the loop this patch fixes. It has to reach the log,
// and it must not emit, so the next tick can retry.
func TestReleasePromoteWatchGC_CleanupFailureIsLogged(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-failapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "failapp", "Running", "R-TID-4",
			watchStateJSON(t, "R-TID-4", -time.Hour))

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		kk.PrependReactor("get", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerr.NewInternalError(fmt.Errorf("etcd unavailable"))
		})

		readStdout := captureStdout(t)
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)
		out := readStdout()

		assert.Empty(t, events, "a failed cleanup MUST NOT emit; got %v", events)
		assert.Contains(t, out, fmt.Sprintf("kind=gc_cleanup_failed namespace=%s", namespace))
	})
}

// TestReleasePromoteWatchGC_TenantNamespace_RecoveryCommandNamesRealNamespace:
// the operator remedy printed for an unrecognized schemaVersion has to name a
// namespace that exists, or running it verbatim fails.
func TestReleasePromoteWatchGC_TenantNamespace_RecoveryCommandNamesRealNamespace(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-futureapp", p.Name)
		raw := `{"schemaVersion":2,"releaseId":"R-TID-5","atomVersion":"R-TID-5","actor":"future@convox.com"}`
		seedWatchNamespace(t, p, namespace, "2bzcs9", "futureapp", "Running", "R-TID-5", raw)

		readStdout := captureStdout(t)
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)
		out := readStdout()

		assert.Empty(t, events, "unknown schemaVersion MUST NOT emit; got %v", events)
		assert.Equal(t, raw, watchAnnotationOf(t, p, namespace),
			"unknown schemaVersion MUST NOT be deleted")
		assert.Contains(t, out,
			fmt.Sprintf(`recovery="kubectl annotate ns %s convox.com/release-promote-watch-"`, namespace))
	})
}

// TestReleasePromoteWatchGC_EmitFollowsTheCompareAndSwap: the GC runs unelected
// in every api pod, so the swap is what decides which replica notifies. A
// caller that loses it must stay quiet.
func TestReleasePromoteWatchGC_EmitFollowsTheCompareAndSwap(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-raceapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "raceapp", "Running", "R-TID-6",
			watchStateJSON(t, "R-TID-6", -time.Hour))

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		kk.PrependReactor("patch", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerr.NewConflict(
				schema.GroupResource{Resource: "namespaces"}, namespace, fmt.Errorf("test op rejected"))
		})

		readStdout := captureStdout(t)
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)
		out := readStdout()

		assert.Empty(t, events, "losing the swap MUST NOT emit; got %v", events)
		assert.Contains(t, out, fmt.Sprintf("kind=gc_cleanup_skip namespace=%s release_id=R-TID-6", namespace),
			"the branch must be reached and its suppression recorded")
		assert.NotContains(t, out, "kind=gc_cleanup namespace=")
		assert.NotEmpty(t, watchAnnotationOf(t, p, namespace),
			"a rejected swap MUST leave the annotation for the next tick")
	})
}

// TestReleasePromoteWatchGC_TenantNamespace_SupersededEmitsOnceThenSilent: the
// superseded branch shares the cleanup path, so it carried the same loop.
func TestReleasePromoteWatchGC_TenantNamespace_SupersededEmitsOnceThenSilent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-supersededapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "supersededapp", "Updating", "R-TID-7-NEW",
			watchStateJSON(t, "R-TID-7-OLD", time.Minute))

		readStdout := captureStdout(t)
		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 2, 500*time.Millisecond)
		out := readStdout()

		require.Len(t, events, 1, "superseded watch MUST emit exactly once; got %v", events)
		assert.Equal(t, "app:promote:cancelled", events[0]["action"])
		assert.Contains(t, out, fmt.Sprintf("kind=gc_cleanup namespace=%s release_id=R-TID-7-OLD status=cancelled", namespace),
			"the GC branch MUST be what emitted, not a spawned watcher")
		assert.Empty(t, watchAnnotationOf(t, p, namespace))

		repeat := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)

		assert.Empty(t, repeat, "a second GC pass MUST be silent; got %v", repeat)
	})
}

// TestReleasePromoteWatchGC_TenantNamespace_TimeoutWhenStatusNotTerminal: an
// expired watch over a namespace the AtomController never reached a terminal
// status for still resolves once, as an error, rather than repeating.
func TestReleasePromoteWatchGC_TenantNamespace_TimeoutWhenStatusNotTerminal(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-stuckapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "stuckapp", "", "R-TID-8",
			watchStateJSON(t, "R-TID-8", -time.Hour))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 2, 500*time.Millisecond)

		require.Len(t, events, 1, "expired watch MUST emit exactly once; got %v", events)
		assert.Equal(t, "app:promote:errored", events[0]["action"])
		data, _ := events[0]["data"].(map[string]any)
		assert.Equal(t, "watcher-timeout", data["message"])
		assert.Empty(t, watchAnnotationOf(t, p, namespace))

		repeat := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 200*time.Millisecond)

		assert.Empty(t, repeat, "a second GC pass MUST be silent; got %v", repeat)
	})
}

// TestReleasePromoteWatchGC_TenantAndRackAppsShareAName: two apps with the same
// name in different namespaces each resolve against their own object.
func TestReleasePromoteWatchGC_TenantAndRackAppsShareAName(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		tenantNamespace := fmt.Sprintf("%s-2bzcs9-web", p.Name)
		rackNamespace := fmt.Sprintf("%s-web", p.Name)

		seedWatchNamespace(t, p, tenantNamespace, "2bzcs9", "web", "Running", "R-TENANT",
			watchStateJSON(t, "R-TENANT", -time.Hour))
		seedWatchNamespace(t, p, rackNamespace, "", "web", "Running", "R-RACK",
			watchStateJSON(t, "R-RACK", -time.Hour))

		events := captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 3, 500*time.Millisecond)

		require.Len(t, events, 2, "each namespace MUST resolve once; got %v", events)
		ids := []string{}
		for _, ev := range events {
			data, _ := ev["data"].(map[string]any)
			id, _ := data["id"].(string)
			ids = append(ids, id)
		}
		assert.ElementsMatch(t, []string{"R-TENANT", "R-RACK"}, ids)
		assert.Empty(t, watchAnnotationOf(t, p, tenantNamespace))
		assert.Empty(t, watchAnnotationOf(t, p, rackNamespace))
	})
}

// TestReleasePromoteWatchGC_LogsEachCleanup: a loop that emits user-visible
// notifications had no log line at all, which is why it ran unnoticed.
func TestReleasePromoteWatchGC_LogsEachCleanup(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-loggedapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "loggedapp", "Running", "R-TID-9",
			watchStateJSON(t, "R-TID-9", -time.Hour))

		readStdout := captureStdout(t)
		captureReleaseWatcherEvents(t, p, func() {
			k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		}, 1, 500*time.Millisecond)
		out := readStdout()

		assert.Contains(t, out, fmt.Sprintf("kind=gc_cleanup namespace=%s release_id=R-TID-9", namespace),
			"a successful GC cleanup MUST leave a log line")
	})
}

// TestReleasePromoteWatchGC_CorruptJSONCleanupFailureIsLogged: the corrupt-JSON
// branch deletes without a compare-and-swap, so a failure there has no other
// trace at all.
func TestReleasePromoteWatchGC_CorruptJSONCleanupFailureIsLogged(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-2bzcs9-corruptapp", p.Name)
		seedWatchNamespace(t, p, namespace, "2bzcs9", "corruptapp", "Running", "R-TID-10", "{not-valid-json")

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		kk.PrependReactor("patch", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerr.NewInternalError(fmt.Errorf("etcd unavailable"))
		})

		readStdout := captureStdout(t)
		k8s.ScanReleasePromoteAnnotationsForTest(p, context.Background())
		out := readStdout()

		assert.Contains(t, out, fmt.Sprintf("kind=gc_cleanup_failed namespace=%s", namespace))
		assert.Equal(t, "{not-valid-json", watchAnnotationOf(t, p, namespace),
			"a failed cleanup MUST leave the annotation for the next tick")
	})
}

// TestReleasePromoteWatcher_OutlivesTheRequestContext: the promote handler's
// context is cancelled the moment it returns, which left the watcher it starts
// dead on arrival on every rack.
func TestReleasePromoteWatcher_OutlivesTheRequestContext(t *testing.T) {
	defer k8s.SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond)()

	testProvider(t, func(p *k8s.Provider) {
		namespace := fmt.Sprintf("%s-detachedapp", p.Name)
		seedWatchNamespace(t, p, namespace, "", "detachedapp", "Running", "R-CTX-1",
			watchStateJSON(t, "R-CTX-1", time.Minute))

		state := structs.ReleasePromoteWatchState{
			SchemaVersion: 1,
			ReleaseID:     "R-CTX-1",
			AtomVersion:   "R-CTX-1",
			StartedAt:     time.Now().UTC(),
			ExpiresAt:     time.Now().UTC().Add(time.Minute),
			Actor:         "deploy-key:test",
		}

		ctx, cancel := context.WithCancel(context.Background())
		request := k8s.WithContextForTest(p, ctx)
		require.NotNil(t, request)
		cancel()

		events := captureReleaseWatcherEvents(t, request, func() {
			k8s.LaunchReleasePromoteWatcherForTest(request, "detachedapp", &state)
		}, 1, 2*time.Second)

		ev := findEventByAction(events, "app:promote:completed")
		require.NotNil(t, ev, "the watcher MUST survive the request context; got %v", events)
		assert.Equal(t, "success", ev["status"])
	})
}
