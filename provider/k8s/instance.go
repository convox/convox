package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	ac "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (p *Provider) InstanceKeyroll() (*structs.KeyPair, error) {
	return nil, errors.WithStack(structs.ErrNotImplemented("unimplemented"))
}

func (p *Provider) InstanceList() (structs.Instances, error) {
	ns, err := p.ListNodesFromInformer("")
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
		return 0, structs.ErrNotFound("instance not found")
	}

	if opts.PrivateKey == nil || *opts.PrivateKey == "" {
		return 0, structs.ErrBadRequest("private key is not provided")
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // skipcq
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

const (
	drainTimeout          = 5 * time.Minute
	evictionRetryInterval = 5 * time.Second
)

func (p *Provider) InstanceTerminate(id string) error {
	ctx := context.TODO()

	node, err := p.Cluster.CoreV1().Nodes().Get(ctx, id, am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return errors.WithStack(structs.ErrNotFound("instance not found: %s", id))
		}
		return errors.WithStack(err)
	}

	// cordon the node (mark unschedulable)
	if !node.Spec.Unschedulable {
		patch := []byte(`{"spec":{"unschedulable":true}}`)
		if _, err := p.Cluster.CoreV1().Nodes().Patch(ctx, id, types.StrategicMergePatchType, patch, am.PatchOptions{}); err != nil {
			return errors.WithStack(fmt.Errorf("failed to cordon node %s: %s", id, err))
		}
	}

	nodeReady := isNodeReady(node)

	if err := p.drainNode(ctx, id, nodeReady); err != nil {
		return errors.WithStack(fmt.Errorf("failed to drain node %s: %s", id, err))
	}

	if err := p.Cluster.CoreV1().Nodes().Delete(ctx, id, am.DeleteOptions{}); err != nil {
		if !ae.IsNotFound(err) {
			return errors.WithStack(fmt.Errorf("failed to delete node %s: %s", id, err))
		}
	}

	return nil
}

func isNodeReady(node *ac.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == ac.NodeReady {
			return c.Status == ac.ConditionTrue
		}
	}
	return false
}

func (p *Provider) drainNode(ctx context.Context, nodeName string, nodeReady bool) error {
	pods, err := p.Cluster.CoreV1().Pods("").List(ctx, am.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	})
	if err != nil {
		return errors.WithStack(err)
	}

	deadline := time.Now().Add(drainTimeout)

	for i := range pods.Items {
		pod := &pods.Items[i]

		// skip mirror pods (managed by kubelet directly)
		if _, isMirror := pod.Annotations[ac.MirrorPodAnnotationKey]; isMirror {
			continue
		}

		// skip DaemonSet-managed pods
		if isDaemonSetPod(pod) {
			continue
		}

		if nodeReady {
			if err := p.evictPod(ctx, pod, deadline); err != nil {
				return err
			}
		} else {
			// on NotReady nodes the kubelet can't process evictions
			if err := p.forceDeletePod(ctx, pod); err != nil {
				return err
			}
		}
	}

	return nil
}

func isDaemonSetPod(pod *ac.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func (p *Provider) evictPod(ctx context.Context, pod *ac.Pod, deadline time.Time) error {
	eviction := &policyv1.Eviction{
		ObjectMeta: am.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}

	for {
		err := p.Cluster.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
		if err == nil {
			return nil
		}

		if ae.IsNotFound(err) {
			return nil
		}

		// PDB is blocking eviction — retry until deadline
		if ae.IsTooManyRequests(err) {
			if time.Now().After(deadline) {
				return p.forceDeletePod(ctx, pod)
			}
			time.Sleep(evictionRetryInterval)
			continue
		}

		// other errors — force delete
		return p.forceDeletePod(ctx, pod)
	}
}

func (p *Provider) forceDeletePod(ctx context.Context, pod *ac.Pod) error {
	grace := int64(0)
	err := p.Cluster.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, am.DeleteOptions{
		GracePeriodSeconds: &grace,
	})
	if err != nil && !ae.IsNotFound(err) {
		return errors.WithStack(fmt.Errorf("failed to force-delete pod %s/%s: %s", pod.Namespace, pod.Name, err))
	}
	return nil
}
