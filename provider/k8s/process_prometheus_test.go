package k8s_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// TestProcessList_PromEnrichment_HappyPath wires a Prometheus test server
// onto the Provider and exercises ProcessList → QueryGPUMetrics → struct
// pointer population. Verifies the per-pod GPU pointers are non-nil and
// carry the expected values.
func TestProcessList_PromEnrichment_HappyPath(t *testing.T) {
	srv := promServer(t, map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "gpu-pod-1", Service: "infer", Value: "75"},
		}),
		"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
			{Pod: "gpu-pod-1", Service: "infer", Value: "4096"},
		}),
		"DCGM_FI_DEV_FB_TOTAL": promResponse("DCGM_FI_DEV_FB_TOTAL", []promSample{
			{Pod: "gpu-pod-1", Service: "infer", Value: "8192"},
		}),
	}, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	require.NotNil(t, pc)

	testProvider(t, func(p *k8s.Provider) {
		// Inject the PromClient. testProvider creates the Provider with
		// PromClient nil by default (matching the FromEnv() short-circuit
		// for non-AWS / no-PROMETHEUS_URL racks).
		p.PromClient = pc

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		// One GPU pod, one CPU-only pod. Only the GPU pod should pick up
		// telemetry; the CPU pod's GpuUtil/MemUsed/MemTotal stay nil.
		require.NoError(t, processCreateGpu(kk, "rack1-app1", "gpu-pod-1",
			"system=convox,rack=rack1,app=app1,service=infer,type=service",
			"nvidia.com/gpu", 1))
		require.NoError(t, processCreate(kk, "rack1-app1", "cpu-pod-1",
			"system=convox,rack=rack1,app=app1,service=infer,type=service"))

		pss, err := p.ProcessList("app1", structs.ProcessListOptions{})
		require.NoError(t, err)
		require.Len(t, pss, 2)

		byId := map[string]structs.Process{}
		for _, ps := range pss {
			byId[ps.Id] = ps
		}

		// GPU pod: pointers populated.
		gpu := byId["gpu-pod-1"]
		require.NotNil(t, gpu.GpuUtil, "GPU pod must have GpuUtil populated")
		assert.Equal(t, 75.0, *gpu.GpuUtil)
		require.NotNil(t, gpu.GpuMemUsed)
		assert.Equal(t, int64(4096*1024*1024), *gpu.GpuMemUsed)
		require.NotNil(t, gpu.GpuMemTotal)
		assert.Equal(t, int64(8192*1024*1024), *gpu.GpuMemTotal)

		// CPU-only pod: pointers stay nil (Gpu==0 → enrichment branch
		// skipped per the `pss[i].Gpu == 0 { continue }` short-circuit).
		cpu := byId["cpu-pod-1"]
		assert.Nil(t, cpu.GpuUtil, "CPU pod must NOT have GpuUtil populated")
		assert.Nil(t, cpu.GpuMemUsed)
		assert.Nil(t, cpu.GpuMemTotal)
	})
}

// TestProcessList_PromTimeout — Prometheus query times out at the 5s
// deadline. ProcessList still succeeds; non-GPU process fields are
// fully populated; GPU pointer fields stay nil. The graceful-degradation
// contract: GPU enrichment is best-effort, the existing process listing
// remains correct on Prom failure (R1 BLOCK-3-IT7 / R2 MR-19).
func TestProcessList_PromTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep past the 5s client deadline; the client returns a
		// context-deadline-exceeded error.
		time.Sleep(6 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	testProvider(t, func(p *k8s.Provider) {
		p.PromClient = pc

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, processCreateGpu(kk, "rack1-app1", "gpu-pod-1",
			"system=convox,rack=rack1,app=app1,service=infer,type=service",
			"nvidia.com/gpu", 1))

		start := time.Now()
		pss, err := p.ProcessList("app1", structs.ProcessListOptions{})
		elapsed := time.Since(start)

		// ProcessList succeeds — the existing slice is fully populated.
		require.NoError(t, err)
		require.Len(t, pss, 1)

		// Non-GPU fields populated normally.
		assert.Equal(t, "gpu-pod-1", pss[0].Id)
		assert.Equal(t, "infer", pss[0].Name)
		assert.Equal(t, 1, pss[0].Gpu)

		// GPU pointer fields stay nil — query timed out, enrichment block
		// logged the error and absorbed it.
		assert.Nil(t, pss[0].GpuUtil)
		assert.Nil(t, pss[0].GpuMemUsed)
		assert.Nil(t, pss[0].GpuMemTotal)

		// The whole call returned within ~5s (the client deadline), not
		// 6s (the server sleep). This pins the timeout contract.
		assert.Less(t, elapsed, 6*time.Second)
	})
}

// TestProcessList_PromNilClient — when PromClient is nil (no
// PROMETHEUS_URL configured), ProcessList must still return the existing
// process listing with GPU pointer fields all nil. This is the default
// path on every non-AWS rack and on AWS racks without
// observability configured. Wire-shape identical to pre-3.24.6.
func TestProcessList_PromNilClient(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		// PromClient stays nil — testProvider creates the Provider
		// without setting it.
		assert.Nil(t, p.PromClient)

		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, processCreateGpu(kk, "rack1-app1", "gpu-pod-1",
			"system=convox,rack=rack1,app=app1,service=infer,type=service",
			"nvidia.com/gpu", 1))

		pss, err := p.ProcessList("app1", structs.ProcessListOptions{})
		require.NoError(t, err)
		require.Len(t, pss, 1)
		assert.Equal(t, 1, pss[0].Gpu)

		// All GPU pointers nil — omitempty strips them from the JSON
		// wire shape; non-AWS / no-Prom-URL clients see no change.
		assert.Nil(t, pss[0].GpuUtil)
		assert.Nil(t, pss[0].GpuMemUsed)
		assert.Nil(t, pss[0].GpuMemTotal)
	})
}
