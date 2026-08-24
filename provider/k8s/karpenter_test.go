package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

func nodePoolObject(name, label string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "karpenter.sh/v1",
		"kind":       "NodePool",
		"metadata":   map[string]interface{}{"name": name},
	}}

	if label != "" {
		u.Object["spec"] = map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{NodePoolLabel: label},
				},
			},
		}
	}

	return u
}

func nodePoolTestProvider(objs ...runtime.Object) *Provider {
	p, _ := karpenterTestProvider()

	p.IsKarpenterEnabled = true
	p.DynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		nodePoolGVR: "NodePoolList",
	}, objs...)

	return p
}

// Pool names are deliberately reverse-sorted against their labels, so this fails
// if the label read is swapped for metadata.name or if the sort is dropped.
func TestKarpenterNodePoolsReadsLabelNotNameAndSorts(t *testing.T) {
	p := nodePoolTestProvider(nodePoolObject("aaa", "zebra"), nodePoolObject("zzz", "alpha"))

	pools, ok := p.karpenterNodePools()
	require.True(t, ok)
	require.Equal(t, []string{"alpha", "zebra"}, pools)
}

func TestKarpenterNodePoolsSkipsPoolsWithoutLabel(t *testing.T) {
	p := nodePoolTestProvider(nodePoolObject("workload", "workload"), nodePoolObject("handmade", ""))

	pools, ok := p.karpenterNodePools()
	require.True(t, ok)
	require.Equal(t, []string{"workload"}, pools)
}

func TestKarpenterNodePoolsUndeterminable(t *testing.T) {
	listErr := func(err error) func(*Provider) {
		return func(p *Provider) {
			if dc, ok := p.DynamicClient.(*dynamicfake.FakeDynamicClient); ok {
				dc.PrependReactor("list", "nodepools", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, err
				})
			}
		}
	}

	for _, tc := range []struct {
		name   string
		objs   []runtime.Object
		mutate func(*Provider)
	}{
		{name: "karpenter disabled", objs: []runtime.Object{nodePoolObject("workload", "workload")}, mutate: func(p *Provider) { p.IsKarpenterEnabled = false }},
		{name: "nil dynamic client", objs: []runtime.Object{nodePoolObject("workload", "workload")}, mutate: func(p *Provider) { p.DynamicClient = nil }},
		{name: "empty list", objs: nil},
		{name: "crd absent", objs: []runtime.Object{nodePoolObject("workload", "workload")}, mutate: listErr(kerr.NewNotFound(schema.GroupResource{Group: "karpenter.sh", Resource: "nodepools"}, ""))},
		{name: "generic list error", objs: []runtime.Object{nodePoolObject("workload", "workload")}, mutate: listErr(fmt.Errorf("connection refused"))},
		{name: "no pool carries the label", objs: []runtime.Object{nodePoolObject("handmade", "")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := nodePoolTestProvider(tc.objs...)
			if tc.mutate != nil {
				tc.mutate(p)
			}

			pools, ok := p.karpenterNodePools()
			require.False(t, ok)
			require.Nil(t, pools)

			require.NoError(t, p.validateManifestNodePools(&manifest.Manifest{
				Services: manifest.Services{{Name: "web", NodeSelectorLabels: manifest.Labels{NodePoolLabel: "nonexistent"}}},
			}))
		})
	}
}

func TestValidateManifestNodePools(t *testing.T) {
	p := nodePoolTestProvider(nodePoolObject("workload", "workload"), nodePoolObject("gpu", "gpu"))

	for _, tc := range []struct {
		name   string
		labels manifest.Labels
		err    string
	}{
		{name: "existing pool", labels: manifest.Labels{NodePoolLabel: "gpu"}},
		{name: "no labels"},
		{name: "unrelated key only", labels: manifest.Labels{"convox.io/label": "analytics", "topology.kubernetes.io/zone": "us-east-1a"}},
		{name: "empty value", labels: manifest.Labels{NodePoolLabel: ""}},
		{name: "unknown pool", labels: manifest.Labels{NodePoolLabel: "gpu-lg"}, err: `service "web" targets Karpenter node pool "gpu-lg"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := p.validateManifestNodePools(&manifest.Manifest{
				Services: manifest.Services{{Name: "web", NodeSelectorLabels: tc.labels}},
			})

			if tc.err == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.err)
			require.Contains(t, err.Error(), "Existing pools: gpu, workload")
		})
	}
}

func TestValidateManifestNodePoolsReportsOffendingService(t *testing.T) {
	p := nodePoolTestProvider(nodePoolObject("workload", "workload"))

	err := p.validateManifestNodePools(&manifest.Manifest{
		Services: manifest.Services{
			{Name: "web", NodeSelectorLabels: manifest.Labels{NodePoolLabel: "workload"}},
			{Name: "worker", NodeSelectorLabels: manifest.Labels{NodePoolLabel: "batch"}},
		},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), `service "worker"`)
}

func releasePromoteNodePoolProvider(t *testing.T, badManifestRelease string) *Provider {
	t.Helper()
	t.Setenv("TEST", "true")

	p, kk, kc := minimalProvider(t)

	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{Name: p.Namespace},
	}, am.CreateOptions{})
	require.NoError(t, err)
	require.NoError(t, p.Initialize(structs.ProviderOptions{}))

	createAppNamespace(t, kk, "rack1", "app1")
	// A promote that reaches Apply spawns a watcher holding a package-global
	// slot. A terminal app-status plus a short tick retire it immediately
	// instead of leaving it to poll to its deadline.
	t.Cleanup(SetReleasePromoteWatchPollIntervalForTest(20 * time.Millisecond))
	setAppStatus(t, kk, "rack1-app1", "Running")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)
	createRelease(t, kc, "rack1-app1", badManifestRelease, manifestWithNodeSelectors)

	aa, ok := p.Atom.(*atom.MockInterface)
	require.True(t, ok)
	aa.On("Apply", tmock.Anything, tmock.Anything, tmock.Anything).Return(nil).Maybe()

	p.IsKarpenterEnabled = true
	p.DynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		nodePoolGVR: "NodePoolList",
	}, nodePoolObject("workload", "workload"))

	return p
}

// manifestWithNodeSelectors pins gpu-worker to convox.io/nodepool=gpu, which the
// provider above does not have, so a promote of any release carrying it is rejected.
const nodePoolRejection = `targets Karpenter node pool "gpu"`

func TestReleasePromote_RejectsUnknownNodePool(t *testing.T) {
	p := releasePromoteNodePoolProvider(t, "rel2")

	err := p.ReleasePromote("app1", "rel2", structs.ReleasePromoteOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), nodePoolRejection)
	require.Contains(t, err.Error(), `service "gpu-worker"`)
}

func TestReleasePromote_SkipsNodePoolCheckOnCurrentRelease(t *testing.T) {
	p := releasePromoteNodePoolProvider(t, "rel2")

	// minimalProvider's Atom mock reports rel1 as current, so this is the
	// AppUpdate re-promote path that apps params set and apps lock use.
	err := p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{})
	if err != nil {
		require.NotContains(t, err.Error(), nodePoolRejection)
	}
}

func TestReleasePromote_SkipsNodePoolCheckOnForce(t *testing.T) {
	p := releasePromoteNodePoolProvider(t, "rel2")

	err := p.ReleasePromote("app1", "rel2", structs.ReleasePromoteOptions{Force: options.Bool(true)})
	if err != nil {
		require.NotContains(t, err.Error(), nodePoolRejection)
	}
}

// TestReleasePromote_StartsTheWatcher pins the launch call site: a promote must
// still start a watcher, and that watcher clears the annotation the promote wrote.
func TestReleasePromote_StartsTheWatcher(t *testing.T) {
	p := releasePromoteNodePoolProvider(t, "rel2")
	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)

	// The watch slot is package-global, so an earlier test's watcher for this
	// same pair would make the launch a no-op and mask the assertion.
	require.Eventually(t, func() bool {
		return !releasePromoteWatchSlotHeldForTest("app1", "rel1")
	}, 5*time.Second, 10*time.Millisecond, "watch slot still held by an earlier test")

	if err := p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{}); err != nil {
		require.NotContains(t, err.Error(), nodePoolRejection)
	}

	require.Eventually(t, func() bool {
		ns, err := kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		return err == nil && ns.Annotations[structs.ReleasePromoteWatchAnnotation] == ""
	}, 3*time.Second, 20*time.Millisecond,
		"the promote path must start a watcher that resolves and clears its annotation")
}
