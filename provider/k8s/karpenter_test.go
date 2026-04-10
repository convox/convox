package k8s_test

import (
	"context"
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
