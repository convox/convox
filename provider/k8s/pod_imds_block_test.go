package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

func TestBuildImdsBlockNetworkPolicy(t *testing.T) {
	np := buildImdsBlockNetworkPolicy("rack1-app1", "rack1")

	assert.Equal(t, buildImdsBlockPolicyName, np.Name)
	assert.Equal(t, "rack1-app1", np.Namespace)
	assert.Equal(t, map[string]string{"system": "convox", "rack": "rack1", "type": "build-imds-block"}, np.Labels)
	assert.Equal(t, map[string]string{"service-type": "build"}, np.Spec.PodSelector.MatchLabels)
	assert.Empty(t, np.Spec.PodSelector.MatchExpressions)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, np.Spec.PolicyTypes)
	assert.Empty(t, np.Spec.Ingress)

	require.Len(t, np.Spec.Egress, 1)
	require.Len(t, np.Spec.Egress[0].To, 1)
	assert.Empty(t, np.Spec.Egress[0].Ports)
	ipb := np.Spec.Egress[0].To[0].IPBlock
	require.NotNil(t, ipb)
	assert.Equal(t, "0.0.0.0/0", ipb.CIDR)
	assert.Equal(t, []string{"169.254.169.254/32"}, ipb.Except)
}

func buildImdsProvider(t *testing.T, enabled bool) (*Provider, *fake.Clientset) {
	t.Helper()
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)
	p.BuildImdsBlockEnabled = enabled
	return p, kk
}

func buildRunOptions() structs.ProcessRunOptions {
	return structs.ProcessRunOptions{Release: options.String("rel1"), Command: options.String("build"), IsBuild: true}
}

func TestProcessRunBuildImdsBlock(t *testing.T) {
	for name, tc := range map[string]struct {
		enabled bool
		service string
		isBuild bool
	}{
		"on":           {enabled: true, service: "build", isBuild: true},
		"off":          {enabled: false, service: "build", isBuild: true},
		"on-not-build": {enabled: true, service: "web", isBuild: false},
	} {
		t.Run(name, func(t *testing.T) {
			p, kk := buildImdsProvider(t, tc.enabled)

			_, err := p.ProcessRun("app1", tc.service, structs.ProcessRunOptions{Release: options.String("rel1"), IsBuild: tc.isBuild})
			require.NoError(t, err)

			pods, err := kk.CoreV1().Pods("rack1-app1").List(context.TODO(), am.ListOptions{})
			require.NoError(t, err)
			require.Len(t, pods.Items, 1)

			sel, err := am.LabelSelectorAsSelector(&buildImdsBlockNetworkPolicy("rack1-app1", "rack1").Spec.PodSelector)
			require.NoError(t, err)
			assert.Equal(t, tc.isBuild, sel.Matches(labels.Set(pods.Items[0].Labels)))

			_, err = kk.NetworkingV1().NetworkPolicies("rack1-app1").Get(context.TODO(), buildImdsBlockPolicyName, am.GetOptions{})
			if !tc.enabled || !tc.isBuild {
				require.True(t, k8serrors.IsNotFound(err))
				return
			}
			require.NoError(t, err)

			var order []string
			for _, a := range kk.Actions() {
				if r := a.GetResource().Resource; a.GetVerb() == "create" && (r == "networkpolicies" || r == "pods") {
					order = append(order, r)
				}
			}
			assert.Equal(t, []string{"networkpolicies", "pods"}, order)
		})
	}
}

func TestProcessRunBuildImdsBlockExisting(t *testing.T) {
	p, kk := buildImdsProvider(t, true)
	_, err := kk.NetworkingV1().NetworkPolicies("rack1-app1").Create(context.TODO(), buildImdsBlockNetworkPolicy("rack1-app1", "rack1"), am.CreateOptions{})
	require.NoError(t, err)

	_, err = p.ProcessRun("app1", "build", buildRunOptions())
	require.NoError(t, err)
}

func TestProcessRunBuildImdsBlockCreateError(t *testing.T) {
	p, kk := buildImdsProvider(t, true)
	kk.PrependReactor("create", "networkpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})

	_, err := p.ProcessRun("app1", "build", buildRunOptions())
	require.ErrorIs(t, err, assert.AnError)

	pods, err := kk.CoreV1().Pods("rack1-app1").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, pods.Items)
}

func TestProcessRunBuildImdsBlockBuildNamespace(t *testing.T) {
	p, kk := buildImdsProvider(t, true)
	p.PodSecurityStandard = "baseline"
	p.PodSecurityMode = "enforce"

	_, err := p.ProcessRun("app1", "build", buildRunOptions())
	require.NoError(t, err)

	_, err = kk.NetworkingV1().NetworkPolicies("rack1-build-app1").Get(context.TODO(), buildImdsBlockPolicyName, am.GetOptions{})
	require.NoError(t, err)

	_, err = kk.NetworkingV1().NetworkPolicies("rack1-app1").Get(context.TODO(), buildImdsBlockPolicyName, am.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err))
}

func createPolicies(t *testing.T, cs *fake.Clientset, nps ...*networkingv1.NetworkPolicy) {
	t.Helper()
	for _, np := range nps {
		_, err := cs.NetworkingV1().NetworkPolicies(np.Namespace).Create(context.Background(), np, am.CreateOptions{})
		require.NoError(t, err)
	}
}

func TestReconcilePodImdsBlock_BuildPolicyRemovedWhenOff(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	other := buildImdsBlockNetworkPolicy("app-a", "rack1")
	other.Name = "convox-other"
	other.Labels["type"] = "balancer"
	createPolicies(t, cs,
		buildImdsBlockNetworkPolicy("app-a", "rack1"),
		buildImdsBlockNetworkPolicy("rack1-build-app-a", "rack1"),
		buildImdsBlockNetworkPolicy("app-foreign", "rack2"),
		imdsBlockNetworkPolicy("rack1-build-app-a"),
		other,
	)
	p := &Provider{Cluster: cs, Name: "rack1"}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	for _, ns := range []string{"app-a", "rack1-build-app-a"} {
		_, err := cs.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), buildImdsBlockPolicyName, am.GetOptions{})
		assert.True(t, k8serrors.IsNotFound(err), "this rack's build policy must be deleted in %s", ns)
	}
	for _, kept := range [][2]string{{"app-foreign", buildImdsBlockPolicyName}, {"rack1-build-app-a", imdsBlockPolicyName}, {"app-a", "convox-other"}} {
		_, err := cs.NetworkingV1().NetworkPolicies(kept[0]).Get(context.Background(), kept[1], am.GetOptions{})
		assert.NoError(t, err, "%s/%s must stay", kept[0], kept[1])
	}
}

func TestReconcilePodImdsBlock_BuildPolicyKeptWhenOn(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	createPolicies(t, cs,
		buildImdsBlockNetworkPolicy("app-a", "rack1"),
		buildImdsBlockNetworkPolicy("rack1-build-app-a", "rack1"),
	)
	p := &Provider{Cluster: cs, Name: "rack1", BuildImdsBlockEnabled: true}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	for _, ns := range []string{"app-a", "rack1-build-app-a"} {
		_, err := cs.NetworkingV1().NetworkPolicies(ns).Get(context.Background(), buildImdsBlockPolicyName, am.GetOptions{})
		assert.NoError(t, err, "build policy must stay in %s", ns)
	}
}

func TestReconcilePodImdsBlock_BuildDeleteErrorContinues(t *testing.T) {
	cs := fake.NewSimpleClientset()
	createPolicies(t, cs,
		buildImdsBlockNetworkPolicy("app-a", "rack1"),
		buildImdsBlockNetworkPolicy("app-b", "rack1"),
	)
	cs.PrependReactor("delete", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "app-a" {
			return true, nil, assert.AnError
		}
		return false, nil, nil
	})
	p := &Provider{Cluster: cs, Name: "rack1"}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	_, err := cs.NetworkingV1().NetworkPolicies("app-b").Get(context.Background(), buildImdsBlockPolicyName, am.GetOptions{})
	assert.True(t, k8serrors.IsNotFound(err), "a failed delete in app-a must not skip app-b")
}

func TestReconcilePodImdsBlock_BuildListErrorContinues(t *testing.T) {
	cs := fake.NewSimpleClientset(newAppNamespace("app-a", "rack1"))
	cs.PrependReactor("list", "networkpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, assert.AnError
	})
	p := &Provider{Cluster: cs, Name: "rack1", PodImdsBlockEnabled: true}

	require.NoError(t, p.reconcilePodImdsBlock(context.Background()))

	_, err := cs.NetworkingV1().NetworkPolicies("app-a").Get(context.Background(), imdsBlockPolicyName, am.GetOptions{})
	assert.NoError(t, err)
}
