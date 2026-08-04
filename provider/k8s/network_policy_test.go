package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/convox/convox/pkg/manifest"
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

func newTenantNamespace(name, rack, tid string) *corev1.Namespace {
	labels := map[string]string{"system": "convox", "rack": rack, "type": "app", "name": name}
	if tid != "" {
		labels["tid"] = tid
	}
	return &corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: name, Labels: labels}}
}

func balancerService(namespace string) *corev1.Service {
	return &corev1.Service{ObjectMeta: am.ObjectMeta{
		Name:      "balancer-public",
		Namespace: namespace,
		Labels:    map[string]string{"system": "convox", "type": "balancer", "service": "web"},
	}}
}

func appService(namespace, app string) *corev1.Service {
	return &corev1.Service{ObjectMeta: am.ObjectMeta{
		Name:      "web",
		Namespace: namespace,
		Labels:    map[string]string{"system": "convox", "rack": "rack1", "app": app, "service": "web"},
	}}
}

func getIsolationPolicy(t *testing.T, cs *fake.Clientset, namespace string) (*networkingv1.NetworkPolicy, error) {
	t.Helper()
	return cs.NetworkingV1().NetworkPolicies(namespace).Get(context.Background(), networkIsolationPolicyName, am.GetOptions{})
}

func TestNetworkIsolationPolicy_NoTid(t *testing.T) {
	np := networkIsolationPolicy("rack1-myapp", "rack1-system", "rack1", "")

	assert.Equal(t, networkIsolationPolicyName, np.Name)
	assert.Equal(t, "rack1-myapp", np.Namespace)
	assert.Equal(t, "convox", np.Labels["system"])

	require.Len(t, np.Spec.PolicyTypes, 1)
	assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])
	assert.Empty(t, np.Spec.Egress)
	assert.Empty(t, np.Spec.PodSelector.MatchLabels)
	assert.Empty(t, np.Spec.PodSelector.MatchExpressions)

	require.Len(t, np.Spec.Ingress, 1)
	assert.Empty(t, np.Spec.Ingress[0].Ports)

	peers := np.Spec.Ingress[0].From
	require.Len(t, peers, 2)

	require.NotNil(t, peers[0].PodSelector)
	assert.Nil(t, peers[0].NamespaceSelector)
	assert.Empty(t, peers[0].PodSelector.MatchLabels)

	require.NotNil(t, peers[1].NamespaceSelector)
	require.Len(t, peers[1].NamespaceSelector.MatchExpressions, 1)
	expr := peers[1].NamespaceSelector.MatchExpressions[0]
	assert.Equal(t, "kubernetes.io/metadata.name", expr.Key)
	assert.Equal(t, am.LabelSelectorOpIn, expr.Operator)
	assert.Equal(t, []string{"rack1-system", "kube-system", "convox-monitoring"}, expr.Values)
}

func TestNetworkIsolationPolicy_WithTid(t *testing.T) {
	np := networkIsolationPolicy("rack1-ab12-myapp", "rack1-system", "rack1", "ab12")

	peers := np.Spec.Ingress[0].From
	require.Len(t, peers, 3)

	require.NotNil(t, peers[2].NamespaceSelector)
	assert.Nil(t, peers[2].PodSelector)

	// The rack value must match what reconcileNetworkPolicy feeds its namespace list selector.
	assert.Equal(t, map[string]string{"rack": "rack1", "tid": "ab12"}, peers[2].NamespaceSelector.MatchLabels)
}

func TestReconcileNetworkPolicy_EnabledCreatesScoped(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-ab12-app-b", "rack1", "ab12"),
		newTenantNamespace("rack2-app-c", "rack2", ""),
		&corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1-build-app-a", Labels: map[string]string{"system": "convox", "rack": "rack1", "type": "build", "name": "app-a"}}},
		&corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1-system", Labels: map[string]string{"system": "convox", "rack": "rack1", "type": "rack"}}},
	)
	p := &Provider{Cluster: cs, Name: "rack1", RackName: "console-supplied-other-name", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	for _, ns := range []string{"rack1-app-a", "rack1-ab12-app-b"} {
		np, err := getIsolationPolicy(t, cs, ns)
		require.NoError(t, err, "expected a policy in %s", ns)
		assert.Equal(t, networkingv1.PolicyTypeIngress, np.Spec.PolicyTypes[0])
	}

	tenant, err := getIsolationPolicy(t, cs, "rack1-ab12-app-b")
	require.NoError(t, err)
	require.Len(t, tenant.Spec.Ingress[0].From, 3, "a tid-labeled namespace gets the same-tenant peer")
	assert.Equal(t, map[string]string{"rack": "rack1", "tid": "ab12"}, tenant.Spec.Ingress[0].From[2].NamespaceSelector.MatchLabels,
		"the same-tenant peer must match the rack label apps actually carry, which comes from p.Name and not p.RackName")

	plain, err := getIsolationPolicy(t, cs, "rack1-app-a")
	require.NoError(t, err)
	require.Len(t, plain.Spec.Ingress[0].From, 2, "a namespace with no tid label gets no same-tenant peer")

	for _, ns := range []string{"rack2-app-c", "rack1-build-app-a", "rack1-system"} {
		_, err := getIsolationPolicy(t, cs, ns)
		assert.True(t, k8serrors.IsNotFound(err), "%s must not be policed", ns)
	}
}

func TestReconcileNetworkPolicy_DisabledRemoves(t *testing.T) {
	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""))
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	p.NetworkPolicyEnabled = false
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()), "removal must be idempotent")

	_, err := getIsolationPolicy(t, cs, "rack1-app-a")
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestReconcileNetworkPolicy_BalancerNamespaceExcluded(t *testing.T) {
	require.True(t, manifest.ReservedLabelKeys["type"], "a user-settable 'type' label would let an app self-exclude from isolation")

	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-app-b", "rack1", ""),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	_, err := cs.CoreV1().Services("rack1-app-b").Create(context.Background(), appService("rack1-app-b", "app-b"), am.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	_, err = cs.CoreV1().Services("rack1-app-a").Create(context.Background(), balancerService("rack1-app-a"), am.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	_, err = getIsolationPolicy(t, cs, "rack1-app-a")
	assert.True(t, k8serrors.IsNotFound(err), "a namespace publishing a balancer must be left unpoliced")

	_, err = getIsolationPolicy(t, cs, "rack1-app-b")
	assert.NoError(t, err, "an ordinary app Service must not read as a balancer")
}

func TestReconcileNetworkPolicy_BalancerProbeFailureLeavesNamespaceAlone(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-app-b", "rack1", ""),
		newTenantNamespace("rack1-app-c", "rack1", ""),
		networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", ""),
	)
	cs.PrependReactor("list", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "rack1-app-b" {
			return false, nil, nil
		}
		return true, nil, assert.AnError
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	_, err := getIsolationPolicy(t, cs, "rack1-app-a")
	assert.NoError(t, err, "an unverifiable namespace must keep the policy it already has")

	_, err = getIsolationPolicy(t, cs, "rack1-app-c")
	assert.True(t, k8serrors.IsNotFound(err), "an unverifiable namespace must not be policed")

	_, err = getIsolationPolicy(t, cs, "rack1-app-b")
	assert.NoError(t, err, "a probe failure in one namespace must not skip the next")
}

func TestEnsureNetworkIsolationPolicy_RepairsDrift(t *testing.T) {
	stale := networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", "")
	stale.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}
	stale.Spec.Ingress = nil

	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""), stale)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	np, err := getIsolationPolicy(t, cs, "rack1-app-a")
	require.NoError(t, err)
	assert.Equal(t, networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", "").Spec, np.Spec)
}

func TestReconcileNetworkPolicy_NoUpdateWhenUnchanged(t *testing.T) {
	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-ab12-app-a", "rack1", "ab12"))
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	updates := 0
	cs.PrependReactor("update", "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		return false, nil, nil
	})

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	assert.Zero(t, updates, "a policy that already matches must not be rewritten on every tick")
}

func TestReconcileNetworkPolicy_ContinuesPastPerNamespaceError(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-app-b", "rack1", ""),
	)
	cs.PrependReactor("create", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if ca, ok := action.(k8stesting.CreateAction); ok && ca.GetNamespace() == "rack1-app-a" {
			return true, nil, assert.AnError
		}
		return false, nil, nil
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background()))

	_, err := getIsolationPolicy(t, cs, "rack1-app-a")
	assert.True(t, k8serrors.IsNotFound(err))

	_, err = getIsolationPolicy(t, cs, "rack1-app-b")
	assert.NoError(t, err, "a per-namespace create error must not skip the next namespace")
}

func TestReconcileNetworkPolicy_ListErrorReturned(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("list", "namespaces", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.Error(t, p.reconcileNetworkPolicy(context.Background()))
}

func TestReconcileNetworkPolicySafe_PanicSurvives(t *testing.T) {
	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""))
	cs.PrependReactor("create", "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		panic("simulated client-go corruption mid-create")
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NotPanics(t, func() {
		p.reconcileNetworkPolicySafe(context.Background())
	})
}

func TestRunNetworkPolicyReconciler_StopsOnContextCancel(t *testing.T) {
	cs := fake.NewSimpleClientset()
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: false}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		p.runNetworkPolicyReconciler(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runNetworkPolicyReconciler did not return after context cancel")
	}
}
