package k8s

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	networkIsolationPolicyName     = "convox-network-isolation"
	sandboxIngressPolicyName       = "convox-sandbox-ingress"
	sandboxEgressPolicyName        = "convox-sandbox-egress"
	networkPolicyReconcileInterval = 2 * time.Minute

	balancerTargetTypeAnnotation      = "service.beta.kubernetes.io/aws-load-balancer-nlb-target-type"
	balancerHealthCheckPortAnnotation = "service.beta.kubernetes.io/aws-load-balancer-healthcheck-port"

	// The VPC CNI agent keeps 24 port entries per source address, and each peer address also carries its own catch-all.
	balancerPolicyPortLimit = 23
)

// sandboxEgressDeniedCIDRs keeps sandbox pods off the VPC, pod and service networks, link-local
// metadata endpoints, and CGNAT space some CNIs use for pods.
var sandboxEgressDeniedCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"100.64.0.0/10",
	"169.254.0.0/16",
}

func networkIsolationPolicy(namespace, systemNamespace, rack, tid string) *networkingv1.NetworkPolicy {
	peers := []networkingv1.NetworkPolicyPeer{
		{PodSelector: &am.LabelSelector{}},
		{NamespaceSelector: &am.LabelSelector{
			MatchExpressions: []am.LabelSelectorRequirement{{
				Key:      "kubernetes.io/metadata.name",
				Operator: am.LabelSelectorOpIn,
				Values:   []string{systemNamespace, "kube-system", "convox-monitoring"},
			}},
		}},
	}

	if tid != "" {
		peers = append(peers, networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &am.LabelSelector{
				MatchLabels: map[string]string{"rack": rack, "tid": tid},
			},
		})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      networkIsolationPolicyName,
			Namespace: namespace,
			Labels:    map[string]string{"system": "convox"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			// Policies are additive, so sandbox pods must be outside this same-namespace allow
			// for convox-sandbox-ingress to isolate them.
			PodSelector: am.LabelSelector{
				MatchExpressions: []am.LabelSelectorRequirement{{
					Key:      SandboxLabel,
					Operator: am.LabelSelectorOpDoesNotExist,
				}},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
		},
	}
}

// balancerNetworkPolicy admits any source on the balancer's ports, since its traffic arrives from node or load balancer addresses no peer can tell apart from pods.
func balancerNetworkPolicy(isolation *networkingv1.NetworkPolicy, s *corev1.Service, ports []networkingv1.NetworkPolicyPort) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      "convox-" + s.Name,
			Namespace: isolation.Namespace,
			Labels:    map[string]string{"system": "convox", "type": "balancer"},
			OwnerReferences: []am.OwnerReference{{
				APIVersion: networkingv1.SchemeGroupVersion.String(),
				Kind:       "NetworkPolicy",
				Name:       isolation.Name,
				UID:        isolation.UID,
			}},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: am.LabelSelector{MatchLabels: s.Spec.Selector},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     append(append([]networkingv1.NetworkPolicyIngressRule{}, isolation.Spec.Ingress...), networkingv1.NetworkPolicyIngressRule{Ports: ports}),
		},
	}
}

func sandboxPodSelector() am.LabelSelector {
	return am.LabelSelector{MatchLabels: map[string]string{SandboxLabel: "true"}}
}

// sandboxSharedEgressSelector leaves out pods with their own egress limit: allowed egress adds up across
// policies, so a restricted pod must not also match the shared one.
func sandboxSharedEgressSelector() am.LabelSelector {
	sel := sandboxPodSelector()
	sel.MatchExpressions = []am.LabelSelectorRequirement{{Key: SandboxEgressLabel, Operator: am.LabelSelectorOpDoesNotExist}}
	return sel
}

// sandboxIngressPolicy denies all ingress to sandbox pods and is also their egress baseline: with Egress among
// its types and no egress rules, a sandbox pod reaches nothing unless another policy allows it. A restricted
// pod whose own policy is missing therefore has no network instead of an open one.
func sandboxIngressPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      sandboxIngressPolicyName,
			Namespace: namespace,
			Labels:    map[string]string{"system": "convox"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: sandboxPodSelector(),
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}
}

// sandboxResolverPort is the container port behind the rack resolver service (53 -> 5454), the nameserver
// rack pods use. Policies match the pod port, not the service port.
const sandboxResolverPort = 5454

// sandboxDNSRules allows DNS to CoreDNS and to the rack's resolver.
func sandboxDNSRules(systemNamespace string) []networkingv1.NetworkPolicyEgressRule {
	udp := corev1.ProtocolUDP
	tcp := corev1.ProtocolTCP
	dns := intstr.FromInt32(53)
	resolver := intstr.FromInt32(sandboxResolverPort)

	return []networkingv1.NetworkPolicyEgressRule{
		{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &am.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}},
				PodSelector:       &am.LabelSelector{MatchLabels: map[string]string{"k8s-app": "kube-dns"}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: &udp, Port: &dns}, {Protocol: &tcp, Port: &dns}},
		},
		{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &am.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": systemNamespace}},
				PodSelector:       &am.LabelSelector{MatchLabels: map[string]string{"system": "convox", "service": "resolver"}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: &udp, Port: &resolver}},
		},
	}
}

func sandboxEgressPolicy(namespace, systemNamespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      sandboxEgressPolicyName,
			Namespace: namespace,
			Labels:    map[string]string{"system": "convox"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: sandboxSharedEgressSelector(),
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: append(sandboxDNSRules(systemNamespace), networkingv1.NetworkPolicyEgressRule{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0", Except: append([]string{}, sandboxEgressDeniedCIDRs...)},
				}},
			}),
		},
	}
}

func balancerPolicyPorts(s *corev1.Service) []networkingv1.NetworkPolicyPort {
	ports := []networkingv1.NetworkPolicyPort{}
	seen := map[string]bool{}

	add := func(protocol corev1.Protocol, port intstr.IntOrString) {
		if protocol == "" {
			protocol = corev1.ProtocolTCP
		}

		np := networkingv1.NetworkPolicyPort{Protocol: &protocol, Port: &port}

		key := networkPolicyPortKey(np)
		if seen[key] {
			return
		}
		seen[key] = true

		ports = append(ports, np)
	}

	for _, sp := range s.Spec.Ports {
		add(sp.Protocol, sp.TargetPort)
	}

	if s.Annotations[balancerTargetTypeAnnotation] != "ip" && s.Annotations[balancerTypeAnnotation] != balancerTypeNlbIp {
		return ports
	}

	hc := s.Annotations[balancerHealthCheckPortAnnotation]

	if n, err := strconv.ParseUint(hc, 10, 16); err == nil {
		if n > 0 {
			add(corev1.ProtocolTCP, intstr.Parse(hc))
		}
		return ports
	}

	trafficPort := hc == "" || hc == "traffic-port"

	for _, sp := range s.Spec.Ports {
		if (trafficPort && sp.Protocol == corev1.ProtocolUDP) || (!trafficPort && sp.Name == hc) {
			add(corev1.ProtocolTCP, sp.TargetPort)
		}
	}

	return ports
}

func networkPolicyPortKey(np networkingv1.NetworkPolicyPort) string {
	return fmt.Sprintf("%s/%s", *np.Protocol, np.Port.String())
}

func (p *Provider) reconcileNetworkPolicy(ctx context.Context, namespace string) error {
	selector := fmt.Sprintf("system=convox,rack=%s,type=app", p.Name)
	if namespace != "" {
		selector += ",kubernetes.io/metadata.name=" + namespace
	}

	nss, err := p.ListNamespacesFromInformer(selector)
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range nss.Items {
		ns := nss.Items[i].Name

		nsLabels := nss.Items[i].Labels
		if rerr := p.reconcileNetworkPolicyNamespace(ctx, ns, nsLabels["tid"], nsLabels[SandboxLabel] == "true"); rerr != nil {
			fmt.Printf("ns=network_policy at=warn kind=reconcile rack=%s namespace=%s err=%q\n", p.Name, ns, rerr)
		}
	}

	return nil
}

func (p *Provider) reconcileNetworkPolicyNamespace(ctx context.Context, namespace, tid string, sandbox bool) error {
	if !p.NetworkPolicyEnabled {
		if sandbox {
			if err := p.removeSandboxNetworkPolicies(ctx, namespace); err != nil {
				return err
			}
			if err := p.removeAllSandboxNetworkPolicies(ctx, namespace); err != nil {
				return err
			}
		}
		return p.removeNetworkIsolationPolicy(ctx, namespace)
	}

	if sandbox {
		if err := p.ensureSandboxNetworkPolicies(ctx, namespace); err != nil {
			return err
		}
		if err := p.removeOrphanSandboxNetworkPolicies(ctx, namespace, time.Now()); err != nil {
			return err
		}
	}

	ss, err := p.Cluster.CoreV1().Services(namespace).List(ctx, am.ListOptions{LabelSelector: "type=balancer"})
	if err != nil {
		return err
	}

	isolation, err := p.ensureNetworkPolicy(ctx, networkIsolationPolicy(namespace, p.Namespace, p.Name, tid))
	if err != nil {
		return err
	}

	return p.reconcileBalancerPolicies(ctx, isolation, ss.Items)
}

func (p *Provider) reconcileBalancerPolicies(ctx context.Context, isolation *networkingv1.NetworkPolicy, ss []corev1.Service) error {
	ports := map[string][]networkingv1.NetworkPolicyPort{}
	selected := map[string]map[string]bool{}

	for i := range ss {
		if len(ss[i].Spec.Selector) == 0 || len(ss[i].Spec.Ports) == 0 {
			continue
		}

		ports[ss[i].Name] = balancerPolicyPorts(&ss[i])

		selector := labels.Set(ss[i].Spec.Selector).String()
		if selected[selector] == nil {
			selected[selector] = map[string]bool{}
		}
		for _, np := range ports[ss[i].Name] {
			selected[selector][networkPolicyPortKey(np)] = true
		}
	}

	keep := map[string]bool{}

	for i := range ss {
		s := &ss[i]

		ps, ok := ports[s.Name]
		if !ok {
			continue
		}

		if n := len(selected[labels.Set(s.Spec.Selector).String()]); n > balancerPolicyPortLimit {
			fmt.Printf("ns=network_policy at=warn kind=port_limit rack=%s namespace=%s service=%s ports=%d\n", p.Name, isolation.Namespace, s.Name, n)
			ps = nil
		}

		np := balancerNetworkPolicy(isolation, s, ps)
		keep[np.Name] = true

		if _, err := p.ensureNetworkPolicy(ctx, np); err != nil {
			fmt.Printf("ns=network_policy at=warn kind=reconcile rack=%s namespace=%s policy=%s err=%q\n", p.Name, np.Namespace, np.Name, err)
		}
	}

	nps := p.Cluster.NetworkingV1().NetworkPolicies(isolation.Namespace)

	existing, err := nps.List(ctx, am.ListOptions{LabelSelector: "system=convox,type=balancer"})
	if err != nil {
		return err
	}

	for i := range existing.Items {
		name := existing.Items[i].Name

		if keep[name] {
			continue
		}

		if err := nps.Delete(ctx, name, am.DeleteOptions{}); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (p *Provider) ensureSandboxNetworkPolicies(ctx context.Context, namespace string) error {
	if _, err := p.ensureNetworkPolicy(ctx, sandboxIngressPolicy(namespace)); err != nil {
		return err
	}
	_, err := p.ensureNetworkPolicy(ctx, sandboxEgressPolicy(namespace, p.Namespace))
	return err
}

func (p *Provider) removeSandboxNetworkPolicies(ctx context.Context, namespace string) error {
	for _, name := range []string{sandboxIngressPolicyName, sandboxEgressPolicyName} {
		err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, am.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// prepareSandboxNamespace marks a namespace as hosting sandboxes and applies its policies before
// the first sandbox pod is created, so no pod waits for the next reconcile tick to be isolated.
func (p *Provider) prepareSandboxNamespace(ctx context.Context, namespace string) error {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, namespace, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	if ns.Labels[SandboxLabel] != "true" {
		patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:"true"}}}`, SandboxLabel))
		if _, err := p.Cluster.CoreV1().Namespaces().Patch(ctx, namespace, types.MergePatchType, patch, am.PatchOptions{}); err != nil {
			return errors.WithStack(err)
		}
	}

	return p.reconcileNetworkPolicyNamespace(ctx, namespace, ns.Labels["tid"], true)
}

func (p *Provider) ensureNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy) (*networkingv1.NetworkPolicy, error) {
	nps := p.Cluster.NetworkingV1().NetworkPolicies(np.Namespace)

	existing, err := nps.Get(ctx, np.Name, am.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, err
		}

		created, cerr := nps.Create(ctx, np, am.CreateOptions{})
		if k8serrors.IsAlreadyExists(cerr) {
			return nps.Get(ctx, np.Name, am.GetOptions{})
		}
		if cerr != nil {
			return nil, cerr
		}

		return created, nil
	}

	if apiequality.Semantic.DeepEqual(existing.Spec, np.Spec) && apiequality.Semantic.DeepEqual(existing.OwnerReferences, np.OwnerReferences) {
		return existing, nil
	}

	existing.Spec = np.Spec
	existing.OwnerReferences = np.OwnerReferences

	return nps.Update(ctx, existing, am.UpdateOptions{})
}

func (p *Provider) removeNetworkIsolationPolicy(ctx context.Context, namespace string) error {
	err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, networkIsolationPolicyName, am.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *Provider) kickNetworkPolicy(namespace string) {
	if !p.NetworkPolicyEnabled {
		return
	}

	select {
	case p.networkPolicyKick <- namespace:
	default:
	}
}

func (p *Provider) reconcileNetworkPolicySafe(ctx context.Context, namespace string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=network_policy at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	if err := p.reconcileNetworkPolicy(ctx, namespace); err != nil {
		fmt.Printf("ns=network_policy at=warn kind=reconcile rack=%s err=%q\n", p.Name, err)
	}
}

func (p *Provider) runNetworkPolicyReconciler(ctx context.Context) {
	p.reconcileNetworkPolicySafe(ctx, "")

	tick := time.NewTicker(networkPolicyReconcileInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reconcileNetworkPolicySafe(ctx, "")
		case ns := <-p.networkPolicyKick:
			p.reconcileNetworkPolicySafe(ctx, ns)
		}
	}
}
