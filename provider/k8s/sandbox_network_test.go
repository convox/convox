package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestParseSandboxNetworkEgress(t *testing.T) {
	e, err := parseSandboxNetworkEgress(" none ")
	require.NoError(t, err)
	assert.True(t, e.BlockAll)

	e, err = parseSandboxNetworkEgress("8.8.8.0/24, 1.1.1.1/32")
	require.NoError(t, err)
	require.Len(t, e.Allow, 2)
	assert.Equal(t, "8.8.8.0/24", e.Allow[0].CIDR)
	assert.Empty(t, e.Allow[0].Except)

	e, err = parseSandboxNetworkEgress("0.0.0.0/0")
	require.NoError(t, err)
	assert.Equal(t, sandboxEgressDeniedCIDRs, e.Allow[0].Except, "a wide block still cannot reach private ranges")

	e, err = parseSandboxNetworkEgress("96.0.0.0/3")
	require.NoError(t, err)
	assert.Equal(t, []string{"100.64.0.0/10"}, e.Allow[0].Except)

	bad := map[string]string{
		"":                   "or a list of CIDRs",
		",":                  "or a list of CIDRs",
		"garbage":            "IPv4 CIDR is required",
		"8.8.8.8":            "IPv4 CIDR is required",
		"2001:db8::/32":      "IPv4 CIDR is required",
		"8.8.8.1/24":         "use 8.8.8.0/24",
		"10.1.0.0/16":        "private range",
		"169.254.169.254/32": "private range",
		"1.0.0.0/8,2.0.0.0/8,3.0.0.0/8,4.0.0.0/8,5.0.0.0/8,6.0.0.0/8,7.0.0.0/8,8.0.0.0/8,9.0.0.0/8,11.0.0.0/8,12.0.0.0/8": "at most 10",
	}
	for raw, msg := range bad {
		_, err := parseSandboxNetworkEgress(raw)
		require.Error(t, err, raw)
		assert.Contains(t, err.Error(), msg, raw)
	}
}

func TestSandboxNetworkPolicyShapes(t *testing.T) {
	block := sandboxNetworkPolicy("ns", "rack1-system", "sbx-1", &SandboxNetworkEgress{BlockAll: true})
	assert.Equal(t, "convox-sandbox-net-sbx-1", block.Name)
	assert.Equal(t, map[string]string{SandboxLabel: "true", SandboxIDLabel: "sbx-1"}, block.Spec.PodSelector.MatchLabels)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, block.Spec.PolicyTypes)
	assert.NotNil(t, block.Spec.Egress)
	assert.Empty(t, block.Spec.Egress, "block-all has no egress rules, not even DNS")

	e, err := parseSandboxNetworkEgress("8.8.8.0/24")
	require.NoError(t, err)
	allow := sandboxNetworkPolicy("ns", "rack1-system", "sbx-1", e)
	require.Len(t, allow.Spec.Egress, 3)
	assert.Equal(t, map[string]string{"k8s-app": "kube-dns"}, allow.Spec.Egress[0].To[0].PodSelector.MatchLabels)
	assert.Equal(t, map[string]string{"system": "convox", "service": "resolver"}, allow.Spec.Egress[1].To[0].PodSelector.MatchLabels)
	assert.Equal(t, int32(5454), allow.Spec.Egress[1].Ports[0].Port.IntVal)
	assert.Equal(t, "8.8.8.0/24", allow.Spec.Egress[2].To[0].IPBlock.CIDR)
}

func TestSandboxSharedEgressExcludesRestrictedPods(t *testing.T) {
	np := sandboxEgressPolicy("ns", "rack1-system")
	require.Len(t, np.Spec.PodSelector.MatchExpressions, 1)
	assert.Equal(t, SandboxEgressLabel, np.Spec.PodSelector.MatchExpressions[0].Key)
	assert.Equal(t, am.LabelSelectorOpDoesNotExist, np.Spec.PodSelector.MatchExpressions[0].Operator)
	assert.Empty(t, sandboxIngressPolicy("ns").Spec.PodSelector.MatchExpressions, "ingress still denies every sandbox pod")
}

func sandboxEgressRunOptions(egress string) structs.ProcessRunOptions {
	o := sandboxRunOptions()
	o.NetworkEgress = options.String(egress)
	return o
}

func TestProcessRun_SandboxNetworkEgressPolicyBeforePod(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	p.NetworkPolicyEnabled = true
	ns := "rack1-" + sandboxHostApp

	var order []string
	kk.PrependReactor("create", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if np, ok := a.(k8stesting.CreateAction).GetObject().(*networkingv1.NetworkPolicy); ok {
			order = append(order, np.Name)
		}
		if _, ok := a.(k8stesting.CreateAction).GetObject().(*ac.Pod); ok {
			order = append(order, "pod")
		}
		return false, nil, nil
	})

	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxEgressRunOptions("none"))
	assert.Equal(t, "restricted", pod.Labels[SandboxEgressLabel])

	np, err := getPolicy(t, kk, ns, "convox-sandbox-net-sbx-1")
	require.NoError(t, err)
	assert.Empty(t, np.Spec.Egress)
	assert.Equal(t, "egress", np.Labels[sandboxNetworkPolicyManagedKey])

	require.Contains(t, order, "pod")
	assert.Equal(t, "convox-sandbox-net-sbx-1", order[len(order)-2], "the policy is created right before the pod")
	assert.Equal(t, "pod", order[len(order)-1])

	require.NoError(t, kk.CoreV1().Pods(ns).Delete(context.TODO(), pod.Name, am.DeleteOptions{}))
	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxEgressRunOptions("8.8.8.0/24"))
	np, err = getPolicy(t, kk, ns, "convox-sandbox-net-sbx-1")
	require.NoError(t, err)
	assert.Len(t, np.Spec.Egress, 3, "a later run updates the sandbox's policy")
}

func TestProcessRun_SandboxNetworkEgressRejected(t *testing.T) {
	p, _ := sandboxHostProvider(t)
	_, err := p.ProcessRun(sandboxHostApp, "sbx", sandboxEgressRunOptions("none"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network_policy_enable", "limits are never silently ignored")

	p.NetworkPolicyEnabled = true
	_, err = p.ProcessRun(sandboxHostApp, "sbx", sandboxEgressRunOptions("10.0.0.0/8"))
	require.Error(t, err)

	o := sandboxEgressRunOptions("none")
	o.Labels = options.String(SandboxLabel + "=true")
	_, err = p.ProcessRun(sandboxHostApp, "sbx", o)
	require.Error(t, err)
	assert.Contains(t, err.Error(), SandboxIDLabel)

	o = sandboxRunOptions()
	o.Labels = options.String(SandboxLabel + "=true," + SandboxIDLabel + "=sbx-1," + SandboxEgressLabel + "=restricted")
	_, err = p.ProcessRun(sandboxHostApp, "sbx", o)
	require.Error(t, err, "the egress label cannot be set without a policy")
	assert.Contains(t, err.Error(), "reserved")

	q, kk := runFlagsProvider(t)
	q.NetworkPolicyEnabled = true
	_, err = q.ProcessRun("app1", "web", structs.ProcessRunOptions{Release: options.String("rel1"), NetworkEgress: options.String("none")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only supported for sandbox")
	nps, err := kk.NetworkingV1().NetworkPolicies(q.AppNamespace("app1")).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, nps.Items)
}

func TestProcessRun_SandboxWithoutEgressHasNoOwnPolicy(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	p.NetworkPolicyEnabled = true
	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions())
	assert.NotContains(t, pod.Labels, SandboxEgressLabel)
	_, err := getPolicy(t, kk, "rack1-"+sandboxHostApp, "convox-sandbox-net-sbx-1")
	assert.True(t, k8serrors.IsNotFound(err))
}

func sandboxNetPolicyAt(ns, id string, created time.Time) *networkingv1.NetworkPolicy {
	np := sandboxNetworkPolicy(ns, "rack1-system", id, &SandboxNetworkEgress{BlockAll: true})
	np.CreationTimestamp = am.NewTime(created)
	return np
}

func TestRemoveOrphanSandboxNetworkPolicies(t *testing.T) {
	ns := "rack1-sandboxes"
	now := time.Now()
	live := &ac.Pod{ObjectMeta: am.ObjectMeta{Name: "p1", Namespace: ns, Labels: map[string]string{SandboxLabel: "true", SandboxIDLabel: "live"}}}
	kk := fake.NewSimpleClientset(
		live,
		sandboxNetPolicyAt(ns, "live", now.Add(-time.Hour)),
		sandboxNetPolicyAt(ns, "gone", now.Add(-time.Hour)),
		sandboxNetPolicyAt(ns, "young", now.Add(-time.Minute)),
		sandboxEgressPolicy(ns, "rack1-system"),
	)
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.removeOrphanSandboxNetworkPolicies(context.Background(), ns, now))

	_, err := getPolicy(t, kk, ns, "convox-sandbox-net-live")
	assert.NoError(t, err)
	_, err = getPolicy(t, kk, ns, "convox-sandbox-net-young")
	assert.NoError(t, err, "a policy may exist briefly before its pod")
	_, err = getPolicy(t, kk, ns, "convox-sandbox-net-gone")
	assert.True(t, k8serrors.IsNotFound(err))
	_, err = getPolicy(t, kk, ns, sandboxEgressPolicyName)
	assert.NoError(t, err, "shared policies are not touched")
}

func TestReconcileNetworkPolicy_DisabledRemovesSandboxEgressPolicies(t *testing.T) {
	ns := "rack1-sandboxes"
	kk := fake.NewSimpleClientset(newSandboxNamespace(ns, "rack1"), sandboxNetPolicyAt(ns, "a", time.Now()))
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system"}
	require.NoError(t, p.reconcileNetworkPolicyNamespace(context.Background(), ns, "", true))
	nps, err := kk.NetworkingV1().NetworkPolicies(ns).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, nps.Items)
}

func TestOrphanSweepKeepsRecentlyEnsuredPolicy(t *testing.T) {
	ns := "rack1-sandboxes"
	now := time.Now()
	old := sandboxNetPolicyAt(ns, "restarting", now.Add(-24*time.Hour))
	old.Annotations = map[string]string{sandboxNetworkEnsuredAnnotation: now.Add(-time.Minute).UTC().Format(time.RFC3339)}
	kk := fake.NewSimpleClientset(old)
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.removeOrphanSandboxNetworkPolicies(context.Background(), ns, now))
	_, err := getPolicy(t, kk, ns, "convox-sandbox-net-restarting")
	assert.NoError(t, err, "a policy ensured for a starting pod survives even though it was created long ago")

	require.NoError(t, p.ensureSandboxNetworkPolicy(context.Background(), sandboxNetworkPolicy(ns, "rack1-system", "restarting", &SandboxNetworkEgress{BlockAll: true}), now.Add(time.Hour)))
	np, err := getPolicy(t, kk, ns, "convox-sandbox-net-restarting")
	require.NoError(t, err)
	assert.Equal(t, now.Add(time.Hour).UTC().Format(time.RFC3339), np.Annotations[sandboxNetworkEnsuredAnnotation])
}
