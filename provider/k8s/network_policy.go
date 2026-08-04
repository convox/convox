package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	networkIsolationPolicyName     = "convox-network-isolation"
	networkPolicyReconcileInterval = 2 * time.Minute
)

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
			PodSelector: am.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
		},
	}
}

func (p *Provider) reconcileNetworkPolicy(ctx context.Context) error {
	nss, err := p.ListNamespacesFromInformer(fmt.Sprintf("system=convox,rack=%s,type=app", p.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range nss.Items {
		ns := nss.Items[i].Name

		if rerr := p.reconcileNetworkPolicyNamespace(ctx, ns, nss.Items[i].Labels["tid"]); rerr != nil {
			fmt.Printf("ns=network_policy at=warn kind=reconcile rack=%s namespace=%s err=%q\n", p.Name, ns, rerr)
		}
	}

	return nil
}

func (p *Provider) reconcileNetworkPolicyNamespace(ctx context.Context, namespace, tid string) error {
	if !p.NetworkPolicyEnabled {
		return p.removeNetworkIsolationPolicy(ctx, namespace)
	}

	// Instance-target NLB traffic arrives from whichever node received it, which no peer can match.
	balanced, err := p.namespaceHasBalancer(ctx, namespace)
	if err != nil {
		return err
	}

	if balanced {
		fmt.Printf("ns=network_policy at=skip kind=balancer rack=%s namespace=%s\n", p.Name, namespace)
		return p.removeNetworkIsolationPolicy(ctx, namespace)
	}

	return p.ensureNetworkIsolationPolicy(ctx, namespace, tid)
}

func (p *Provider) namespaceHasBalancer(ctx context.Context, namespace string) (bool, error) {
	ss, err := p.Cluster.CoreV1().Services(namespace).List(ctx, am.ListOptions{LabelSelector: "type=balancer"})
	if err != nil {
		return false, err
	}

	return len(ss.Items) > 0, nil
}

func (p *Provider) ensureNetworkIsolationPolicy(ctx context.Context, namespace, tid string) error {
	np := networkIsolationPolicy(namespace, p.Namespace, p.Name, tid)

	nps := p.Cluster.NetworkingV1().NetworkPolicies(namespace)

	existing, err := nps.Get(ctx, networkIsolationPolicyName, am.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		if _, cerr := nps.Create(ctx, np, am.CreateOptions{}); cerr != nil && !k8serrors.IsAlreadyExists(cerr) {
			return cerr
		}

		return nil
	}

	if apiequality.Semantic.DeepEqual(existing.Spec, np.Spec) {
		return nil
	}

	existing.Spec = np.Spec

	if _, uerr := nps.Update(ctx, existing, am.UpdateOptions{}); uerr != nil {
		return uerr
	}

	return nil
}

func (p *Provider) removeNetworkIsolationPolicy(ctx context.Context, namespace string) error {
	err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, networkIsolationPolicyName, am.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *Provider) reconcileNetworkPolicySafe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=network_policy at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	if err := p.reconcileNetworkPolicy(ctx); err != nil {
		fmt.Printf("ns=network_policy at=warn kind=reconcile rack=%s err=%q\n", p.Name, err)
	}
}

func (p *Provider) runNetworkPolicyReconciler(ctx context.Context) {
	p.reconcileNetworkPolicySafe(ctx)

	tick := time.NewTicker(networkPolicyReconcileInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reconcileNetworkPolicySafe(ctx)
		}
	}
}
