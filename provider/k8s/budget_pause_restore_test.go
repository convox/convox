package k8s_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func pauseRestoreStateService(name string, keda bool) structs.AppBudgetShutdownStateService {
	s := structs.AppBudgetShutdownStateService{
		Name:                       name,
		OriginalScale:              structs.AppBudgetShutdownStateOriginalScale{Count: 3, Replicas: 3},
		OriginalGracePeriodSeconds: 60,
	}
	if keda {
		s.KedaScaledObject = &structs.AppBudgetShutdownStateKeda{Name: name, PausedReplicasAnnotationSet: true}
	}
	return s
}

func pauseRestoreState(services ...structs.AppBudgetShutdownStateService) *structs.AppBudgetShutdownState {
	state := pauseShutdownState()
	shutdownAt := time.Now().UTC().Add(-time.Minute)
	state.ShutdownAt = &shutdownAt
	state.Services = services
	state.EligibleServiceCount = len(services)
	return state
}

func pauseRestoredDeployment(t *testing.T, kk *kfake.Clientset, name string) *appsv1.Deployment {
	t.Helper()
	d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, d.Spec.Replicas)
	require.NotNil(t, d.Spec.Template.Spec.TerminationGracePeriodSeconds)
	return d
}

type pauseRestoreEvents struct {
	mu       sync.Mutex
	captured []map[string]any
}

func (e *pauseRestoreEvents) restored() []map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	return findAllByAction(e.captured, k8s.ShutdownEventRestored)
}

func requireOneRestoredEvent(t *testing.T, events *pauseRestoreEvents) map[string]any {
	t.Helper()
	require.Eventually(t, func() bool { return len(events.restored()) > 0 }, 5*time.Second, 20*time.Millisecond)
	restored := events.restored()
	require.Len(t, restored, 1)
	data, _ := restored[0]["data"].(map[string]any)
	return data
}

func pauseFlapSuppressedUntil(t *testing.T, kk *kfake.Clientset) (string, bool) {
	t.Helper()
	ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
	require.NoError(t, err)
	v, ok := ns.Annotations[structs.BudgetFlapSuppressedUntilAnnotation]
	return v, ok
}

func runPauseRestore(t *testing.T, deployments []string, scaledObjects []*unstructured.Unstructured, state *structs.AppBudgetShutdownState, check func(*k8s.Provider, *kfake.Clientset)) *pauseRestoreEvents {
	t.Helper()
	events := &pauseRestoreEvents{}
	srv := webhookCaptureServer(&events.mu, &events.captured)
	t.Cleanup(srv.Close)

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		grace := int64(30)
		for _, name := range deployments {
			makeDeployment(t, kk, "rack1-app1", name, 0, &grace)
		}
		dyn := fake.NewSimpleDynamicClient(newDynamicScheme())
		for _, so := range scaledObjects {
			_, err := dyn.Resource(testScaledObjectGVR).Namespace("rack1-app1").Create(context.TODO(), so, am.CreateOptions{})
			require.NoError(t, err)
		}
		p.DynamicClient = dyn

		require.NoError(t, k8s.RestoreFromAnnotationForTest(p, "app1", "test-actor", state, "reset"))
		drainPendingDispatches()
		check(p, kk)
	})

	return events
}

func TestBudgetRestoreLeavesManualPausePaused(t *testing.T) {
	runPauseRestore(t,
		[]string{"worker"},
		[]*unstructured.Unstructured{pausedScaledObject("worker", 1, 5, pauseAnnosManual)},
		pauseRestoreState(pauseRestoreStateService("worker", true)),
		func(p *k8s.Provider, kk *kfake.Clientset) {
			d := pauseRestoredDeployment(t, kk, "worker")
			require.Equal(t, int32(0), *d.Spec.Replicas)
			require.Equal(t, int64(60), *d.Spec.Template.Spec.TerminationGracePeriodSeconds)
			require.Equal(t, pauseAnnosManual, pauseGetScaledObject(t, p, "worker").GetAnnotations())
		})
}

func TestBudgetRestoreReportsOnlyServicesItRestored(t *testing.T) {
	events := runPauseRestore(t,
		[]string{"worker", "web"},
		[]*unstructured.Unstructured{pausedScaledObject("worker", 1, 5, pauseAnnosManual)},
		pauseRestoreState(pauseRestoreStateService("worker", true), pauseRestoreStateService("web", false)),
		func(p *k8s.Provider, kk *kfake.Clientset) {
			require.Equal(t, int32(3), *pauseRestoredDeployment(t, kk, "web").Spec.Replicas)
			require.Equal(t, int32(0), *pauseRestoredDeployment(t, kk, "worker").Spec.Replicas)
		})

	data := requireOneRestoredEvent(t, events)
	require.Equal(t, "web", data["restored_services"])
	require.Equal(t, "1", data["restored_count"])
	require.Equal(t, "worker", data["paused_services"])
}

func TestBudgetRestoreAllManuallyPausedStillEndsShutdown(t *testing.T) {
	events := runPauseRestore(t,
		[]string{"worker", "queue"},
		[]*unstructured.Unstructured{
			pausedScaledObject("worker", 1, 5, pauseAnnosManual),
			pausedScaledObject("queue", 0, 5, pauseAnnosManual),
		},
		pauseRestoreState(pauseRestoreStateService("worker", true), pauseRestoreStateService("queue", true)),
		func(p *k8s.Provider, kk *kfake.Clientset) {
			require.Equal(t, int32(0), *pauseRestoredDeployment(t, kk, "worker").Spec.Replicas)
			require.Equal(t, int32(0), *pauseRestoredDeployment(t, kk, "queue").Spec.Replicas)
			_, ok := pauseFlapSuppressedUntil(t, kk)
			require.True(t, ok)
		})

	data := requireOneRestoredEvent(t, events)
	require.Equal(t, "0", data["restored_count"])
	require.Equal(t, "", data["restored_services"])
	require.Equal(t, "worker,queue", data["paused_services"])
	require.Equal(t, "reset", data["recovery_trigger"])
}

func TestBudgetRestoreArmedOnlyAllManuallyPausedSendsNoRestoredEvent(t *testing.T) {
	state := pauseRestoreState(pauseRestoreStateService("worker", true), pauseRestoreStateService("queue", true))
	state.ShutdownAt = nil

	events := runPauseRestore(t,
		[]string{"worker", "queue"},
		[]*unstructured.Unstructured{
			pausedScaledObject("worker", 1, 5, pauseAnnosManual),
			pausedScaledObject("queue", 0, 5, pauseAnnosManual),
		},
		state,
		func(p *k8s.Provider, kk *kfake.Clientset) {
			require.Equal(t, int32(0), *pauseRestoredDeployment(t, kk, "worker").Spec.Replicas)
			require.Equal(t, int32(0), *pauseRestoredDeployment(t, kk, "queue").Spec.Replicas)
			_, ok := pauseFlapSuppressedUntil(t, kk)
			require.False(t, ok)
		})

	require.Empty(t, events.restored())
}

func TestBudgetRestoreMarkerWithoutPausedReplicasRestoresService(t *testing.T) {
	events := runPauseRestore(t,
		[]string{"worker"},
		[]*unstructured.Unstructured{pausedScaledObject("worker", 1, 5, pauseAnnosMarker)},
		pauseRestoreState(pauseRestoreStateService("worker", true)),
		func(p *k8s.Provider, kk *kfake.Clientset) {
			d := pauseRestoredDeployment(t, kk, "worker")
			require.Equal(t, int32(3), *d.Spec.Replicas)
			require.Equal(t, int64(60), *d.Spec.Template.Spec.TerminationGracePeriodSeconds)
			_, paused := pauseGetScaledObject(t, p, "worker").GetAnnotations()[structs.KedaPausedReplicasAnnotation]
			require.False(t, paused)
		})

	data := requireOneRestoredEvent(t, events)
	require.Equal(t, "worker", data["restored_services"])
}
