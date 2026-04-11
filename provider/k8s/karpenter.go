package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	KarpenterNodePoolLabel = "karpenter.sh/nodepool"
	drainTimeout           = 5 * time.Minute
	evictionRetryInterval  = 5 * time.Second
)

func (p *Provider) KarpenterCleanup() error {
	ctx := p.Context()

	nodes, err := p.Cluster.CoreV1().Nodes().List(ctx, am.ListOptions{
		LabelSelector: KarpenterNodePoolLabel,
	})
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to list karpenter nodes: %s", err))
	}

	if len(nodes.Items) == 0 {
		return nil
	}

	var errs []error
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if err := p.cleanupNode(ctx, node); err != nil {
			errs = append(errs, fmt.Errorf("node %s: %s", node.Name, err))
		}
	}

	if len(errs) > 0 {
		return errors.WithStack(fmt.Errorf("cleanup errors: %v", errs))
	}

	return nil
}

func (p *Provider) cleanupNode(ctx context.Context, node *ac.Node) error {
	if !node.Spec.Unschedulable {
		patch := []byte(`{"spec":{"unschedulable":true}}`)
		if _, err := p.Cluster.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patch, am.PatchOptions{}); err != nil {
			return errors.WithStack(fmt.Errorf("failed to cordon: %s", err))
		}
	}

	if err := p.drainKarpenterNode(ctx, node.Name); err != nil {
		return errors.WithStack(fmt.Errorf("failed to drain: %s", err))
	}

	// Strip Karpenter finalizer — the controller that would process it is dead.
	finalizerPatch := []byte(`{"metadata":{"finalizers":null}}`)
	if _, err := p.Cluster.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, finalizerPatch, am.PatchOptions{}); err != nil && !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("failed to strip finalizers: %s", err))
	}

	if err := p.Cluster.CoreV1().Nodes().Delete(ctx, node.Name, am.DeleteOptions{}); err != nil && !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("failed to delete node: %s", err))
	}

	return nil
}

func (p *Provider) drainKarpenterNode(ctx context.Context, nodeName string) error {
	pods, err := p.Cluster.CoreV1().Pods("").List(ctx, am.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	deadline := time.Now().Add(drainTimeout)

	for i := range pods.Items {
		pod := &pods.Items[i]

		if _, isMirror := pod.Annotations[ac.MirrorPodAnnotationKey]; isMirror {
			continue
		}
		if isDaemonSetManaged(pod) {
			continue
		}

		if err := p.evictPodWithRetry(ctx, pod, deadline); err != nil {
			return err
		}
	}

	return nil
}

func isDaemonSetManaged(pod *ac.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func (p *Provider) evictPodWithRetry(ctx context.Context, pod *ac.Pod, deadline time.Time) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: am.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}

	for {
		err := p.Cluster.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
		if err == nil {
			return nil
		}

		if ae.IsNotFound(err) {
			return nil
		}

		if ae.IsTooManyRequests(err) {
			if time.Now().After(deadline) {
				return p.forceDeletePod(ctx, pod)
			}
			time.Sleep(evictionRetryInterval)
			continue
		}

		return p.forceDeletePod(ctx, pod)
	}
}

func (p *Provider) forceDeletePod(ctx context.Context, pod *ac.Pod) error {
	grace := int64(0)
	err := p.Cluster.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, am.DeleteOptions{
		GracePeriodSeconds: &grace,
	})
	if err != nil && !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("failed to force-delete pod %s/%s: %s", pod.Namespace, pod.Name, err))
	}
	return nil
}
