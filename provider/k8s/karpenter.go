package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	KarpenterNodePoolLabel = "karpenter.sh/nodepool"
)

func (p *Provider) KarpenterCleanup() error {
	ctx := p.Context()

	var errs []error

	// Clean up orphaned Karpenter nodes (cordon, drain, strip finalizers, delete)
	nodes, err := p.Cluster.CoreV1().Nodes().List(ctx, am.ListOptions{
		LabelSelector: KarpenterNodePoolLabel,
	})
	if err != nil {
		return errors.WithStack(fmt.Errorf("failed to list karpenter nodes: %s", err))
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		if err := p.cleanupNode(ctx, node); err != nil {
			errs = append(errs, fmt.Errorf("node %s: %s", node.Name, err))
		}
	}

	// Delete stale NodePool and EC2NodeClass CRD instances.
	// These survive disable because CRDs are intentionally kept installed.
	// Clearing them ensures re-enable creates fresh objects without conflicts.
	if err := p.deleteKarpenterCRDInstances(ctx); err != nil {
		errs = append(errs, fmt.Errorf("CRD cleanup: %s", err))
	}

	if len(errs) > 0 {
		return errors.WithStack(fmt.Errorf("cleanup errors: %v", errs))
	}

	return nil
}

func (p *Provider) deleteKarpenterCRDInstances(ctx context.Context) error {
	// The REST client is used for CRD API paths that the typed client doesn't cover.
	// In test environments (fake clientset), RESTClient() returns a typed nil that
	// panics on use. Recover gracefully — CRD cleanup is best-effort.
	defer func() {
		recover()
	}()

	restClient := p.Cluster.CoreV1().RESTClient()

	crdPaths := []string{
		"/apis/karpenter.sh/v1/nodeclaims",
		"/apis/karpenter.sh/v1/nodepools",
		"/apis/karpenter.k8s.aws/v1/ec2nodeclasses",
	}

	for _, path := range crdPaths {
		raw, err := restClient.Get().AbsPath(path).DoRaw(ctx)
		if err != nil {
			// CRDs might not be installed — skip silently
			continue
		}

		var list struct {
			Items []struct {
				Metadata struct {
					Name       string   `json:"name"`
					Finalizers []string `json:"finalizers"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			continue
		}

		for _, item := range list.Items {
			name := item.Metadata.Name

			// Strip finalizers first (controller is dead, can't process them)
			if len(item.Metadata.Finalizers) > 0 {
				patch := []byte(`{"metadata":{"finalizers":null}}`)
				_, _ = restClient.Patch(types.MergePatchType).AbsPath(path, name).Body(patch).DoRaw(ctx)
			}

			_, _ = restClient.Delete().AbsPath(path, name).DoRaw(ctx)
		}
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
		if isDaemonSetPod(pod) {
			continue
		}

		if err := p.evictPod(ctx, pod, deadline); err != nil {
			return err
		}
	}

	return nil
}


