package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	mv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func scraperNode(name, cpu, mem string) *ac.Node {
	return &ac.Node{
		ObjectMeta: am.ObjectMeta{Name: name},
		Status: ac.NodeStatus{
			Allocatable: ac.ResourceList{
				ac.ResourceCPU:    resource.MustParse(cpu),
				ac.ResourceMemory: resource.MustParse(mem),
			},
		},
	}
}

func scraperNodeMetrics(t *testing.T, mc *metricfake.Clientset, name, cpu, mem string) {
	t.Helper()

	nm := &mv1beta1.NodeMetrics{
		ObjectMeta: am.ObjectMeta{Name: name},
		Usage: ac.ResourceList{
			ac.ResourceCPU:    resource.MustParse(cpu),
			ac.ResourceMemory: resource.MustParse(mem),
		},
	}

	require.NoError(t, mc.Tracker().Create(mv1beta1.SchemeGroupVersion.WithResource("nodes"), nm, ""))
}

func scraperClient(t *testing.T) (*MetricScraperClient, *metricfake.Clientset, *time.Time) {
	t.Helper()

	kc := fake.NewSimpleClientset(
		scraperNode("node-1", "2", "4Gi"),
		scraperNode("node-2", "2000m", "4Gi"),
		scraperNode("node-3", "4", "8Gi"),
	)

	mc := metricfake.NewSimpleClientset()
	scraperNodeMetrics(t, mc, "node-1", "500m", "1Gi")
	scraperNodeMetrics(t, mc, "node-2", "1500m", "1Gi")

	now := time.Date(2026, 10, 25, 12, 0, 0, 500, time.UTC)

	m := NewMetricScraperClient(kc, mc)
	m.now = func() time.Time { return now }

	return m, mc, &now
}

func TestMetricScraperSample(t *testing.T) {
	m, _, _ := scraperClient(t)

	require.NoError(t, m.sample(context.TODO()))

	require.Equal(t, []rackMetricsSample{
		{time: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC), cpu: 50, mem: 25},
	}, m.samples)
}

func TestMetricScraperRun(t *testing.T) {
	m, _, _ := scraperClient(t)

	ctx, cancel := context.WithCancel(context.TODO())
	cancel()
	m.Run(ctx)

	ms, err := m.GetRackMetrics(structs.MetricsOptions{})
	require.NoError(t, err)
	require.Len(t, ms[0].Values, 1)
	require.Equal(t, 50.0, ms[0].Values[0].Average)
}

func TestMetricScraperListNodesInformer(t *testing.T) {
	m, _, _ := scraperClient(t)

	ni := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Core().V1().Nodes()
	require.NoError(t, ni.Informer().GetIndexer().Add(scraperNode("node-1", "2", "4Gi")))
	m.SetProvider(&Provider{nodeInformer: ni})

	ns, err := m.ListNodes()
	require.NoError(t, err)
	require.Len(t, ns.Items, 1)
	require.Equal(t, "node-1", ns.Items[0].Name)
}

func TestMetricScraperSampleErrors(t *testing.T) {
	m, mc, _ := scraperClient(t)
	mc.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("service unavailable")
	})

	require.ErrorContains(t, m.sample(context.TODO()), "service unavailable")
	require.Empty(t, m.samples)

	m = NewMetricScraperClient(fake.NewSimpleClientset(scraperNode("node-1", "2", "4Gi")), metricfake.NewSimpleClientset())

	require.Error(t, m.sample(context.TODO()))
	require.Empty(t, m.samples)
}

func TestMetricScraperWindow(t *testing.T) {
	m, mc, now := scraperClient(t)
	m.running.Store(true)
	start := *now

	m.tick(context.TODO())
	*now = start.Add(2 * time.Hour)
	m.tick(context.TODO())
	require.Len(t, m.samples, 2)

	mc.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("service unavailable")
	})

	*now = start.Add(3*time.Hour + 30*time.Second)
	m.tick(context.TODO())
	require.Len(t, m.samples, 1)
	require.Equal(t, start.Add(2*time.Hour).Truncate(time.Second), m.samples[0].time)

	*now = start.Add(6 * time.Hour)
	m.tick(context.TODO())
	require.Empty(t, m.samples)

	_, err := m.GetRackMetrics(structs.MetricsOptions{})
	require.EqualError(t, err, "metrics unavailable")
}

func TestMetricScraperNotRunning(t *testing.T) {
	m, _, _ := scraperClient(t)
	require.NoError(t, m.sample(context.TODO()))

	_, err := m.GetRackMetrics(structs.MetricsOptions{})
	require.EqualError(t, err, "unimplemented")
}

func TestMetricScraperOptions(t *testing.T) {
	m, _, _ := scraperClient(t)
	m.running.Store(true)

	base := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		m.samples = append(m.samples, rackMetricsSample{time: base.Add(time.Duration(i) * 30 * time.Second), cpu: float64(i), mem: float64(100 - i)})
	}
	stored := append([]rackMetricsSample{}, m.samples...)

	cpu := &structs.Metric{Name: "cluster:cpu:utilization"}
	mem := &structs.Metric{Name: "cluster:mem:utilization"}
	for _, s := range stored {
		cpu.Values = append(cpu.Values, structs.MetricValue{Average: s.cpu, Count: 1, Maximum: s.cpu, Minimum: s.cpu, Sum: s.cpu, Time: s.time})
		mem.Values = append(mem.Values, structs.MetricValue{Average: s.mem, Count: 1, Maximum: s.mem, Minimum: s.mem, Sum: s.mem, Time: s.time})
	}

	ms, err := m.GetRackMetrics(structs.MetricsOptions{})
	require.NoError(t, err)
	require.Equal(t, structs.Metrics{*cpu, *mem}, ms)

	start := base.Add(2 * time.Minute)
	ms, err = m.GetRackMetrics(structs.MetricsOptions{Period: options.Int64(60), Start: &start})
	require.NoError(t, err)

	want := structs.Metrics{
		*filterMetricByStart(aggregateMetricByPeriod(cpu, 60), start),
		*filterMetricByStart(aggregateMetricByPeriod(mem, 60), start),
	}
	require.Equal(t, want, ms)
	require.NotEmpty(t, ms[0].Values)
	require.True(t, ms[0].Values[0].Time.After(ms[0].Values[len(ms[0].Values)-1].Time))

	require.Equal(t, stored, m.samples)
}
