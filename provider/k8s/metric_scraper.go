package k8s

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	rackMetricsInterval = 30 * time.Second
	rackMetricsTimeout  = 10 * time.Second
	rackMetricsWindow   = 3 * time.Hour
)

type rackMetricsSample struct {
	time time.Time
	cpu  float64
	mem  float64
}

type MetricScraperClient struct {
	cluster  kubernetes.Interface
	metrics  metricsclientset.Interface
	provider *Provider
	now      func() time.Time
	running  atomic.Bool

	mu      sync.RWMutex
	samples []rackMetricsSample
}

func NewMetricScraperClient(cluster kubernetes.Interface, metrics metricsclientset.Interface) *MetricScraperClient {
	return &MetricScraperClient{
		cluster: cluster,
		metrics: metrics,
		now:     time.Now,
	}
}

func (m *MetricScraperClient) SetProvider(p *Provider) {
	m.provider = p
}

func (m *MetricScraperClient) ListNodes() (*corev1.NodeList, error) {
	if m.provider != nil && m.provider.nodeInformer != nil {
		ns, err := m.provider.nodeInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, errors.WithStack(err)
		}

		nl := &corev1.NodeList{Items: make([]corev1.Node, len(ns))}
		for i := range ns {
			nl.Items[i] = *ns[i]
		}

		return nl, nil
	}

	ns, err := m.cluster.CoreV1().Nodes().List(context.TODO(), am.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return ns, nil
}

func (m *MetricScraperClient) Run(ctx context.Context) {
	m.running.Store(true)
	m.tick(ctx)

	t := time.NewTicker(rackMetricsInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.tick(ctx)
		}
	}
}

func (m *MetricScraperClient) tick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=metrics at=error kind=panic_recovered recovered=%v\n", r)
		}
	}()

	m.prune()

	if err := m.sample(ctx); err != nil {
		fmt.Printf("ns=metrics at=sample error=%q\n", err)
	}
}

func (m *MetricScraperClient) prune() {
	cutoff := m.now().Add(-rackMetricsWindow)

	m.mu.Lock()
	defer m.mu.Unlock()

	i := 0
	for i < len(m.samples) && !m.samples[i].time.After(cutoff) {
		i++
	}
	m.samples = m.samples[i:]
}

func (m *MetricScraperClient) sample(ctx context.Context) error {
	ns, err := m.ListNodes()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, rackMetricsTimeout)
	defer cancel()

	nms, err := m.metrics.MetricsV1beta1().NodeMetricses().List(ctx, am.ListOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	usage := map[string]corev1.ResourceList{}
	for i := range nms.Items {
		usage[nms.Items[i].Name] = nms.Items[i].Usage
	}

	var cpu, mem, cpuAllocatable, memAllocatable int64
	for i := range ns.Items {
		u, ok := usage[ns.Items[i].Name]
		if !ok {
			continue
		}
		cpu += u.Cpu().MilliValue()
		mem += u.Memory().Value()
		cpuAllocatable += ns.Items[i].Status.Allocatable.Cpu().MilliValue()
		memAllocatable += ns.Items[i].Status.Allocatable.Memory().Value()
	}

	if cpuAllocatable == 0 || memAllocatable == 0 {
		return errors.Errorf("no metrics for %d nodes", len(ns.Items))
	}

	s := rackMetricsSample{
		time: m.now().UTC().Truncate(time.Second),
		cpu:  caculatePercentage(float64(cpu), float64(cpuAllocatable)),
		mem:  caculatePercentage(float64(mem), float64(memAllocatable)),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.samples = append(m.samples, s)

	return nil
}

func (m *MetricScraperClient) series() (*structs.Metric, *structs.Metric) {
	cpum := &structs.Metric{Name: "cluster:cpu:utilization"}
	memm := &structs.Metric{Name: "cluster:mem:utilization"}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.samples {
		cpum.Values = append(cpum.Values, structs.MetricValue{Average: s.cpu, Count: 1, Maximum: s.cpu, Minimum: s.cpu, Sum: s.cpu, Time: s.time})
		memm.Values = append(memm.Values, structs.MetricValue{Average: s.mem, Count: 1, Maximum: s.mem, Minimum: s.mem, Sum: s.mem, Time: s.time})
	}

	return cpum, memm
}

func (m *MetricScraperClient) GetRackMetrics(opts structs.MetricsOptions) (structs.Metrics, error) {
	if !m.running.Load() {
		return nil, errors.WithStack(structs.ErrNotImplemented("unimplemented"))
	}

	cpum, memm := m.series()

	if len(cpum.Values) == 0 {
		return nil, structs.ErrNotImplemented("metrics unavailable")
	}

	if opts.Period != nil {
		cpum = aggregateMetricByPeriod(cpum, *opts.Period)
		memm = aggregateMetricByPeriod(memm, *opts.Period)
	}
	if opts.Start != nil {
		cpum = filterMetricByStart(cpum, *opts.Start)
		memm = filterMetricByStart(memm, *opts.Start)
	}

	return structs.Metrics{*cpum, *memm}, nil
}
