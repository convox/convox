package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestImdsBlockNetworkPolicy(t *testing.T) {
	np := imdsBlockNetworkPolicy("app-myapp")

	assert.Equal(t, imdsBlockPolicyName, np.Name)
	assert.Equal(t, "app-myapp", np.Namespace)
	assert.Equal(t, "convox", np.Labels["system"])

	require.Len(t, np.Spec.PolicyTypes, 1)
	assert.Equal(t, networkingv1.PolicyTypeEgress, np.Spec.PolicyTypes[0])
	assert.Empty(t, np.Spec.PodSelector.MatchLabels)
	assert.Empty(t, np.Spec.PodSelector.MatchExpressions)

	require.Len(t, np.Spec.Egress, 1)
	require.Len(t, np.Spec.Egress[0].To, 1)
	assert.Empty(t, np.Spec.Egress[0].Ports)
	assert.Empty(t, np.Spec.Ingress)
	ipb := np.Spec.Egress[0].To[0].IPBlock
	require.NotNil(t, ipb)
	assert.Equal(t, "0.0.0.0/0", ipb.CIDR)
	assert.Equal(t, []string{"169.254.169.254/32"}, ipb.Except)
}

func newAppNamespace(name, rack string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: am.ObjectMeta{
		Name:   name,
		Labels: map[string]string{"system": "convox", "rack": rack, "type": "app", "name": name},
	}}
}

func TestReconcilePodImdsBlock_EnabledCreatesScoped(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newAppNamespace("app-a", "rack1"),
		newAppNamespace("app-b", "rack1"),
		newAppNamespace("app-foreign", "rack2"),
		&corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1", Labels: map[string]string{"system": "convox", "rack": "rack1", "type": "system"}}},
	)
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))
	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	for _, ns := range []string{"app-a", "app-b"} {
		got, err := cs.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
		require.NoError(t, err, "policy should exist in %s", ns)
		require.Len(t, got.Spec.Egress, 1)
		require.Len(t, got.Spec.Egress[0].To, 1)
		require.NotNil(t, got.Spec.Egress[0].To[0].IPBlock)
		assert.Equal(t, []string{"169.254.169.254/32"}, got.Spec.Egress[0].To[0].IPBlock.Except)
		assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, got.Spec.PolicyTypes)
	}
	for _, ns := range []string{"app-foreign", "rack1"} {
		_, err := cs.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
		assert.True(t, k8serrors.IsNotFound(err), "policy must NOT exist in non-(this-rack-app) namespace %s", ns)
	}
}

func TestReconcilePodImdsBlock_DisabledDeletesScoped(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newAppNamespace("app-a", "rack1"),
		newAppNamespace("app-foreign", "rack2"),
	)
	_, _ = cs.NetworkingV1().NetworkPolicies("app-a").Create(context.Background(), imdsBlockNetworkPolicy("app-a"), am.CreateOptions{})
	_, _ = cs.NetworkingV1().NetworkPolicies("app-foreign").Create(context.Background(), imdsBlockNetworkPolicy("app-foreign"), am.CreateOptions{})
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: false}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	_, err := cs.NetworkingV1().NetworkPolicies("app-a").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err), "this rack's policy should be deleted when flag off")

	_, err = cs.NetworkingV1().NetworkPolicies("app-foreign").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	assert.NoError(t, err, "another rack's policy must NOT be deleted")
}

func TestEnsureImdsBlockPolicy_Idempotent(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.NoError(t, p.ensureImdsBlockPolicy(context.Background(), "app-a"))
	require.NoError(t, p.ensureImdsBlockPolicy(context.Background(), "app-a"), "second ensure must swallow AlreadyExists")

	_, err := cs.NetworkingV1().NetworkPolicies("app-a").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	require.NoError(t, err)
}

func TestRemoveImdsBlockPolicy_AbsentIsNoop(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: false}

	require.NoError(t, p.removeImdsBlockPolicy(context.Background(), "app-a"), "remove must swallow NotFound when no policy exists")
}

func TestReconcilePodImdsBlock_ContinuesPastPerNamespaceError(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newAppNamespace("app-a", "rack1"),
		newAppNamespace("app-b", "rack1"),
	)
	cs.PrependReactor("create", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if ca, ok := action.(k8stesting.CreateAction); ok && ca.GetNamespace() == "app-a" {
			return true, nil, assert.AnError
		}
		return false, nil, nil
	})
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	_, errA := cs.NetworkingV1().NetworkPolicies("app-a").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(errA), "app-a's create was forced to fail, so its policy must be absent")

	_, err := cs.NetworkingV1().NetworkPolicies("app-b").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	assert.NoError(t, err, "a per-namespace create error in app-a must not skip app-b")
}

func TestReconcilePodImdsBlock_ListErrorReturned(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.Error(t, p.reconcilePodImdsBlock(context.Background()), "a namespace List failure must surface as an error")
}

func TestReconcilePodImdsBlockSafe_PanicSurvives(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	cs.PrependReactor("create", "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		panic("simulated client-go corruption mid-create")
	})
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.NotPanics(t, func() {
		p.reconcilePodImdsBlockSafe(context.Background())
	}, "reconcile must recover from a panic inside the client call")
}

func TestRunPodImdsBlockReconciler_StopsOnContextCancel(t *testing.T) {
	cs := fake.NewSimpleClientset()
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: false}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		p.runPodImdsBlockReconciler(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runPodImdsBlockReconciler did not return after context cancel")
	}
}
