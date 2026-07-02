package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	imdsBlockPolicyName           = "convox-imds-block"
	podImdsBlockReconcileInterval = 2 * time.Minute
)

func imdsBlockNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: am.ObjectMeta{
			Name:      imdsBlockPolicyName,
			Namespace: namespace,
			Labels:    map[string]string{"system": "convox"},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: am.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{
						CIDR:   "0.0.0.0/0",
						Except: []string{"169.254.169.254/32"},
					},
				}},
			}},
		},
	}
}

func (p *Provider) reconcilePodImdsBlock(ctx context.Context) error {
	nss, err := p.ListNamespacesFromInformer(fmt.Sprintf("system=convox,rack=%s,type=app", p.Name))
	if err != nil {
		return errors.WithStack(err)
	}

	for i := range nss.Items {
		ns := nss.Items[i].Name

		var rerr error
		if p.PodImdsBlockEnabled {
			rerr = p.ensureImdsBlockPolicy(ctx, ns)
		} else {
			rerr = p.removeImdsBlockPolicy(ctx, ns)
		}

		if rerr != nil {
			fmt.Printf("ns=pod_imds_block at=warn kind=reconcile rack=%s namespace=%s err=%q\n", p.Name, ns, rerr)
		}
	}

	return nil
}

func (p *Provider) ensureImdsBlockPolicy(ctx context.Context, namespace string) error {
	_, err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Create(ctx, imdsBlockNetworkPolicy(namespace), am.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (p *Provider) removeImdsBlockPolicy(ctx context.Context, namespace string) error {
	err := p.Cluster.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, imdsBlockPolicyName, am.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (p *Provider) reconcilePodImdsBlockSafe(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=pod_imds_block at=error kind=panic_recovered rack=%s recovered=%v\n", p.Name, r)
		}
	}()

	if err := p.reconcilePodImdsBlock(ctx); err != nil {
		fmt.Printf("ns=pod_imds_block at=warn kind=reconcile rack=%s err=%q\n", p.Name, err)
	}
}

func (p *Provider) runPodImdsBlockReconciler(ctx context.Context) {
	p.reconcilePodImdsBlockSafe(ctx)

	tick := time.NewTicker(podImdsBlockReconcileInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.reconcilePodImdsBlockSafe(ctx)
		}
	}
}
