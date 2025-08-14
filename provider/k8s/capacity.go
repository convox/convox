package k8s

import (
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
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
		}
	}

	return c, nil
}
