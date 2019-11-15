package k8s

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) BalancerList(app string) (structs.Balancers, error) {
	if _, err := p.AppGet(app); err != nil {
		return nil, err
	}

	ss, err := p.Cluster.CoreV1().Services(p.AppNamespace(app)).List(am.ListOptions{
		LabelSelector: fmt.Sprintf("system=convox,type=balancer,app=%s", app),
	})
	if err != nil {
		return nil, err
	}

	bs := structs.Balancers{}

	for _, s := range ss.Items {
		b := structs.Balancer{
			Name:    s.Labels["balancer"],
			Ports:   structs.BalancerPorts{},
			Service: s.Labels["service"],
		}

		if is := s.Status.LoadBalancer.Ingress; len(is) > 0 {
			if ip := is[0].IP; ip != "" {
				b.Endpoint = ip
			}
			if host := is[0].Hostname; host != "" {
				b.Endpoint = host
			}
		}

		for _, p := range s.Spec.Ports {
			b.Ports = append(b.Ports, structs.BalancerPort{
				Protocol: string(p.Protocol),
				Source:   int(p.Port),
				Target:   p.TargetPort.IntValue(),
			})
		}

		bs = append(bs, b)
	}

	return bs, nil
}
