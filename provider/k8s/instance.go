package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (p *Provider) InstanceKeyroll() (*structs.KeyPair, error) {
	return nil, errors.WithStack(fmt.Errorf("unimplemented"))
}

func (p *Provider) InstanceList() (structs.Instances, error) {
	ns, err := p.Cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	metricsByNode := map[string]metricsv1beta1.NodeMetrics{}
	ms, err := p.MetricsClient.MetricsV1beta1().NodeMetricses().List(context.TODO(), am.ListOptions{})
	if err != nil {
		p.logger.Errorf("failed to fetch node metrics: %s", err)
	} else {
		for _, m := range ms.Items {
			metricsByNode[m.ObjectMeta.Name] = m
		}
	}

	is := structs.Instances{}

	for _, n := range ns.Items {
		pds, err := p.Cluster.CoreV1().Pods("").List(context.TODO(), am.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", n.ObjectMeta.Name)})
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
	instances, err := p.InstanceList()
	if err != nil {
		return 0, err
	}

	var instance *structs.Instance
	for i := range instances {
		if instances[i].Id == id {
			instance = &instances[i]
		}
	}
	if instance == nil {
		return 0, fmt.Errorf("instance not found")
	}

	if opts.PrivateKey == nil || *opts.PrivateKey == "" {
		return 0, fmt.Errorf("private key is not provided")
	}

	privateKeyBytes, err := base64.StdEncoding.DecodeString(*opts.PrivateKey)
	if len(privateKeyBytes) == 0 {
		return 0, err
	}

	// configure SSH client
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return 0, err
	}

	config := &ssh.ClientConfig{
		User:            "ec2-user",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	ip := instance.PrivateIp
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()

	// Setup I/O
	session.Stdout = rw
	session.Stdin = rw
	session.Stderr = rw

	width := 0
	height := 0

	if opts.Width != nil {
		width = *opts.Width
	}

	if opts.Height != nil {
		height = *opts.Height
	}

	if err := session.RequestPty("xterm", height, width, ssh.TerminalModes{}); err != nil {
		return 0, err
	}

	code := 0

	if opts.Command != nil {
		if err := session.Start(*opts.Command); err != nil {
			return 0, err
		}
	} else {
		if err := session.Shell(); err != nil {
			return 0, err
		}
	}

	if err := session.Wait(); err != nil {
		if ee, ok := err.(*ssh.ExitError); ok {
			code = ee.Waitmsg.ExitStatus()
		}
	}

	return code, nil
}

func (p *Provider) InstanceTerminate(id string) error {
	return errors.WithStack(fmt.Errorf("unimplemented"))
}
