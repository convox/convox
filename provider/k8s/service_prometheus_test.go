package k8s_test

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// gpuDeploymentFixture creates a Deployment that ServiceList will pick
// up (matching the "app=app1,type=service" label selector). The named
// service requests 1 nvidia.com/gpu so the GPU branch in ServiceList
// runs and includes this service in the QueryGPUMetrics call.
func gpuDeploymentFixture(t *testing.T, c *fake.Clientset, ns, name string) {
	t.Helper()
	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": "app1", "type": "service", "service": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Template: ac.PodTemplateSpec{
				Spec: ac.PodSpec{
					Containers: []ac.Container{{
						Name: "app1",
						Resources: ac.ResourceRequirements{
							Requests: ac.ResourceList{
								"nvidia.com/gpu": resource.MustParse("1"),
							},
						},
					}},
				},
			},
		},
	}
	_, err := c.AppsV1().Deployments(ns).Create(context.TODO(), dep, am.CreateOptions{})
	require.NoError(t, err)
}

// TestServiceListAggregation_AverageAcrossPods — a service with multiple
// GPU pods reports averaged Util/MemUsed/MemTotal. Pin: average = sum /
// count over the pods that reported a sample for that service. Pods
// scraped but bucketed into other services don't pull the denominator.
//
// Spec test name: TestServiceListAggregation_AverageAcrossPods
// (R1 BLOCK-3-IT7 / R2 MR-09 — load-bearing aggregation arithmetic).
func TestServiceListAggregation_AverageAcrossPods(t *testing.T) {
	// 3 pods labeled service=infer with util 70, 80, 90.
	// Average = 80.0; sum mem-used = 1024+2048+512 = 3584 MiB → avg = 1194.66 MiB.
	srv := promServer(t, map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "pod-a", Service: "infer", Value: "70"},
			{Pod: "pod-b", Service: "infer", Value: "80"},
			{Pod: "pod-c", Service: "infer", Value: "90"},
			// A pod from a different service in the same Vector — must
			// NOT pull the infer denominator.
			{Pod: "pod-d", Service: "web", Value: "10"},
		}),
		"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
			{Pod: "pod-a", Service: "infer", Value: "1024"},
			{Pod: "pod-b", Service: "infer", Value: "2048"},
			{Pod: "pod-c", Service: "infer", Value: "512"},
		}),
		// MemTotal is derived from FB_USED + FB_FREE + FB_RESERVED — the
		// DCGM exporter's default-counters.csv does not emit FB_TOTAL. Each
		// pod's three values sum to 8192 MiB.
		"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
			{Pod: "pod-a", Service: "infer", Value: "7104"},
			{Pod: "pod-b", Service: "infer", Value: "6080"},
			{Pod: "pod-c", Service: "infer", Value: "7616"},
		}),
		"DCGM_FI_DEV_FB_RESERVED": promResponse("DCGM_FI_DEV_FB_RESERVED", []promSample{
			{Pod: "pod-a", Service: "infer", Value: "64"},
			{Pod: "pod-b", Service: "infer", Value: "64"},
			{Pod: "pod-c", Service: "infer", Value: "64"},
		}),
	}, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	testProvider(t, func(p *k8s.Provider) {
		p.PromClient = pc

		kk, _ := p.Cluster.(*fake.Clientset)
		cc, _ := p.Convox.(*cvfake.Clientset)
		aa, _ := p.Atom.(*atom.MockInterface)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// Inline a manifest with a single gpu service so ServiceList's
		// deployment builder runs against m.Service(name).
		manifest := "services:\n  infer:\n    port: 5000\n"
		releaseID := "rel1"
		aa.On("Status", "rack1-app1", "app").Return("Running", releaseID, nil)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", releaseID, manifest))

		gpuDeploymentFixture(t, kk, "rack1-app1", "infer")

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, ss, 1)
		s := ss[0]
		assert.Equal(t, "infer", s.Name)
		require.Equal(t, 1, s.Gpu)

		// Aggregation arithmetic: 3 pods, util sum = 240, avg = 80.
		require.NotNil(t, s.GpuUtil)
		assert.InDelta(t, 80.0, *s.GpuUtil, 0.001)

		// MemUsed: (1024+2048+512) MiB / 3 = 1194.66 MiB → bytes.
		// integer division on the bytes after MiB→bytes conversion:
		// (1024+2048+512)*1024*1024 = 3,758,096,384; /3 = 1,252,698,794.
		require.NotNil(t, s.GpuMemUsed)
		assert.InDelta(t, int64(1252698794), *s.GpuMemUsed, 1<<20,
			"GpuMemUsed within 1 MiB of (1024+2048+512)/3 MiB in bytes")

		// MemTotal: all 3 = 8192 MiB → avg = 8192 MiB in bytes.
		require.NotNil(t, s.GpuMemTotal)
		assert.Equal(t, int64(8192*1024*1024), *s.GpuMemTotal)
	})
}

// TestServiceList_PromNilClient — when PromClient is nil, ServiceList
// returns the existing slice unchanged. GPU avg pointers all nil.
// Wire-shape identical to pre-3.24.6.
func TestServiceList_PromNilClient(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		assert.Nil(t, p.PromClient)

		kk, _ := p.Cluster.(*fake.Clientset)
		cc, _ := p.Convox.(*cvfake.Clientset)
		aa, _ := p.Atom.(*atom.MockInterface)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		manifest := "services:\n  infer:\n    port: 5000\n"
		releaseID := "rel1"
		aa.On("Status", "rack1-app1", "app").Return("Running", releaseID, nil)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", releaseID, manifest))

		gpuDeploymentFixture(t, kk, "rack1-app1", "infer")

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, ss, 1)
		assert.Equal(t, 1, ss[0].Gpu)
		// Pointers stay nil on the no-Prom path; omitempty strips the
		// keys from JSON output.
		assert.Nil(t, ss[0].GpuUtil)
		assert.Nil(t, ss[0].GpuMemUsed)
		assert.Nil(t, ss[0].GpuMemTotal)
	})
}

// TestServiceList_NoGpuServices — when no service has Gpu>0, the Prom
// query is skipped (no round-trip); aggregation block short-circuits.
// This pins the "lower Prom load" optimization: services with no GPU
// don't generate a query that always returns empty.
func TestServiceList_NoGpuServices(t *testing.T) {
	var queryCount int64
	srv := promServer(t, map[string]string{}, &queryCount)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	testProvider(t, func(p *k8s.Provider) {
		p.PromClient = pc

		kk, _ := p.Cluster.(*fake.Clientset)
		cc, _ := p.Convox.(*cvfake.Clientset)
		aa, _ := p.Atom.(*atom.MockInterface)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		manifest := "services:\n  web:\n    port: 5000\n"
		releaseID := "rel1"
		aa.On("Status", "rack1-app1", "app").Return("Running", releaseID, nil)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", releaseID, manifest))

		// CPU-only deployment (no GPU resource request).
		replicas := int32(1)
		dep := &appsv1.Deployment{
			ObjectMeta: am.ObjectMeta{
				Name:      "web",
				Namespace: "rack1-app1",
				Labels:    map[string]string{"app": "app1", "type": "service", "service": "web"},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
			},
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Create(context.TODO(), dep, am.CreateOptions{})
		require.NoError(t, err)

		ss, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, ss, 1)
		assert.Equal(t, 0, ss[0].Gpu)
		assert.Nil(t, ss[0].GpuUtil)

		// Critical assertion: zero queries — the gpuServices slice was
		// empty so QueryGPUMetrics was skipped entirely.
		assert.Equal(t, int64(0), queryCount,
			"ServiceList must NOT issue Prom queries when no service has Gpu>0")
	})
}
