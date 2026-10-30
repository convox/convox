package k8s

import (
	"context"
	"fmt"
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newTenantNamespace(name, rack, tid string) *corev1.Namespace {
	labels := map[string]string{"system": "convox", "rack": rack, "type": "app", "name": name, "kubernetes.io/metadata.name": name}
	if tid != "" {
		labels["tid"] = tid
	}
	return &corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: name, Labels: labels}}
}

func balancerService(namespace, name, service string, ports ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: am.ObjectMeta{
			Name:      "balancer-" + name,
			Namespace: namespace,
			Labels:    map[string]string{"system": "convox", "type": "balancer", "service": service},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"service": service, "type": "service"},
			Ports:    ports,
		},
	}
}

func servicePort(name string, protocol corev1.Protocol, port, target int32) corev1.ServicePort {
	return corev1.ServicePort{Name: name, Protocol: protocol, Port: port, TargetPort: intstr.FromInt32(target)}
}

func withHealthCheckPort(s *corev1.Service, port string) *corev1.Service {
	if s.Annotations == nil {
		s.Annotations = map[string]string{}
	}
	s.Annotations["service.beta.kubernetes.io/aws-load-balancer-healthcheck-port"] = port
	return s
}

func ipTarget(s *corev1.Service, healthCheckPort string) *corev1.Service {
	s.Annotations = map[string]string{"service.beta.kubernetes.io/aws-load-balancer-nlb-target-type": "ip"}
	if healthCheckPort != "" {
		withHealthCheckPort(s, healthCheckPort)
	}
	return s
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

func getBalancerPolicy(t *testing.T, cs *fake.Clientset, namespace, name string) (*networkingv1.NetworkPolicy, error) {
	t.Helper()
	return cs.NetworkingV1().NetworkPolicies(namespace).Get(context.Background(), "convox-balancer-"+name, am.GetOptions{})
}

func portStrings(ps []networkingv1.NetworkPolicyPort) []string {
	ss := []string{}
	for _, p := range ps {
		proto := "<nil>"
		if p.Protocol != nil {
			proto = string(*p.Protocol)
		}
		ss = append(ss, fmt.Sprintf("%s/%s", proto, p.Port.String()))
	}
	return ss
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

func TestBalancerPolicyPorts(t *testing.T) {
	tests := []struct {
		Name    string
		Service *corev1.Service
		Want    []string
	}{
		{
			Name:    "instance target ignores health check annotation",
			Service: withHealthCheckPort(balancerService("ns", "b", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)), "8080"),
			Want:    []string{"TCP/3000"},
		},
		{
			Name:    "empty protocol is TCP",
			Service: balancerService("ns", "b", "web", servicePort("80", "", 80, 3000)),
			Want:    []string{"TCP/3000"},
		},
		{
			Name:    "tcp udp pair",
			Service: balancerService("ns", "b", "web", servicePort("80-tcp", corev1.ProtocolTCP, 80, 3000), servicePort("80-udp", corev1.ProtocolUDP, 80, 3000)),
			Want:    []string{"TCP/3000", "UDP/3000"},
		},
		{
			Name:    "ip target udp with no health check port probes the traffic port over tcp",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), ""),
			Want:    []string{"UDP/5353", "TCP/5353"},
		},
		{
			Name:    "ip target udp with traffic-port",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), "traffic-port"),
			Want:    []string{"UDP/5353", "TCP/5353"},
		},
		{
			Name:    "ip target numeric health check port",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), "8080"),
			Want:    []string{"UDP/5353", "TCP/8080"},
		},
		{
			Name:    "ip target health check port already a tcp target",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000), servicePort("53", corev1.ProtocolUDP, 53, 5353)), "3000"),
			Want:    []string{"TCP/3000", "UDP/5353"},
		},
		{
			Name:    "ip target health check port zero",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), "0"),
			Want:    []string{"UDP/5353"},
		},
		{
			Name:    "ip target health check port out of range",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), "70000"),
			Want:    []string{"UDP/5353"},
		},
		{
			Name:    "ip target health check port names a service port",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53-udp", corev1.ProtocolUDP, 53, 5353), servicePort("80", corev1.ProtocolUDP, 80, 8053)), "53-udp"),
			Want:    []string{"UDP/5353", "UDP/8053", "TCP/5353"},
		},
		{
			Name: "nlb-ip balancer type is an ip target",
			Service: func() *corev1.Service {
				s := balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353))
				s.Annotations = map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb-ip"}
				return s
			}(),
			Want: []string{"UDP/5353", "TCP/5353"},
		},
		{
			Name:    "ip target numeric health check port is a port, not a port name",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("80", corev1.ProtocolUDP, 80, 3000)), "80"),
			Want:    []string{"UDP/3000", "TCP/80"},
		},
		{
			Name:    "ip target health check port names nothing",
			Service: ipTarget(balancerService("ns", "b", "web", servicePort("53", corev1.ProtocolUDP, 53, 5353)), "health"),
			Want:    []string{"UDP/5353"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Want, portStrings(balancerPolicyPorts(test.Service)))
		})
	}
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

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

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
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	p.NetworkPolicyEnabled = false
	cs.ClearActions()
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""), "removal must be idempotent")

	calls := []string{}
	for _, a := range cs.Actions() {
		if a.GetResource().Resource != "namespaces" {
			calls = append(calls, a.GetVerb()+" "+a.GetResource().Resource)
		}
	}
	assert.Equal(t, []string{"delete networkpolicies", "delete networkpolicies"}, calls, "the disabled path is one delete per namespace, the owner reference removes the balancer policies")

	_, err := getIsolationPolicy(t, cs, "rack1-app-a")
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestReconcileNetworkPolicy_BalancerNamespaceIsolated(t *testing.T) {
	require.True(t, manifest.ReservedLabelKeys["type"], "a user-settable 'type' label would let an app open its ports to any source")

	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-app-b", "rack1", ""),
		appService("rack1-app-b", "app-b"),
		balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000), servicePort("53", corev1.ProtocolUDP, 53, 5353)),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	isolation, err := getIsolationPolicy(t, cs, "rack1-app-a")
	require.NoError(t, err, "a namespace publishing a balancer keeps its isolation policy")

	np, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"system": "convox", "type": "balancer"}, np.Labels)
	assert.Equal(t, map[string]string{"service": "web", "type": "service"}, np.Spec.PodSelector.MatchLabels)
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, np.Spec.PolicyTypes)

	require.Len(t, np.Spec.Ingress, 2)
	assert.Equal(t, isolation.Spec.Ingress[0], np.Spec.Ingress[0], "the balancer policy carries the isolation peers so it is never stricter on its own")
	assert.Nil(t, np.Spec.Ingress[1].From)
	assert.Equal(t, []string{"TCP/3000", "UDP/5353"}, portStrings(np.Spec.Ingress[1].Ports))

	require.Len(t, np.OwnerReferences, 1)
	assert.Equal(t, am.OwnerReference{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy", Name: networkIsolationPolicyName}, np.OwnerReferences[0])

	_, err = getIsolationPolicy(t, cs, "rack1-app-b")
	assert.NoError(t, err, "an ordinary app Service must not read as a balancer")

	nps, err := cs.NetworkingV1().NetworkPolicies("rack1-app-b").List(context.Background(), am.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, nps.Items, 1)
}

func TestReconcileNetworkPolicy_BalancerOwnerUIDFromCreate(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)),
	)
	cs.PrependReactor("create", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ca, ok := action.(k8stesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		np, ok := ca.GetObject().(*networkingv1.NetworkPolicy)
		if !ok {
			return false, nil, nil
		}
		created := np.DeepCopy()
		created.UID = types.UID("uid-" + np.Name)
		return true, created, cs.Tracker().Create(networkingv1.SchemeGroupVersion.WithResource("networkpolicies"), created, ca.GetNamespace())
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	np, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
	require.NoError(t, err)
	require.Len(t, np.OwnerReferences, 1)
	assert.Equal(t, types.UID("uid-"+networkIsolationPolicyName), np.OwnerReferences[0].UID, "the owner UID must come from the object the apiserver created")
}

func TestReconcileNetworkPolicy_BalancerOwnerRepointed(t *testing.T) {
	for _, drifted := range []bool{false, true} {
		t.Run(fmt.Sprintf("isolation drifted %t", drifted), func(t *testing.T) {
			svc := balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000))

			isolation := networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", "")
			isolation.UID = "live"

			old := isolation.DeepCopy()
			old.UID = "old"
			stale := balancerNetworkPolicy(old, svc, balancerPolicyPorts(svc))

			if drifted {
				isolation.Spec.Ingress = nil
			}

			cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""), svc, isolation, stale)
			p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

			require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

			np, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
			require.NoError(t, err)
			require.Len(t, np.OwnerReferences, 1)
			assert.Equal(t, types.UID("live"), np.OwnerReferences[0].UID, "a balancer policy owned by a replaced isolation policy must be repointed before garbage collection removes it")
		})
	}
}

func TestReconcileNetworkPolicy_BalancerOwnerUIDFromRace(t *testing.T) {
	isolation := networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", "")
	isolation.UID = "seeded"

	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)),
		isolation,
	)
	missed := false
	cs.PrependReactor("get", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if ga, ok := action.(k8stesting.GetAction); ok && ga.GetName() == networkIsolationPolicyName && !missed {
			missed = true
			return true, nil, k8serrors.NewNotFound(networkingv1.Resource("networkpolicies"), ga.GetName())
		}
		return false, nil, nil
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	np, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
	require.NoError(t, err)
	require.Len(t, np.OwnerReferences, 1)
	assert.Equal(t, types.UID("seeded"), np.OwnerReferences[0].UID, "a create that loses the race must use the object another replica created")
}

func TestReconcileNetworkPolicy_BalancerPortLimit(t *testing.T) {
	ports := func(base, n int32) []corev1.ServicePort {
		ps := []corev1.ServicePort{}
		for i := int32(0); i < n; i++ {
			ps = append(ps, servicePort(fmt.Sprintf("p%d", i), corev1.ProtocolTCP, 20000+i, base+i))
		}
		return ps
	}

	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "under", "web", ports(1000, balancerPolicyPortLimit)...),
		balancerService("rack1-app-a", "over", "game", ports(2000, balancerPolicyPortLimit+1)...),
		balancerService("rack1-app-a", "split-a", "voice", ports(3000, 12)...),
		balancerService("rack1-app-a", "split-b", "voice", ports(4000, 12)...),
		balancerService("rack1-app-a", "public", "api", ports(5000, 12)...),
		balancerService("rack1-app-a", "internal", "api", ports(5000, 12)...),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	under, err := getBalancerPolicy(t, cs, "rack1-app-a", "under")
	require.NoError(t, err)
	assert.Len(t, under.Spec.Ingress[1].Ports, balancerPolicyPortLimit)

	for _, name := range []string{"public", "internal"} {
		np, err := getBalancerPolicy(t, cs, "rack1-app-a", name)
		require.NoError(t, err)
		assert.Len(t, np.Spec.Ingress[1].Ports, 12, "%s shares its ports with a sibling balancer, so the pods hold 12 entries, not 24", name)
	}

	for _, name := range []string{"over", "split-a", "split-b"} {
		np, err := getBalancerPolicy(t, cs, "rack1-app-a", name)
		require.NoError(t, err)
		assert.Equal(t, networkingv1.NetworkPolicyIngressRule{}, np.Spec.Ingress[1], "%s exceeds the agent's per-address port entries and must fall back to an open rule", name)
	}
}

func TestReconcileNetworkPolicy_BalancerWithoutSelectorSkipped(t *testing.T) {
	s := balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000))
	s.Spec.Selector = nil

	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""), s)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	_, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
	assert.True(t, k8serrors.IsNotFound(err), "an empty selector would admit any source to every pod in the namespace")

	_, err = getIsolationPolicy(t, cs, "rack1-app-a")
	assert.NoError(t, err)
}

func TestReconcileNetworkPolicy_BalancerRemoved(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	_, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
	require.NoError(t, err)

	require.NoError(t, cs.CoreV1().Services("rack1-app-a").Delete(context.Background(), "balancer-public", am.DeleteOptions{}))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	_, err = getBalancerPolicy(t, cs, "rack1-app-a", "public")
	assert.True(t, k8serrors.IsNotFound(err), "removing the last balancer must remove its policy")

	_, err = getIsolationPolicy(t, cs, "rack1-app-a")
	assert.NoError(t, err)
}

func TestReconcileNetworkPolicy_BalancerErrorContinues(t *testing.T) {
	stale := balancerNetworkPolicy(networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", ""), balancerService("rack1-app-a", "first", "web"), nil)

	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		balancerService("rack1-app-a", "first", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)),
		balancerService("rack1-app-a", "second", "api", servicePort("81", corev1.ProtocolTCP, 81, 4000)),
		stale,
	)
	cs.PrependReactor("update", "networkpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if ua, ok := action.(k8stesting.UpdateAction); ok {
			if np, ok := ua.GetObject().(*networkingv1.NetworkPolicy); ok && np.Name == "convox-balancer-first" {
				return true, nil, assert.AnError
			}
		}
		return false, nil, nil
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	_, err := getBalancerPolicy(t, cs, "rack1-app-a", "second")
	assert.NoError(t, err, "one balancer's error must not skip the next")

	_, err = getBalancerPolicy(t, cs, "rack1-app-a", "first")
	assert.NoError(t, err, "a balancer whose update failed must not be deleted as stale")
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

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

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

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	np, err := getIsolationPolicy(t, cs, "rack1-app-a")
	require.NoError(t, err)
	assert.Equal(t, networkIsolationPolicy("rack1-app-a", "rack1-system", "rack1", "").Spec, np.Spec)
}

func TestReconcileNetworkPolicy_NoUpdateWhenUnchanged(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-ab12-app-a", "rack1", "ab12"),
		newTenantNamespace("rack1-ab12-app-b", "rack1", "ab12"),
		balancerService("rack1-ab12-app-b", "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000), servicePort("53", corev1.ProtocolUDP, 53, 5353)),
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	writes := 0
	for _, verb := range []string{"create", "update", "delete"} {
		cs.PrependReactor(verb, "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			writes++
			return false, nil, nil
		})
	}

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))
	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

	assert.Zero(t, writes, "a policy that already matches must not be rewritten on every tick")
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

	require.NoError(t, p.reconcileNetworkPolicy(context.Background(), ""))

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

	require.Error(t, p.reconcileNetworkPolicy(context.Background(), ""))
}

func TestReconcileNetworkPolicySafe_PanicSurvives(t *testing.T) {
	cs := fake.NewSimpleClientset(newTenantNamespace("rack1-app-a", "rack1", ""))
	cs.PrependReactor("create", "networkpolicies", func(_ k8stesting.Action) (bool, runtime.Object, error) {
		panic("simulated client-go corruption mid-create")
	})
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true}

	require.NotPanics(t, func() {
		p.reconcileNetworkPolicySafe(context.Background(), "")
	})
}

func TestKickNetworkPolicy(t *testing.T) {
	p := &Provider{NetworkPolicyEnabled: true}
	require.NotPanics(t, func() { p.kickNetworkPolicy("rack1-app-a") }, "a provider built without the channel must not block")

	p = &Provider{NetworkPolicyEnabled: false, networkPolicyKick: make(chan string, 1)}
	p.kickNetworkPolicy("rack1-app-a")
	assert.Empty(t, p.networkPolicyKick, "no kick when the parameter is off")

	p = &Provider{NetworkPolicyEnabled: true, networkPolicyKick: make(chan string, 1)}
	p.kickNetworkPolicy("rack1-app-a")
	p.kickNetworkPolicy("rack1-app-b")
	require.Len(t, p.networkPolicyKick, 1, "a full buffer drops the kick instead of blocking the atom controller")
	assert.Equal(t, "rack1-app-a", <-p.networkPolicyKick)
}

func TestRunNetworkPolicyReconciler_KickReconcilesOneNamespace(t *testing.T) {
	cs := fake.NewSimpleClientset(
		newTenantNamespace("rack1-app-a", "rack1", ""),
		newTenantNamespace("rack1-app-b", "rack1", ""),
		&corev1.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1-build-app-a", Labels: map[string]string{"system": "convox", "rack": "rack1", "type": "build", "kubernetes.io/metadata.name": "rack1-build-app-a"}}},
	)
	p := &Provider{Cluster: cs, Name: "rack1", Namespace: "rack1-system", NetworkPolicyEnabled: true, networkPolicyKick: make(chan string, 4)}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go p.runNetworkPolicyReconciler(ctx)

	require.Eventually(t, func() bool {
		_, aerr := getIsolationPolicy(t, cs, "rack1-app-a")
		_, berr := getIsolationPolicy(t, cs, "rack1-app-b")
		return aerr == nil && berr == nil
	}, 5*time.Second, 10*time.Millisecond, "the initial pass must have listed both namespaces' Services")

	for _, ns := range []string{"rack1-app-a", "rack1-app-b"} {
		_, err := cs.CoreV1().Services(ns).Create(context.Background(), balancerService(ns, "public", "web", servicePort("80", corev1.ProtocolTCP, 80, 3000)), am.CreateOptions{})
		require.NoError(t, err)
	}

	p.kickNetworkPolicy("rack1-build-app-a")
	p.kickNetworkPolicy("rack1-app-a")

	require.Eventually(t, func() bool {
		_, err := getBalancerPolicy(t, cs, "rack1-app-a", "public")
		return err == nil
	}, 5*time.Second, 10*time.Millisecond, "a kicked namespace must be reconciled without waiting for the tick")

	_, err := getBalancerPolicy(t, cs, "rack1-app-b", "public")
	assert.True(t, k8serrors.IsNotFound(err), "a kick reconciles only the namespace it names")

	_, err = getIsolationPolicy(t, cs, "rack1-build-app-a")
	assert.True(t, k8serrors.IsNotFound(err), "a kick for a namespace outside the app selector does nothing")
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
