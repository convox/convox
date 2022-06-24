package k8s

import (
	"fmt"
	"io"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (p *Provider) InstanceKeyroll() error {
	return errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) InstanceList() (structs.Instances, error) {
	ns, err := p.Cluster.CoreV1().Nodes().List(am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	metricsByNode := map[string]metricsv1beta1.NodeMetrics{}
	ms, err := p.MetricsClient.MetricsV1beta1().NodeMetricses().List(am.ListOptions{})
	if err != nil {
		p.logger.Errorf("failed to fetch node metrics: %s", err)
	} else {
		for _, m := range ms.Items {
			metricsByNode[m.ObjectMeta.Name] = m
		}
	}

	is := structs.Instances{}

	for _, n := range ns.Items {
		pds, err := p.Cluster.CoreV1().Pods("").List(am.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", n.ObjectMeta.Name)})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		status := "pending"

		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" && c.Status == "True" {
				status = "running"
			}
		}

		private := ""
		public := ""

		for _, na := range n.Status.Addresses {
			switch na.Type {
			case ac.NodeExternalIP:
				public = na.Address
			case ac.NodeInternalIP:
				private = na.Address
			}
		}

		var cpu, mem float64
		if m, has := metricsByNode[n.ObjectMeta.Name]; has {
			cpu = toCpuCore(m.Usage.Cpu().MilliValue())
			mem = toMemMB(m.Usage.Memory().Value())
		}

		cpuCapacity := toCpuCore(n.Status.Capacity.Cpu().MilliValue())
		memCapacity := toMemMB(n.Status.Capacity.Memory().Value())

		cpuAllocatable := toCpuCore(n.Status.Allocatable.Cpu().MilliValue())
		memAllocatable := toMemMB(n.Status.Allocatable.Memory().Value())

		is = append(is, structs.Instance{
			Cpu:               cpu,
			CpuCapacity:       cpuCapacity,
			CpuAllocatable:    cpuAllocatable,
			Id:                n.ObjectMeta.Name,
			Memory:            mem,
			MemoryCapacity:    memCapacity,
			MemoryAllocatable: memAllocatable,
			PrivateIp:         private,
			Processes:         len(pds.Items),
			PublicIp:          public,
			Started:           n.CreationTimestamp.Time,
			Status:            status,
		})
	}

	return is, nil
}

func (p *Provider) InstanceShell(id string, rw io.ReadWriter, opts structs.InstanceShellOptions) (int, error) {
	return 0, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) InstanceTerminate(id string) error {
	return errors.WithStack(fmt.Errorf("unimplemented"))
}
