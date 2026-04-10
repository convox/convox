package k8s

import (
	"context"
	"fmt"
	"time"

	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

var karpenterGVRs = []schema.GroupVersionResource{
	{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"},
	{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"},
	{Group: "karpenter.k8s.aws", Version: "v1", Resource: "ec2nodeclasses"},
}

// KarpenterCleanup removes Karpenter CRD instances and controller for clean disable.
//
// Strategy: attempt graceful drain first (delete NodePools, let the running controller
// process finalizers and terminate EC2 instances), then fall back to force cleanup
// (kill controller, strip finalizers manually) if graceful drain times out.
//
// Individual step errors are logged as warnings (best-effort). Only final verification
// returns a hard error.
func (p *Provider) KarpenterCleanup() error {
	count := p.karpenterCRDInstanceCount()
	if count == 0 {
		fmt.Println("KarpenterCleanup: no Karpenter CRD instances found, skipping")
		return nil
	}

	fmt.Printf("KarpenterCleanup: found %d Karpenter CRD instances, cleaning up\n", count)

	// Phase 1: Graceful drain — delete NodePools and let the controller handle it.
	// The controller will drain pods (PDB-respecting), terminate EC2 instances,
	// and remove finalizers from NodeClaims. This is the AWS-recommended approach.
	if p.karpenterControllerRunning() {
		fmt.Println("KarpenterCleanup: controller is running, attempting graceful drain via NodePool deletion")
		p.deleteKarpenterNodePools()
		if p.waitForNodeClaimsDrained(300 * time.Second) {
			fmt.Println("KarpenterCleanup: graceful drain succeeded")
		} else {
			fmt.Println("KarpenterCleanup: graceful drain timed out, falling back to force cleanup")
		}
	}

	// Phase 2: Force cleanup — kill controller, strip finalizers, delete everything.
	// This handles: controller not running, graceful drain timed out, or leftover resources.
	remaining := p.karpenterCRDInstanceCount()
	if remaining > 0 {
		fmt.Printf("KarpenterCleanup: %d instances remain, running force cleanup\n", remaining)

		if err := p.killKarpenterController(); err != nil {
			fmt.Printf("KarpenterCleanup WARNING: failed to kill karpenter controller: %v\n", err)
		}

		if err := p.stripKarpenterFinalizers(); err != nil {
			fmt.Printf("KarpenterCleanup WARNING: failed to strip finalizers: %v\n", err)
		}

		if err := p.deleteKarpenterCRDInstances(); err != nil {
			fmt.Printf("KarpenterCleanup WARNING: failed to delete CRD instances: %v\n", err)
		}
	}

	final := p.karpenterCRDInstanceCount()
	if final > 0 {
		return fmt.Errorf("KarpenterCleanup: %d Karpenter CRD instances remain after cleanup", final)
	}

	fmt.Println("KarpenterCleanup: all Karpenter CRD instances removed")
	return nil
}

// karpenterCRDInstanceCount returns the total count of Karpenter CRD instances across all GVR types.
// Errors from listing (e.g., CRD not installed) are silently skipped.
func (p *Provider) karpenterCRDInstanceCount() int {
	total := 0

	for _, gvr := range karpenterGVRs {
		items := p.safeListKarpenterCRD(gvr)
		total += len(items)
	}

	return total
}

// safeListKarpenterCRD lists instances of a Karpenter CRD type, returning nil on any error or panic.
// This handles both real API errors (CRD not installed) and fake client panics (GVR not registered).
func (p *Provider) safeListKarpenterCRD(gvr schema.GroupVersionResource) (items []unstructured.Unstructured) {
	defer func() {
		if r := recover(); r != nil {
			items = nil
		}
	}()

	list, err := p.DynamicClient.Resource(gvr).List(context.TODO(), am.ListOptions{})
	if err != nil {
		return nil
	}

	return list.Items
}

// karpenterControllerRunning returns true if at least one Karpenter controller pod is running.
func (p *Provider) karpenterControllerRunning() bool {
	pods, err := p.Cluster.CoreV1().Pods("kube-system").List(context.TODO(), am.ListOptions{
		LabelSelector: "app.kubernetes.io/name=karpenter",
	})
	return err == nil && len(pods.Items) > 0
}

// deleteKarpenterNodePools deletes all NodePool resources (triggering controller-managed graceful drain).
func (p *Provider) deleteKarpenterNodePools() {
	npGVR := karpenterGVRs[0] // nodepools
	items := p.safeListKarpenterCRD(npGVR)
	for _, item := range items {
		fmt.Printf("KarpenterCleanup: deleting NodePool %s (triggering graceful drain)\n", item.GetName())
		if err := p.DynamicClient.Resource(npGVR).Delete(context.TODO(), item.GetName(), am.DeleteOptions{}); err != nil {
			fmt.Printf("KarpenterCleanup WARNING: failed to delete NodePool %s: %v\n", item.GetName(), err)
		}
	}
}

// waitForNodeClaimsDrained waits for all NodeClaims to be removed by the controller.
// Returns true if all NodeClaims are gone within the timeout, false otherwise.
func (p *Provider) waitForNodeClaimsDrained(timeout time.Duration) bool {
	ncGVR := karpenterGVRs[1] // nodeclaims
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		items := p.safeListKarpenterCRD(ncGVR)
		if len(items) == 0 {
			return true
		}
		elapsed := time.Since(deadline.Add(-timeout))
		if elapsed%(30*time.Second) < 2*time.Second {
			fmt.Printf("KarpenterCleanup: %d NodeClaims remaining (%v/%v)\n", len(items), elapsed.Truncate(time.Second), timeout)
		}
		time.Sleep(2 * time.Second)
	}

	return false
}

// killKarpenterController patches the karpenter deployment in kube-system to 0 replicas,
// force-deletes its pods, and waits up to 60s for pods to terminate.
func (p *Provider) killKarpenterController() error {
	ctx := context.TODO()

	// Patch deployment replicas to 0
	patch := []byte(`{"spec":{"replicas":0}}`)
	_, err := p.Cluster.AppsV1().Deployments("kube-system").Patch(ctx, "karpenter", types.MergePatchType, patch, am.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patch karpenter deployment replicas to 0: %w", err)
	}

	// Force-delete karpenter pods
	pods, err := p.Cluster.CoreV1().Pods("kube-system").List(ctx, am.ListOptions{
		LabelSelector: "app.kubernetes.io/name=karpenter",
	})
	if err != nil {
		return fmt.Errorf("list karpenter pods: %w", err)
	}

	gracePeriod := int64(0)
	for _, pod := range pods.Items {
		err := p.Cluster.CoreV1().Pods("kube-system").Delete(ctx, pod.Name, am.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		})
		if err != nil {
			fmt.Printf("KarpenterCleanup WARNING: failed to force-delete pod %s: %v\n", pod.Name, err)
		}
	}

	// Wait up to 60s for pods to die
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		pods, err := p.Cluster.CoreV1().Pods("kube-system").List(ctx, am.ListOptions{
			LabelSelector: "app.kubernetes.io/name=karpenter",
		})
		if err != nil {
			return fmt.Errorf("list karpenter pods during wait: %w", err)
		}
		if len(pods.Items) == 0 {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("karpenter pods did not terminate within 60s")
}

// stripKarpenterFinalizers removes all finalizers from Karpenter CRD instances using a merge patch.
// Returns an error if any patch operation failed.
func (p *Provider) stripKarpenterFinalizers() error {
	ctx := context.TODO()
	patch := []byte(`{"metadata":{"finalizers":null}}`)
	var failures int

	for _, gvr := range karpenterGVRs {
		items := p.safeListKarpenterCRD(gvr)
		for _, item := range items {
			_, err := p.DynamicClient.Resource(gvr).Patch(ctx, item.GetName(), types.MergePatchType, patch, am.PatchOptions{})
			if err != nil {
				fmt.Printf("KarpenterCleanup WARNING: failed to strip finalizers from %s/%s: %v\n", gvr.Resource, item.GetName(), err)
				failures++
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("failed to strip finalizers from %d resources", failures)
	}
	return nil
}

// deleteKarpenterCRDInstances deletes all Karpenter CRD instances individually.
// Returns an error if any delete operation failed.
func (p *Provider) deleteKarpenterCRDInstances() error {
	ctx := context.TODO()
	var failures int

	for _, gvr := range karpenterGVRs {
		items := p.safeListKarpenterCRD(gvr)
		for _, item := range items {
			err := p.DynamicClient.Resource(gvr).Delete(ctx, item.GetName(), am.DeleteOptions{})
			if err != nil {
				fmt.Printf("KarpenterCleanup WARNING: failed to delete %s/%s: %v\n", gvr.Resource, item.GetName(), err)
				failures++
			}
		}
	}

	if failures > 0 {
		return fmt.Errorf("failed to delete %d resources", failures)
	}
	return nil
}
