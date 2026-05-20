package k8s

import (
	"context"
	"fmt"
	"time"

	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Force-deletes pods stuck in Terminating on NotReady nodes (kubelet death
// leaves pods that block StatefulSet PVC reattachment).
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

func nodeNotReadyPastCutoff(node *ac.Node, cutoff time.Time) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != ac.NodeReady {
			continue
		}
		if cond.Status == ac.ConditionTrue {
			return false
		}
		return cond.LastTransitionTime.Time.Before(cutoff)
	}
	return false
}
