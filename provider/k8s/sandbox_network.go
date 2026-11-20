package k8s

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	SandboxEgressLabel              = "convox.com/sandbox-egress"
	sandboxNetworkPolicyPrefix      = "convox-sandbox-net-"
	sandboxNetworkEgressBlockAll    = "none"
	sandboxNetworkMaxCIDRs          = 10
	sandboxNetworkPolicyOrphanAge   = 5 * time.Minute
	sandboxNetworkPolicyManagedKey  = "convox.com/sandbox-network"
	sandboxNetworkEnsuredAnnotation = "convox.com/sandbox-network-ensured-at"
)

// ensureSandboxNetworkPolicy writes the sandbox's egress policy and stamps when it was last needed, so the orphan
// sweep never removes a policy a pod is about to use.
func (p *Provider) ensureSandboxNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy, now time.Time) error {
	np.Annotations = map[string]string{sandboxNetworkEnsuredAnnotation: now.UTC().Format(time.RFC3339)}
	nps := p.Cluster.NetworkingV1().NetworkPolicies(np.Namespace)
	existing, err := nps.Get(ctx, np.Name, am.GetOptions{})
	if k8serrors.IsNotFound(err) {
		if _, cerr := nps.Create(ctx, np, am.CreateOptions{}); cerr != nil && !k8serrors.IsAlreadyExists(cerr) {
			return cerr
		}
		return nil
	}
	if err != nil {
		return err
	}
	existing.Spec = np.Spec
	existing.Labels = np.Labels
	if existing.Annotations == nil {
		existing.Annotations = map[string]string{}
	}
	existing.Annotations[sandboxNetworkEnsuredAnnotation] = np.Annotations[sandboxNetworkEnsuredAnnotation]
	_, err = nps.Update(ctx, existing, am.UpdateOptions{})
	return err
}

// lastNeeded is when a sandbox policy was last ensured, or created when it predates the stamp.
func lastNeeded(np *networkingv1.NetworkPolicy) time.Time {
	if t, err := time.Parse(time.RFC3339, np.Annotations[sandboxNetworkEnsuredAnnotation]); err == nil {
		return t
	}
	return np.CreationTimestamp.Time
}

// SandboxNetworkEgress is a per-sandbox egress limit: no egress at all, or only the listed IPv4 blocks.
type SandboxNetworkEgress struct {
	BlockAll bool
	Allow    []networkingv1.IPBlock
}

func parseSandboxNetworkEgress(raw string) (*SandboxNetworkEgress, error) {
	raw = strings.TrimSpace(raw)
	if raw == sandboxNetworkEgressBlockAll {
		return &SandboxNetworkEgress{BlockAll: true}, nil
	}

	cidrs := splitCSV(raw)
	if len(cidrs) == 0 {
		return nil, structs.ErrBadRequest("network egress must be %q or a list of CIDRs", sandboxNetworkEgressBlockAll)
	}
	if len(cidrs) > sandboxNetworkMaxCIDRs {
		return nil, structs.ErrBadRequest("at most %d network egress CIDRs are supported", sandboxNetworkMaxCIDRs)
	}

	denied := make([]*net.IPNet, 0, len(sandboxEgressDeniedCIDRs))
	for _, d := range sandboxEgressDeniedCIDRs {
		_, n, _ := net.ParseCIDR(d)
		denied = append(denied, n)
	}

	e := &SandboxNetworkEgress{}
	for _, c := range cidrs {
		ip, n, err := net.ParseCIDR(c)
		if err != nil || ip.To4() == nil {
			return nil, structs.ErrBadRequest("invalid network egress CIDR %q: an IPv4 CIDR is required", c)
		}
		if n.String() != c {
			return nil, structs.ErrBadRequest("invalid network egress CIDR %q: use %s", c, n.String())
		}
		block := networkingv1.IPBlock{CIDR: c}
		for i, d := range denied {
			switch {
			case cidrContains(d, n):
				return nil, structs.ErrBadRequest("network egress CIDR %q is inside a private range sandboxes cannot reach", c)
			case cidrContains(n, d):
				block.Except = append(block.Except, sandboxEgressDeniedCIDRs[i])
			}
		}
		e.Allow = append(e.Allow, block)
	}
	return e, nil
}

func cidrContains(outer, inner *net.IPNet) bool {
	op, _ := outer.Mask.Size()
	ip, _ := inner.Mask.Size()
	return op <= ip && outer.Contains(inner.IP)
}

func sandboxNetworkPolicyName(sandboxID string) string {
	return sandboxNetworkPolicyPrefix + sandboxID
}

func sandboxNetworkPolicy(namespace, sandboxID string, e *SandboxNetworkEgress) *networkingv1.NetworkPolicy {
	spec := networkingv1.NetworkPolicySpec{
		PodSelector: am.LabelSelector{MatchLabels: map[string]string{
			SandboxLabel:   "true",
			SandboxIDLabel: sandboxID,
		}},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		Egress:      []networkingv1.NetworkPolicyEgressRule{},
	}

	if !e.BlockAll {
		udp := corev1.ProtocolUDP
		tcp := corev1.ProtocolTCP
		dns := intstr.FromInt32(53)
		spec.Egress = append(spec.Egress, networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{{
				NamespaceSelector: &am.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"}},
				PodSelector:       &am.LabelSelector{MatchLabels: map[string]string{"k8s-app": "kube-dns"}},
			}},
			Ports: []networkingv1.NetworkPolicyPort{{Protocol: &udp, Port: &dns}, {Protocol: &tcp, Port: &dns}},
		})
		peers := make([]networkingv1.NetworkPolicyPeer, 0, len(e.Allow))
		for i := range e.Allow {
			b := e.Allow[i]
			peers = append(peers, networkingv1.NetworkPolicyPeer{IPBlock: &b})
		}
		spec.Egress = append(spec.Egress, networkingv1.NetworkPolicyEgressRule{To: peers})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      sandboxNetworkPolicyName(sandboxID),
			Namespace: namespace,
			Labels: map[string]string{
				"system":                       "convox",
				SandboxLabel:                   "true",
				SandboxIDLabel:                 sandboxID,
				sandboxNetworkPolicyManagedKey: "egress",
			},
		},
		Spec: spec,
	}
}

func sandboxNetworkPolicySelector() string {
	return sandboxNetworkPolicyManagedKey + "=egress"
}

// removeOrphanSandboxNetworkPolicies deletes per-sandbox egress policies whose sandbox has no pod left.
// Young policies are kept: ProcessRun creates the policy before the pod.
func (p *Provider) removeOrphanSandboxNetworkPolicies(ctx context.Context, namespace string, now time.Time) error {
	nps, err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).List(ctx, am.ListOptions{LabelSelector: sandboxNetworkPolicySelector()})
	if err != nil {
		return err
	}
	if len(nps.Items) == 0 {
		return nil
	}

	pods, err := p.Cluster.CoreV1().Pods(namespace).List(ctx, am.ListOptions{LabelSelector: SandboxLabel + "=true"})
	if err != nil {
		return err
	}
	live := map[string]bool{}
	for i := range pods.Items {
		live[pods.Items[i].Labels[SandboxIDLabel]] = true
	}

	for i := range nps.Items {
		np := nps.Items[i]
		if live[np.Labels[SandboxIDLabel]] || now.Sub(lastNeeded(&np)) < sandboxNetworkPolicyOrphanAge {
			continue
		}
		if derr := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, np.Name, am.DeleteOptions{}); derr != nil && !k8serrors.IsNotFound(derr) {
			return derr
		}
	}
	return nil
}

func (p *Provider) removeAllSandboxNetworkPolicies(ctx context.Context, namespace string) error {
	nps, err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).List(ctx, am.ListOptions{LabelSelector: sandboxNetworkPolicySelector()})
	if err != nil {
		return err
	}
	for i := range nps.Items {
		if derr := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, nps.Items[i].Name, am.DeleteOptions{}); derr != nil && !k8serrors.IsNotFound(derr) {
			return derr
		}
	}
	return nil
}
