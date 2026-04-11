package k8s_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	nodepoolGVR     = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodepools"}
	nodeclaimGVR    = schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims"}
	ec2nodeclassGVR = schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1", Resource: "ec2nodeclasses"}
)

func makeUnstructured(apiVersion, kind, name string, finalizers []string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
	if len(finalizers) > 0 {
		fins := make([]interface{}, len(finalizers))
		for i, f := range finalizers {
			fins[i] = f
		}
		obj.Object["metadata"].(map[string]interface{})["finalizers"] = fins
	}
	return obj
}

func newKarpenterTestProvider(t *testing.T, k8sObjects []runtime.Object, dynamicObjects []runtime.Object) *k8s.Provider {
	t.Helper()

	c := fake.NewSimpleClientset(k8sObjects...)

	scheme := runtime.NewScheme()
	gvrMap := map[schema.GroupVersionResource]string{
		nodepoolGVR:     "NodePoolList",
		nodeclaimGVR:    "NodeClaimList",
		ec2nodeclassGVR: "EC2NodeClassList",
	}
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrMap, dynamicObjects...)

	p := &k8s.Provider{
		Cluster:       c,
		DynamicClient: dc,
		Namespace:     "test",
		Name:          "rack1",
	}

	return p
}

func TestKarpenterCleanup_NoCRDInstances(t *testing.T) {
	p := newKarpenterTestProvider(t, nil, nil)

	err := p.KarpenterCleanup()
	require.NoError(t, err)
}

func TestKarpenterCleanup_FullCleanup(t *testing.T) {
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      "karpenter",
			Namespace: "kube-system",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &am.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/name": "karpenter"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: am.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/name": "karpenter"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "controller", Image: "karpenter:latest"},
					},
				},
			},
		},
	}

	np := makeUnstructured("karpenter.sh/v1", "NodePool", "default", []string{"karpenter.sh/termination"})
	nc := makeUnstructured("karpenter.sh/v1", "NodeClaim", "claim-1", []string{"karpenter.sh/termination"})
	ec2 := makeUnstructured("karpenter.k8s.aws/v1", "EC2NodeClass", "default", []string{"karpenter.k8s.aws/termination"})

	p := newKarpenterTestProvider(t,
		[]runtime.Object{deployment},
		[]runtime.Object{np, nc, ec2},
	)

	err := p.KarpenterCleanup()
	require.NoError(t, err)

	// Verify deployment scaled to 0
	dep, err := p.Cluster.AppsV1().Deployments("kube-system").Get(context.TODO(), "karpenter", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, int32(0), *dep.Spec.Replicas)

	// Verify all CRD instances are gone
	nps, err := p.DynamicClient.Resource(nodepoolGVR).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, nps.Items, 0)

	ncs, err := p.DynamicClient.Resource(nodeclaimGVR).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, ncs.Items, 0)

	ec2s, err := p.DynamicClient.Resource(ec2nodeclassGVR).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, ec2s.Items, 0)
}

func TestKarpenterCleanup_GracefulDrain(t *testing.T) {
	// Controller pods exist → Phase 1 (graceful drain) should execute.
	// Only a NodePool exists (no NodeClaims) → drain succeeds immediately.
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      "karpenter",
			Namespace: "kube-system",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &am.LabelSelector{
				MatchLabels: map[string]string{"app.kubernetes.io/name": "karpenter"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: am.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/name": "karpenter"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "controller", Image: "karpenter:latest"},
					},
				},
			},
		},
	}

	// Controller pods with the correct label (so karpenterControllerRunning returns true)
	pod1 := &corev1.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "karpenter-abc",
			Namespace: "kube-system",
			Labels:    map[string]string{"app.kubernetes.io/name": "karpenter"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "controller", Image: "karpenter:latest"},
			},
		},
	}

	// Only a NodePool (no NodeClaims) — graceful drain should succeed immediately
	// because waitForNodeClaimsDrained checks for NodeClaims, and there are none
	np := makeUnstructured("karpenter.sh/v1", "NodePool", "workload", []string{"karpenter.sh/termination"})

	p := newKarpenterTestProvider(t,
		[]runtime.Object{deployment, pod1},
		[]runtime.Object{np},
	)

	err := p.KarpenterCleanup()
	require.NoError(t, err)

	// Verify NodePool was deleted (graceful drain deletes NodePools)
	nps, err := p.DynamicClient.Resource(nodepoolGVR).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, nps.Items, 0, "NodePool should be deleted by graceful drain")

	// Controller pods should still exist — Phase 2 is skipped when graceful drain succeeds.
	// Terraform's Helm release destruction handles normal controller shutdown.
	pods, err := p.Cluster.CoreV1().Pods("kube-system").List(context.TODO(), am.ListOptions{
		LabelSelector: "app.kubernetes.io/name=karpenter",
	})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1, "controller pods remain for Helm to clean up")
}

func TestKarpenterCleanup_DeletesOrphanedNodesAndPods(t *testing.T) {
	// Karpenter nodes + orphaned daemonset pods should be cleaned up to prevent
	// EKS addon health checks from blocking Terraform.
	karpenterNode := &corev1.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "ip-10-1-220-93",
			Labels: map[string]string{
				"karpenter.sh/nodepool": "workload",
			},
		},
	}
	eksNode := &corev1.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "ip-10-1-116-97",
			Labels: map[string]string{
				"eks.amazonaws.com/nodegroup": "system",
			},
		},
	}
	// Daemonset pod stuck on the Karpenter node (the thing that blocks EBS CSI addon)
	stuckPod := &corev1.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "ebs-csi-node-abc123",
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			NodeName: "ip-10-1-220-93",
			Containers: []corev1.Container{
				{Name: "ebs-plugin", Image: "ebs-csi:latest"},
			},
		},
	}
	// Pod on an EKS node (should NOT be deleted)
	healthyPod := &corev1.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "ebs-csi-node-healthy",
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			NodeName: "ip-10-1-116-97",
			Containers: []corev1.Container{
				{Name: "ebs-plugin", Image: "ebs-csi:latest"},
			},
		},
	}

	np := makeUnstructured("karpenter.sh/v1", "NodePool", "workload", []string{"karpenter.sh/termination"})

	p := newKarpenterTestProvider(t,
		[]runtime.Object{karpenterNode, eksNode, stuckPod, healthyPod},
		[]runtime.Object{np},
	)

	err := p.KarpenterCleanup()
	require.NoError(t, err)

	// Karpenter node should be deleted
	nodes, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{
		LabelSelector: "karpenter.sh/nodepool",
	})
	require.NoError(t, err)
	require.Len(t, nodes.Items, 0, "Karpenter node should be deleted")

	// EKS managed node should remain
	allNodes, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, allNodes.Items, 1, "EKS managed node should remain")

	// Stuck pod on Karpenter node should be deleted, healthy pod should remain
	pods, err := p.Cluster.CoreV1().Pods("kube-system").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1, "only the healthy pod should remain")
	require.Equal(t, "ebs-csi-node-healthy", pods.Items[0].Name)
}

func TestKarpenterCleanup_CRDsNotInstalled(t *testing.T) {
	// Create a dynamic client with NO registered GVRs
	scheme := runtime.NewScheme()
	dc := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, nil)

	p := &k8s.Provider{
		Cluster:       fake.NewSimpleClientset(),
		DynamicClient: dc,
		Namespace:     "test",
		Name:          "rack1",
	}

	err := p.KarpenterCleanup()
	require.NoError(t, err)
}

func TestKarpenterCleanup_TerminatesEC2Instances(t *testing.T) {
	// Karpenter nodes with AWS providerID should trigger EC2 termination
	karpenterNode := &corev1.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "ip-10-1-220-93",
			Labels: map[string]string{
				"karpenter.sh/nodepool": "workload",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: "aws:///us-east-1a/i-0abc123def456",
		},
	}

	np := makeUnstructured("karpenter.sh/v1", "NodePool", "workload", []string{"karpenter.sh/termination"})

	var terminatedIDs []string
	p := newKarpenterTestProvider(t,
		[]runtime.Object{karpenterNode},
		[]runtime.Object{np},
	)
	p.TerminateKarpenterEC2 = func(instanceIDs []string) error {
		terminatedIDs = instanceIDs
		return nil
	}

	err := p.KarpenterCleanup()
	require.NoError(t, err)

	require.Equal(t, []string{"i-0abc123def456"}, terminatedIDs)

	nodes, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{
		LabelSelector: "karpenter.sh/nodepool",
	})
	require.NoError(t, err)
	require.Len(t, nodes.Items, 0, "Karpenter node should be deleted")
}

func TestKarpenterCleanup_EC2FailureContinuesCleanup(t *testing.T) {
	// EC2 termination failure should not prevent node/pod cleanup
	karpenterNode := &corev1.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "ip-10-1-220-93",
			Labels: map[string]string{
				"karpenter.sh/nodepool": "workload",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: "aws:///us-east-1a/i-0abc123def456",
		},
	}
	stuckPod := &corev1.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "ebs-csi-node-abc123",
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			NodeName: "ip-10-1-220-93",
			Containers: []corev1.Container{
				{Name: "ebs-plugin", Image: "ebs-csi:latest"},
			},
		},
	}

	np := makeUnstructured("karpenter.sh/v1", "NodePool", "workload", []string{"karpenter.sh/termination"})

	p := newKarpenterTestProvider(t,
		[]runtime.Object{karpenterNode, stuckPod},
		[]runtime.Object{np},
	)
	p.TerminateKarpenterEC2 = func(instanceIDs []string) error {
		return fmt.Errorf("AccessDenied: not authorized to terminate instances")
	}

	err := p.KarpenterCleanup()
	require.NoError(t, err)

	// Node should still be cleaned up despite EC2 termination failure
	nodes, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{
		LabelSelector: "karpenter.sh/nodepool",
	})
	require.NoError(t, err)
	require.Len(t, nodes.Items, 0, "node should be deleted despite EC2 failure")

	// Stuck pod should also be cleaned up
	pods, err := p.Cluster.CoreV1().Pods("kube-system").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pods.Items, 0, "stuck pod should be deleted despite EC2 failure")
}
