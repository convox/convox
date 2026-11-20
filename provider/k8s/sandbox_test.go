package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const sandboxHostApp = "sandboxes"

func sandboxHostProvider(t *testing.T) (*Provider, *fake.Clientset) {
	t.Helper()
	p, kk, _ := minimalProvider(t)
	p.Atom.(*atom.MockInterface).On("Status", "rack1-"+sandboxHostApp, "app").Return("Running", "", nil)
	createAppNamespace(t, kk, "rack1", sandboxHostApp)
	return p, kk
}

func sandboxRunOptions() structs.ProcessRunOptions {
	return structs.ProcessRunOptions{
		Image:   options.String("python:3.12-slim-bookworm"),
		Command: options.String(`sh -c "sleep infinity"`),
		Labels:  options.String(SandboxLabel + "=true," + SandboxIDLabel + "=sbx-1"),
	}
}

func getPolicy(t *testing.T, kk *fake.Clientset, namespace, name string) (*networkingv1.NetworkPolicy, error) {
	t.Helper()
	return kk.NetworkingV1().NetworkPolicies(namespace).Get(context.TODO(), name, am.GetOptions{})
}

func TestProcessRun_EmptyHostAppUsesImageOverride(t *testing.T) {
	p, kk := sandboxHostProvider(t)

	pod := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions())

	require.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	assert.Equal(t, "python:3.12-slim-bookworm", c.Image)
	assert.Equal(t, sandboxHostApp, c.Name, "exec targets the container named after the app")
	assert.Empty(t, pod.Spec.ServiceAccountName, "a release-less pod must not get a per-service ServiceAccount")
	assert.Equal(t, "true", pod.Labels[SandboxLabel])
	assert.Equal(t, "sbx-1", pod.Labels[SandboxIDLabel])
	assert.Equal(t, ac.RestartPolicyNever, pod.Spec.RestartPolicy)
}

func TestProcessRun_EphemeralStorage(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	opts := sandboxRunOptions()
	opts.EphemeralStorage = options.Int(3072)

	c := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts).Spec.Containers[0]

	want := resource.MustParse("3072Mi")
	assert.True(t, want.Equal(c.Resources.Requests[ac.ResourceEphemeralStorage]))
	assert.True(t, want.Equal(c.Resources.Limits[ac.ResourceEphemeralStorage]))
}

func TestProcessRun_EphemeralStorageUnsetLeavesResourcesAlone(t *testing.T) {
	p, kk := runFlagsProvider(t)
	c := runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{Release: options.String("rel1")}).Spec.Containers[0]

	_, req := c.Resources.Requests[ac.ResourceEphemeralStorage]
	_, lim := c.Resources.Limits[ac.ResourceEphemeralStorage]
	assert.False(t, req)
	assert.False(t, lim)
}

func TestProcessRun_EphemeralStorageNonPositiveRejected(t *testing.T) {
	for _, v := range []int{0, -1} {
		p, _ := sandboxHostProvider(t)
		opts := sandboxRunOptions()
		opts.EphemeralStorage = options.Int(v)

		_, err := p.ProcessRun(sandboxHostApp, "sbx", opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ephemeral storage")
	}
}

func TestProcessRun_RuntimeClassAllowed(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	p.SandboxRuntimeClasses = []string{"gvisor"}
	opts := sandboxRunOptions()
	opts.RuntimeClass = options.String("gvisor")

	spec := runAndGetPod(t, p, kk, sandboxHostApp, "sbx", opts).Spec

	require.NotNil(t, spec.RuntimeClassName)
	assert.Equal(t, "gvisor", *spec.RuntimeClassName)
}

func TestProcessRun_RuntimeClassRejected(t *testing.T) {
	cases := map[string][]string{
		"empty allowlist": nil,
		"not listed":      {"kata"},
	}
	for name, allow := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := sandboxHostProvider(t)
			p.SandboxRuntimeClasses = allow
			opts := sandboxRunOptions()
			opts.RuntimeClass = options.String("gvisor")

			_, err := p.ProcessRun(sandboxHostApp, "sbx", opts)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `runtime class "gvisor" is not enabled`)
		})
	}
}

func TestProcessRun_RuntimeClassUnsetByDefault(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	assert.Nil(t, runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions()).Spec.RuntimeClassName)
}

func TestProcessRun_SandboxPreparesNamespacePolicies(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	p.NetworkPolicyEnabled = true
	ns := "rack1-" + sandboxHostApp

	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions())

	n, err := kk.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "true", n.Labels[SandboxLabel])

	for _, name := range []string{sandboxIngressPolicyName, sandboxEgressPolicyName, networkIsolationPolicyName} {
		_, err := getPolicy(t, kk, ns, name)
		assert.NoError(t, err, "%s must exist before the sandbox pod runs", name)
	}
}

func TestProcessRun_SandboxNetworkPolicyDisabled(t *testing.T) {
	p, kk := sandboxHostProvider(t)
	ns := "rack1-" + sandboxHostApp

	runAndGetPod(t, p, kk, sandboxHostApp, "sbx", sandboxRunOptions())

	n, err := kk.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "true", n.Labels[SandboxLabel], "labelled so enabling network policies later covers it")

	nps, err := kk.NetworkingV1().NetworkPolicies(ns).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, nps.Items)
}

func TestProcessRun_NonSandboxRunUntouched(t *testing.T) {
	p, kk := runFlagsProvider(t)
	p.NetworkPolicyEnabled = true

	writes := 0
	kk.PrependReactor("*", "networkpolicies", func(a k8stesting.Action) (bool, runtime.Object, error) {
		writes++
		return false, nil, nil
	})
	kk.PrependReactor("patch", "namespaces", func(a k8stesting.Action) (bool, runtime.Object, error) {
		writes++
		return false, nil, nil
	})

	runAndGetPod(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release: options.String("rel1"),
		Labels:  options.String("team=ml"),
	})

	assert.Zero(t, writes, "ordinary runs must not touch namespaces or network policies")
}

func TestSandboxIngressPolicy(t *testing.T) {
	np := sandboxIngressPolicy("rack1-sandboxes")

	assert.Equal(t, sandboxIngressPolicyName, np.Name)
	assert.Equal(t, map[string]string{SandboxLabel: "true"}, np.Spec.PodSelector.MatchLabels)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress}, np.Spec.PolicyTypes)
	assert.Empty(t, np.Spec.Egress, "no egress rules: the fail-closed baseline every sandbox pod starts from")
	assert.Empty(t, np.Spec.Ingress, "no ingress rules means deny all")
}

func TestSandboxEgressPolicy(t *testing.T) {
	np := sandboxEgressPolicy("rack1-sandboxes")

	assert.Equal(t, sandboxEgressPolicyName, np.Name)
	assert.Equal(t, map[string]string{SandboxLabel: "true"}, np.Spec.PodSelector.MatchLabels)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, np.Spec.PolicyTypes)
	require.Len(t, np.Spec.Egress, 2)

	dns := np.Spec.Egress[0]
	require.Len(t, dns.To, 1)
	assert.Equal(t, map[string]string{"k8s-app": "kube-dns"}, dns.To[0].PodSelector.MatchLabels)
	require.Len(t, dns.Ports, 2)
	assert.Equal(t, int32(53), dns.Ports[0].Port.IntVal)

	world := np.Spec.Egress[1].To[0].IPBlock
	require.NotNil(t, world)
	assert.Equal(t, "0.0.0.0/0", world.CIDR)
	assert.Contains(t, world.Except, "169.254.0.0/16")
	assert.Contains(t, world.Except, "10.0.0.0/8")
}

func newSandboxNamespace(name, rack string) *ac.Namespace {
	ns := newTenantNamespace(name, rack, "")
	ns.Labels[SandboxLabel] = "true"
	return ns
}

func TestReconcileNetworkPolicy_SandboxNamespaceGetsSandboxPolicies(t *testing.T) {
	kk := fake.NewSimpleClientset(newSandboxNamespace("rack1-sandboxes", "rack1"), newTenantNamespace("rack1-app-a", "rack1", ""))
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	for _, name := range []string{sandboxIngressPolicyName, sandboxEgressPolicyName, networkIsolationPolicyName} {
		_, err := getPolicy(t, kk, "rack1-sandboxes", name)
		assert.NoError(t, err, name)
	}
	for _, name := range []string{sandboxIngressPolicyName, sandboxEgressPolicyName} {
		_, err := getPolicy(t, kk, "rack1-app-a", name)
		assert.True(t, k8serrors.IsNotFound(err), "%s must not appear in a namespace without sandboxes", name)
	}
}

func TestReconcileNetworkPolicy_DisabledRemovesSandboxPolicies(t *testing.T) {
	kk := fake.NewSimpleClientset(
		newSandboxNamespace("rack1-sandboxes", "rack1"),
		sandboxIngressPolicy("rack1-sandboxes"),
		sandboxEgressPolicy("rack1-sandboxes"),
	)
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system"}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	nps, err := kk.NetworkingV1().NetworkPolicies("rack1-sandboxes").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, nps.Items)
}

func TestReconcileNetworkPolicy_SandboxPoliciesRepairDrift(t *testing.T) {
	stale := sandboxEgressPolicy("rack1-sandboxes")
	stale.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{}}
	kk := fake.NewSimpleClientset(newSandboxNamespace("rack1-sandboxes", "rack1"), stale)
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	np, err := getPolicy(t, kk, "rack1-sandboxes", sandboxEgressPolicyName)
	require.NoError(t, err)
	assert.Equal(t, sandboxEgressPolicy("rack1-sandboxes").Spec, np.Spec)
}

func TestReconcileNetworkPolicy_SandboxNoUpdateWhenUnchanged(t *testing.T) {
	kk := fake.NewSimpleClientset(newSandboxNamespace("rack1-sandboxes", "rack1"))
	p := &Provider{Cluster: kk, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	updates := 0
	kk.PrependReactor("update", "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		return false, nil, nil
	})

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))
	assert.Zero(t, updates)
}

func TestSplitCSV(t *testing.T) {
	assert.Nil(t, splitCSV(""))
	assert.Equal(t, []string{"gvisor"}, splitCSV("gvisor"))
	assert.Equal(t, []string{"gvisor", "kata"}, splitCSV(" gvisor , ,kata,"))
}
