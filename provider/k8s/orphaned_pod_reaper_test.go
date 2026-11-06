package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// reaperTestProvider builds a Provider whose Cluster is a fresh in-memory
// fake clientset. Internal-package tests (package k8s, not k8s_test) so
// reapOrphanedTerminatingPods stays unexported.
//
// The fake clientset does not honor FieldSelector, so we install a list
// reactor that parses spec.nodeName=X out of the list options and filters
// the returned PodList accordingly. Real apiservers do this; the test
// scaffolding has to too, otherwise per-node scope can't be validated.
func reaperTestProvider() (*Provider, *fake.Clientset) {
	c := fake.NewSimpleClientset()
	c.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		la, ok := action.(k8stesting.ListActionImpl)
		if !ok {
			return false, nil, nil
		}
		fs := la.GetListRestrictions().Fields
		if fs == nil || fs.Empty() {
			return false, nil, nil
		}
		// Extract spec.nodeName=<val> from the FieldSelector string.
		want := ""
		for _, term := range strings.Split(fs.String(), ",") {
			if strings.HasPrefix(term, "spec.nodeName=") {
				want = strings.TrimPrefix(term, "spec.nodeName=")
				break
			}
		}
		if want == "" {
			return false, nil, nil
		}
		// List everything from the tracker, then filter to pods on `want`.
		raw, err := c.Tracker().List(la.GetResource(), la.GetKind(), la.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		filtered := &ac.PodList{}
		_ = meta.EachListItem(raw, func(obj runtime.Object) error {
			pod, ok := obj.(*ac.Pod)
			if !ok {
				return nil
			}
			if pod.Spec.NodeName == want {
				filtered.Items = append(filtered.Items, *pod)
			}
			return nil
		})
		return true, filtered, nil
	})
	return &Provider{
		Cluster: c,
		Name:    "rack1",
	}, c
}

func makeNode(name string, ready ac.ConditionStatus, transitioned time.Time) *ac.Node {
	return &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: name},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{
					Type:               ac.NodeReady,
					Status:             ready,
					LastTransitionTime: am.NewTime(transitioned),
				},
			},
		},
	}
}

func makePod(name, namespace, nodeName string, deletionTime *time.Time) *ac.Pod {
	pod := &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ac.PodSpec{NodeName: nodeName},
	}
	if deletionTime != nil {
		dt := am.NewTime(*deletionTime)
		pod.DeletionTimestamp = &dt
	}
	return pod
}

// podStillExists returns true if the pod is still present in the fake
// clientset. The reaper issues Delete with grace=0; the fake clientset
// removes the object on Delete (it does not honor finalizers), so a
// successfully-reaped pod is gone from the store.
func podStillExists(t *testing.T, c *fake.Clientset, namespace, name string) bool {
	t.Helper()
	_, err := c.CoreV1().Pods(namespace).Get(context.TODO(), name, am.GetOptions{})
	return err == nil
}

// TestReapOrphanedTerminatingPods_NodeReady_NoOp confirms that a pod with
// a DeletionTimestamp on a Ready node is left alone. That pod is in
// graceful-shutdown — the kubelet is alive and processing the deletion
// normally. The reaper must not stomp on the graceful path.
func TestReapOrphanedTerminatingPods_NodeReady_NoOp(t *testing.T) {
	p, c := reaperTestProvider()

	deletion := time.Now()
	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("ready-node", ac.ConditionTrue, time.Now().Add(-time.Hour)),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("graceful-pod", "default", "ready-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.True(t, podStillExists(t, c, "default", "graceful-pod"),
		"pod on Ready node must not be force-deleted by reaper")
}

// TestReapOrphanedTerminatingPods_NotReadyButRecent_NoOp confirms the
// threshold gate. A NotReady node whose transition time is younger than
// the threshold may have a kubelet that is about to recover; reaping
// would short-circuit normal kubelet recovery.
func TestReapOrphanedTerminatingPods_NotReadyButRecent_NoOp(t *testing.T) {
	p, c := reaperTestProvider()

	// transitioned 1 minute ago, threshold is 5 minutes
	transitioned := time.Now().Add(-1 * time.Minute)
	deletion := time.Now()

	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("flapping-node", ac.ConditionFalse, transitioned),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("recent-pod", "default", "flapping-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.True(t, podStillExists(t, c, "default", "recent-pod"),
		"pod on recently-NotReady node must not be force-deleted before threshold")
}

// TestReapOrphanedTerminatingPods_NotReadyNoDeletion_NoOp confirms that
// a pod without a DeletionTimestamp is left alone even on a long-NotReady
// node. The reaper only acts on pods someone has asked to delete.
func TestReapOrphanedTerminatingPods_NotReadyNoDeletion_NoOp(t *testing.T) {
	p, c := reaperTestProvider()

	transitioned := time.Now().Add(-30 * time.Minute)

	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("dead-node", ac.ConditionFalse, transitioned),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("running-pod", "default", "dead-node", nil),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.True(t, podStillExists(t, c, "default", "running-pod"),
		"pod without DeletionTimestamp must not be force-deleted regardless of node state")
}

// TestReapOrphanedTerminatingPods_NotReadyAndStuckTerminating_ForceDeletes
// exercises the recovery path. NotReady > threshold + DeletionTimestamp set
// is the kube-prometheus deadlock signature: kubelet is dead, controller asked
// for deletion, kubelet can never ack. The reaper force-deletes (grace=0) so
// a StatefulSet-managed PVC can re-attach to a freshly-scheduled replacement.
func TestReapOrphanedTerminatingPods_NotReadyAndStuckTerminating_ForceDeletes(t *testing.T) {
	p, c := reaperTestProvider()

	transitioned := time.Now().Add(-30 * time.Minute)
	deletion := time.Now().Add(-10 * time.Minute)

	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("dead-node", ac.ConditionFalse, transitioned),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("convox-monitoring").Create(context.TODO(),
		makePod("prometheus-0", "convox-monitoring", "dead-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.False(t, podStillExists(t, c, "convox-monitoring", "prometheus-0"),
		"pod stuck Terminating on long-NotReady node must be force-deleted")
}

// TestReapOrphanedTerminatingPods_UnknownReadiness_TreatedAsNotReady
// covers the kubelet-not-heartbeating case. NodeReady=Unknown is
// equivalent to NotReady from the reaper's perspective: the kubelet
// is not telling us it's healthy, so it cannot ack pod deletions.
func TestReapOrphanedTerminatingPods_UnknownReadiness_TreatedAsNotReady(t *testing.T) {
	p, c := reaperTestProvider()

	transitioned := time.Now().Add(-30 * time.Minute)
	deletion := time.Now().Add(-10 * time.Minute)

	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("silent-node", ac.ConditionUnknown, transitioned),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("orphan-pod", "default", "silent-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.False(t, podStillExists(t, c, "default", "orphan-pod"),
		"pod stuck Terminating on Unknown-readiness node must be force-deleted (treated same as NotReady)")
}

// TestReapOrphanedTerminatingPods_PodOnReadyNode_LeftAlone confirms the
// per-node scope. A mixed cluster where one node is dead and another is
// healthy must not cause the reaper to drag healthy-node pods into the reap.
func TestReapOrphanedTerminatingPods_PodOnReadyNode_LeftAlone(t *testing.T) {
	p, c := reaperTestProvider()

	deadTransition := time.Now().Add(-30 * time.Minute)
	deletion := time.Now().Add(-10 * time.Minute)

	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("dead-node", ac.ConditionFalse, deadTransition),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Create(context.TODO(),
		makeNode("live-node", ac.ConditionTrue, time.Now().Add(-time.Hour)),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("orphan-pod", "default", "dead-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("graceful-pod", "default", "live-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.False(t, podStillExists(t, c, "default", "orphan-pod"),
		"orphan pod on dead node must be reaped")
	require.True(t, podStillExists(t, c, "default", "graceful-pod"),
		"graceful pod on live node must NOT be reaped")
}

// TestReapOrphanedTerminatingPods_NodeWithoutReadyCondition_NoOp covers the
// just-created-node defensive path. A node that has not yet reported a
// NodeReady condition is treated as Ready (do nothing) — bootstrapping
// nodes should not have their workloads stomped on.
func TestReapOrphanedTerminatingPods_NodeWithoutReadyCondition_NoOp(t *testing.T) {
	p, c := reaperTestProvider()

	deletion := time.Now().Add(-10 * time.Minute)

	// node with empty conditions slice
	_, err := c.CoreV1().Nodes().Create(context.TODO(),
		&ac.Node{ObjectMeta: am.ObjectMeta{Name: "booting-node"}},
		am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(),
		makePod("starting-pod", "default", "booting-node", &deletion),
		am.CreateOptions{})
	require.NoError(t, err)

	p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)

	require.True(t, podStillExists(t, c, "default", "starting-pod"),
		"pod on a node without NodeReady condition must not be force-deleted")
}

// TestReapOrphanedTerminatingPods_PanicInClientSurvives confirms the
// outer reaper goroutine survives a panic from any nested client call.
// A panic here would otherwise kill the goroutine for the rest of the
// rack process's lifetime, silently disabling stuck-pod recovery.
//
// Reproduce by injecting a reactor that panics on Nodes().List(); confirm
// the call returns (recover fired) without propagating the panic.
func TestReapOrphanedTerminatingPods_PanicInClientSurvives(t *testing.T) {
	p, c := reaperTestProvider()

	c.PrependReactor("list", "nodes", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		panic("simulated client-go corruption mid-list")
	})

	require.NotPanics(t, func() {
		p.reapOrphanedTerminatingPods(context.TODO(), 5*time.Minute)
	}, "reaper must recover from panics inside Cluster.CoreV1().Nodes().List")
}
