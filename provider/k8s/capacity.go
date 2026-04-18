package k8s

import (
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func (p *Provider) CapacityGet() (*structs.Capacity, error) {
	ns, err := p.ListNodesFromInformer("")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c := &structs.Capacity{}

	for _, n := range ns.Items {
		c.ClusterCPU += n.Status.Capacity.Cpu().MilliValue()
		c.ClusterMemory += n.Status.Capacity.Memory().ScaledValue(resource.Mega)
		for key := range gpuKeyToVendor {
			if q, ok := n.Status.Capacity[corev1.ResourceName(key)]; ok {
				c.ClusterGPU += q.Value()
			}
		}
	}

	filters := []string{
		"system=convox",
		"type in (process,service)",
	}

	ps, err := p.ListPodsFromInformer("", strings.Join(filters, ","))
	if err != nil {
		return nil, err
	}

	for _, p := range ps.Items {
		c.ProcessCount += 1

		for _, pc := range p.Spec.Containers {
			c.ProcessCPU += pc.Resources.Requests.Cpu().MilliValue()
			c.ProcessMemory += pc.Resources.Requests.Memory().ScaledValue(resource.Mega)
			for key := range gpuKeyToVendor {
				if q, ok := pc.Resources.Requests[corev1.ResourceName(key)]; ok {
					c.ProcessGPU += q.Value()
				}
			}
		}
	}

	return c, nil
}
