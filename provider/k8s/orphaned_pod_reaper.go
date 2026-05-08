package k8s

import (
	"context"
	"fmt"
	"time"

	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// runOrphanedPodReaper is the rack-side recovery loop for pods stuck in
// Terminating on NotReady nodes. The trigger scenario is kubelet death
// (e.g. a workload pod overcommits the node and the kubelet is OOM-killed):
// every pod on the dead node has a DeletionTimestamp set by the controller
// that wanted them gone, but the kubelet can't ack the deletion. The pod
// stays in Terminating forever. For StatefulSet workloads, the new instance
// can't schedule until the old pod's record is gone (PVC reattachment
// blocks). Force-deleting with grace=0 unblocks the scheduler.
//
// The reaper acts only when ALL three hold:
//   - node.Status.Conditions[NodeReady].Status != ConditionTrue (False or Unknown)
//   - LastTransitionTime older than orphanedPodReaperNotReadyThreshold
//   - pod.DeletionTimestamp is set
//
// A Ready node with a pod-in-graceful-shutdown is left alone (graceful is
// the normal path). A NotReady node where no pod has been asked to delete
// is left alone (no one wants those pods gone). Brief NotReady blips under
// the threshold are left alone (kubelet may recover).
//
// The 60s tick + 5min threshold means recovery takes 5-6 minutes from
// kubelet death — slow enough to avoid stomping on transient blips,
// fast enough that an operator's `kubectl get pods` shows progress
// within a reasonable window.
func (p *Provider) runOrphanedPodReaper(ctx context.Context) {
	tick := time.NewTicker(orphanedPodReaperTickInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reapOrphanedTerminatingPods(ctx, orphanedPodReaperNotReadyThreshold)
		}
	}
}

// reapOrphanedTerminatingPods is one reaper pass. Lists nodes, for each
// not-ready-past-threshold node lists pods on the node, force-deletes
// every pod that already has a DeletionTimestamp set. A defer-recover
// shields the outer goroutine: a panic here (e.g. nil-pointer in client-go
// during an apiserver disconnect) would otherwise kill the reaper for the
// remaining lifetime of the rack process, silently disabling recovery
// from kubelet-OOM deadlocks.
func (p *Provider) reapOrphanedTerminatingPods(ctx context.Context, threshold time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=orphaned_pod_reaper at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	nodes, err := p.Cluster.CoreV1().Nodes().List(ctx, am.ListOptions{})
	if err != nil {
		fmt.Printf("ns=orphaned_pod_reaper at=warn kind=list_nodes err=%q\n", err)
		return
	}

	cutoff := time.Now().Add(-threshold)
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if !nodeNotReadyPastCutoff(node, cutoff) {
			continue
		}
		pods, err := p.Cluster.CoreV1().Pods("").List(ctx, am.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.Name),
		})
		if err != nil {
			fmt.Printf("ns=orphaned_pod_reaper at=warn kind=list_pods node=%s err=%q\n", node.Name, err)
			continue
		}
		for j := range pods.Items {
			pod := &pods.Items[j]
			if pod.DeletionTimestamp == nil {
				continue
			}
			if err := p.forceDeletePod(ctx, pod); err != nil {
				fmt.Printf("ns=orphaned_pod_reaper at=warn kind=force_delete_failed rack=%s ns=%s pod=%s node=%s err=%q\n",
					p.Name, pod.Namespace, pod.Name, node.Name, err)
				continue
			}
			fmt.Printf("ns=orphaned_pod_reaper at=info kind=force_deleted rack=%s ns=%s pod=%s node=%s\n",
				p.Name, pod.Namespace, pod.Name, node.Name)
		}
	}
}

// nodeNotReadyPastCutoff returns true iff the node's NodeReady condition
// is not True (False or Unknown) AND the LastTransitionTime is older than
// the cutoff. A node without any NodeReady condition (just-created, never
// reported in) is treated as Ready: don't reap pods on a node that may
// just be booting.
func nodeNotReadyPastCutoff(node *ac.Node, cutoff time.Time) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != ac.NodeReady {
			continue
		}
		if cond.Status == ac.ConditionTrue {
			return false
		}
		// ConditionFalse or ConditionUnknown both mean "kubelet is not
		// telling us it's healthy". Both qualify if the transition is
		// old enough.
		return cond.LastTransitionTime.Time.Before(cutoff)
	}
	return false
}
