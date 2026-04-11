package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func karpenterTestProvider() (*Provider, *fake.Clientset) {
	c := fake.NewSimpleClientset()
	cc := cvfake.NewSimpleClientset()
	mc := metricfake.NewSimpleClientset()
	a := &atom.MockInterface{}

	p := &Provider{
		Atom:          a,
		Cluster:       c,
		Convox:        cc,
		Domain:        "domain1",
		Engine:        &mock.TestEngine{},
		MetricsClient: mc,
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
		ctx:           context.Background(),
	}

	return p, c
}

func TestKarpenterCleanupNoNodes(t *testing.T) {
	p, _ := karpenterTestProvider()

	err := p.KarpenterCleanup()
	require.NoError(t, err)
}

func TestKarpenterCleanupRemovesKarpenterNodes(t *testing.T) {
	p, c := karpenterTestProvider()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name:   "karpenter-node-1",
			Labels: map[string]string{"karpenter.sh/nodepool": "default"},
		},
		Spec: ac.NodeSpec{ProviderID: "aws:///us-east-1a/i-abc123"},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.KarpenterCleanup()
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Get(context.TODO(), "karpenter-node-1", am.GetOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestKarpenterCleanupSkipsDaemonSetPods(t *testing.T) {
	p, c := karpenterTestProvider()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name:   "karpenter-node-2",
			Labels: map[string]string{"karpenter.sh/nodepool": "workload"},
		},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("default").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: "app-pod", Namespace: "default"},
		Spec:       ac.PodSpec{NodeName: "karpenter-node-2"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("kube-system").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "ds-pod",
			Namespace: "kube-system",
			OwnerReferences: []am.OwnerReference{
				{Kind: "DaemonSet", Name: "fluentd", APIVersion: "apps/v1"},
			},
		},
		Spec: ac.PodSpec{NodeName: "karpenter-node-2"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.KarpenterCleanup()
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Get(context.TODO(), "karpenter-node-2", am.GetOptions{})
	require.Error(t, err)

	_, err = c.CoreV1().Pods("kube-system").Get(context.TODO(), "ds-pod", am.GetOptions{})
	require.NoError(t, err)
}

func TestKarpenterCleanupSkipsMirrorPods(t *testing.T) {
	p, c := karpenterTestProvider()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name:   "karpenter-node-3",
			Labels: map[string]string{"karpenter.sh/nodepool": "workload"},
		},
		Status: ac.NodeStatus{
			Conditions: []ac.NodeCondition{
				{Type: ac.NodeReady, Status: ac.ConditionTrue},
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("kube-system").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      "kube-apiserver-mirror",
			Namespace: "kube-system",
			Annotations: map[string]string{
				ac.MirrorPodAnnotationKey: "mirror-hash",
			},
		},
		Spec: ac.PodSpec{NodeName: "karpenter-node-3"},
	}, am.CreateOptions{})
	require.NoError(t, err)

	err = p.KarpenterCleanup()
	require.NoError(t, err)

	_, err = c.CoreV1().Pods("kube-system").Get(context.TODO(), "kube-apiserver-mirror", am.GetOptions{})
	require.NoError(t, err)
}
