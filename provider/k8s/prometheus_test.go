package k8s_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promResponse renders a Prometheus-API instant-query JSON response for
// the given metric, mapping each (pod, service, value) tuple to a
// model.Vector sample. The labels emitted match what the DCGM exporter
// in kubernetes mode actually produces: plain `app`, `service`, `pod`
// (no `label_` prefix — that's a kube-state-metrics convention, not a
// DCGM one). Used by the httptest server below.
func promResponse(metric string, samples []promSample) string {
	results := []string{}
	now := time.Now().Unix()
	for _, s := range samples {
		labels := fmt.Sprintf(`"__name__":%q,"pod":%q,"app":"app1"`, metric, s.Pod)
		if s.Service != "" {
			labels += fmt.Sprintf(`,"service":%q`, s.Service)
		}
		results = append(results, fmt.Sprintf(
			`{"metric":{%s},"value":[%d,%q]}`,
			labels, now, s.Value,
		))
	}
	return fmt.Sprintf(
		`{"status":"success","data":{"resultType":"vector","result":[%s]}}`,
		strings.Join(results, ","),
	)
}

type promSample struct {
	Pod     string
	Service string
	Value   string // Prometheus serializes sample values as strings
}

// promServer stands up an httptest server that returns a per-metric
// payload from `byMetric`. Metrics not in the map return an empty Vector.
// `requestCount` is incremented atomically per /api/v1/query call.
//
// The Prometheus v1 client sends queries as POST with form-urlencoded
// body, falling back to GET on 405/501. We read the `query` param from
// either source so the test handler is symmetric on both transports.
func promServer(t *testing.T, byMetric map[string]string, requestCount *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestCount != nil {
			atomic.AddInt64(requestCount, 1)
		}
		// ParseForm merges URL.Query() and the POST body (when
		// Content-Type=application/x-www-form-urlencoded) into r.Form.
		_ = r.ParseForm()
		q := r.Form.Get("query")
		// q looks like: DCGM_FI_DEV_GPU_UTIL{app="app1",service=~"web|inf"}
		// Extract the leading metric name.
		metric := q
		if i := strings.Index(q, "{"); i >= 0 {
			metric = q[:i]
		}
		w.Header().Set("Content-Type", "application/json")
		if body, has := byMetric[metric]; has {
			_, _ = w.Write([]byte(body))
			return
		}
		// Default: empty Vector.
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
}

// int64Ptr is a small helper to take the address of an int64 literal in
// table-driven test cases. Standalone because Go does not allow `&int64(x)`.
func int64Ptr(v int64) *int64 { return &v }

// TestNewPrometheusClient_EmptyHostReturnsNil — the "no PROMETHEUS_URL
// configured" path. NewPrometheusClient returns nil when host is empty;
// callers must nil-check the result.
func TestNewPrometheusClient_EmptyHostReturnsNil(t *testing.T) {
	pc, err := k8s.NewPrometheusClient("")
	require.NoError(t, err)
	assert.Nil(t, pc)
}

// TestNewPrometheusClient_BadURLReturnsError — bad URL → caller logs and
// treats nil as "no client".
func TestNewPrometheusClient_BadURLReturnsError(t *testing.T) {
	// %ZZ is an invalid URL escape — promapi.NewClient via net/url.Parse
	// rejects it.
	pc, err := k8s.NewPrometheusClient("http://%ZZ-not-a-url")
	require.Error(t, err)
	assert.Nil(t, pc)
}

// TestQueryGPUMetrics_NilReceiverShortCircuits — calling on a nil
// receiver must return (nil, nil), enabling fail-soft callers.
func TestQueryGPUMetrics_NilReceiverShortCircuits(t *testing.T) {
	var pc *k8s.PrometheusClient
	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestQueryGPUMetrics_EmptyVector — Prometheus reachable, no samples
// (DCGM not yet emitting / no GPU pods). Must return empty map, nil
// error, distinguishing this from a transport failure.
func TestQueryGPUMetrics_EmptyVector(t *testing.T) {
	srv := promServer(t, map[string]string{}, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	require.NotNil(t, pc)

	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Len(t, got, 0)
}

// TestQueryGPUMetrics_TypicalSample — happy path. Three pods, util +
// FB_USED + FB_FREE + FB_RESERVED. After the 1L pointer-typed fields
// refactor, per-metric assertions deref *float64 / *int64 pointers;
// nil = "not reported by this pod" (distinct from non-nil zero).
func TestQueryGPUMetrics_TypicalSample(t *testing.T) {
	// Three pods on a 16 GiB T4 each: USED + FREE + RESERVED == 16384 MiB.
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "pod-a", Service: "web", Value: "70"},
			{Pod: "pod-b", Service: "web", Value: "85"},
			{Pod: "pod-c", Service: "inference", Value: "0"}, // idle GPU at 0%
		}),
		"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
			{Pod: "pod-a", Service: "web", Value: "1024"},
			{Pod: "pod-b", Service: "web", Value: "2048"},
			{Pod: "pod-c", Service: "inference", Value: "512"},
		}),
		"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
			{Pod: "pod-a", Service: "web", Value: "15048"},
			{Pod: "pod-b", Service: "web", Value: "14024"},
			{Pod: "pod-c", Service: "inference", Value: "15560"},
		}),
		"DCGM_FI_DEV_FB_RESERVED": promResponse("DCGM_FI_DEV_FB_RESERVED", []promSample{
			{Pod: "pod-a", Service: "web", Value: "312"},
			{Pod: "pod-b", Service: "web", Value: "312"},
			{Pod: "pod-c", Service: "inference", Value: "312"},
		}),
	}
	var queryCount int64
	srv := promServer(t, byMetric, &queryCount)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web", "inference"})
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Util is now *float64 — non-nil = "pod reported a sample"; deref to
	// compare. Non-nil zero (pod-c) must be distinct from nil.
	require.NotNil(t, got["pod-a"].Util)
	require.NotNil(t, got["pod-b"].Util)
	require.NotNil(t, got["pod-c"].Util)
	assert.Equal(t, 70.0, *got["pod-a"].Util)
	assert.Equal(t, 85.0, *got["pod-b"].Util)
	assert.Equal(t, 0.0, *got["pod-c"].Util,
		"non-nil zero must round-trip — distinguishes 'idle GPU' from 'no data'")

	// Memory in MiB → bytes (×1024×1024). MemUsed is *int64.
	require.NotNil(t, got["pod-a"].MemUsed)
	require.NotNil(t, got["pod-b"].MemUsed)
	require.NotNil(t, got["pod-c"].MemUsed)
	assert.Equal(t, int64(1024*1024*1024), *got["pod-a"].MemUsed)
	assert.Equal(t, int64(2048*1024*1024), *got["pod-b"].MemUsed)
	assert.Equal(t, int64(512*1024*1024), *got["pod-c"].MemUsed)

	// Total derived from USED + FREE + RESERVED == 16384 MiB. Now *int64.
	// All three FB_* parts present → MemTotal non-nil.
	require.NotNil(t, got["pod-a"].MemTotal)
	require.NotNil(t, got["pod-b"].MemTotal)
	require.NotNil(t, got["pod-c"].MemTotal)
	assert.Equal(t, int64(16384*1024*1024), *got["pod-a"].MemTotal)
	assert.Equal(t, int64(16384*1024*1024), *got["pod-b"].MemTotal)
	assert.Equal(t, int64(16384*1024*1024), *got["pod-c"].MemTotal)

	// Service mirrored from the `service` label.
	assert.Equal(t, "web", got["pod-a"].Service)
	assert.Equal(t, "web", got["pod-b"].Service)
	assert.Equal(t, "inference", got["pod-c"].Service)

	// One batched query per metric (NOT one per pod). 10 metrics →
	// 10 queries.
	assert.Equal(t, int64(10), atomic.LoadInt64(&queryCount),
		"QueryGPUMetrics must issue exactly one Prom round-trip per metric")
}

// TestQueryGPUMetrics_PartialMetricSet — DCGM emits Util but no FB_*
// metrics. After 1L: missing samples leave matching pointer nil
// rather than zero. MemTotal stays nil with no counter increment
// (memTotalParts == 0 short-circuits).
func TestQueryGPUMetrics_PartialMetricSet(t *testing.T) {
	k8s.ResetMemTotalNilCount()
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "pod-a", Service: "web", Value: "73"},
		}),
	}
	srv := promServer(t, byMetric, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	require.NoError(t, err, "partial-metric response must NOT propagate error")
	require.Len(t, got, 1)

	require.NotNil(t, got["pod-a"].Util)
	assert.Equal(t, 73.0, *got["pod-a"].Util)
	// MemUsed and MemTotal stay NIL — Vue renders "—" (no data).
	assert.Nil(t, got["pod-a"].MemUsed)
	assert.Nil(t, got["pod-a"].MemTotal)
	// memTotalNilCount unchanged: 0 FB_* samples = no-data path.
	assert.Equal(t, int64(0), k8s.MemTotalNilCount(),
		"no FB_* at all is the no-data path; counter only fires on partial")
}

// TestMemTotalPartialSeries — per-pod presence tracking for the FB_*
// trio. MemTotal is set ONLY when all three of FB_USED, FB_FREE,
// FB_RESERVED arrived for the pod; otherwise MemTotal stays nil
// (resolver decodes as null, Vue renders `—`). Per-pod accounting:
// pod A may have all three while pod B has only two.
//
// Acceptance:
//   - all three present → MemTotal correct (USED + FREE + RESERVED)
//   - one missing → MemTotal nil
//   - two missing → MemTotal nil
//   - mixed-pod (A=3, B=2) → A correct, B nil
func TestMemTotalPartialSeries(t *testing.T) {
	tests := []struct {
		name             string
		byMetric         map[string]string
		expectedMemTotal map[string]*int64
		// expectedCounterDelta — counter only fires when pod observed at
		// least one FB_* sample but not all three.
		expectedCounterDelta int64
	}{
		{
			name: "all_three_FB_present_MemTotal_correct",
			byMetric: map[string]string{
				"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "1024"},
				}),
				"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
					{Pod: "pod-a", Service: "web", Value: "15000"},
				}),
				"DCGM_FI_DEV_FB_RESERVED": promResponse("DCGM_FI_DEV_FB_RESERVED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "360"},
				}),
			},
			expectedMemTotal: map[string]*int64{
				"pod-a": int64Ptr((1024 + 15000 + 360) * 1024 * 1024),
			},
			expectedCounterDelta: 0,
		},
		{
			name: "FB_RESERVED_missing_MemTotal_nil_counter_increments",
			byMetric: map[string]string{
				"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "1024"},
				}),
				"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
					{Pod: "pod-a", Service: "web", Value: "15000"},
				}),
			},
			expectedMemTotal: map[string]*int64{
				"pod-a": nil,
			},
			expectedCounterDelta: 1,
		},
		{
			name: "FB_FREE_and_FB_RESERVED_missing_MemTotal_nil",
			byMetric: map[string]string{
				"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "1024"},
				}),
			},
			expectedMemTotal: map[string]*int64{
				"pod-a": nil,
			},
			expectedCounterDelta: 1,
		},
		{
			name: "podA_3_podB_2_partial_per_pod",
			byMetric: map[string]string{
				"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "1000"},
					{Pod: "pod-b", Service: "web", Value: "2000"},
				}),
				"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
					{Pod: "pod-a", Service: "web", Value: "5000"},
					{Pod: "pod-b", Service: "web", Value: "5000"},
				}),
				"DCGM_FI_DEV_FB_RESERVED": promResponse("DCGM_FI_DEV_FB_RESERVED", []promSample{
					{Pod: "pod-a", Service: "web", Value: "100"},
				}),
			},
			expectedMemTotal: map[string]*int64{
				"pod-a": int64Ptr((1000 + 5000 + 100) * 1024 * 1024),
				"pod-b": nil,
			},
			expectedCounterDelta: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8s.ResetMemTotalNilCount()
			srv := promServer(t, tc.byMetric, nil)
			defer srv.Close()

			pc, err := k8s.NewPrometheusClient(srv.URL)
			require.NoError(t, err)

			got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
			require.NoError(t, err)

			for pod, want := range tc.expectedMemTotal {
				gm, has := got[pod]
				require.True(t, has, "expected pod %s in result map", pod)
				if want == nil {
					assert.Nil(t, gm.MemTotal,
						"pod %s: MemTotal must be nil when FB_* trio incomplete", pod)
				} else {
					require.NotNil(t, gm.MemTotal,
						"pod %s: MemTotal must be set when all three FB_* arrived", pod)
					assert.Equal(t, *want, *gm.MemTotal,
						"pod %s: MemTotal arithmetic", pod)
				}
			}
			assert.Equal(t, tc.expectedCounterDelta, k8s.MemTotalNilCount(),
				"memTotalNilCount delta — only increments on partial (>=1 FB_* arrived but <3)")
		})
	}
}

// TestPerMetricPerPodPresence — every one of the 10 DCGM metrics is
// independently presence-tracked per pod. A pod that reports Util but
// not Tensor leaves Util non-nil and Tensor nil. Aggregation in
// service.go skips nil pointers per metric so a missing sample for
// one metric cannot pull a different metric's average toward zero.
//
// Table-driven: 8 metrics × {present, absent_others_unaffected} cases
// + an "all 10 metrics present" baseline + a regression covering the
// 5 active counters.
func TestPerMetricPerPodPresence(t *testing.T) {
	type metricCase struct {
		metricName    string
		inputValue    string
		expectedFloat float64
		expectedInt   int64
		floatField    func(gm k8s.GpuMetrics) *float64
		intField      func(gm k8s.GpuMetrics) *int64
	}

	metrics := []metricCase{
		{
			metricName: "DCGM_FI_DEV_GPU_UTIL", inputValue: "73", expectedFloat: 73,
			floatField: func(gm k8s.GpuMetrics) *float64 { return gm.Util },
		},
		{
			metricName: "DCGM_FI_DEV_FB_USED", inputValue: "2048",
			expectedInt: int64(2048) * 1024 * 1024,
			intField:    func(gm k8s.GpuMetrics) *int64 { return gm.MemUsed },
		},
		{
			metricName: "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", inputValue: "0.42",
			expectedFloat: 42.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.TensorActive },
		},
		{
			metricName: "DCGM_FI_PROF_SM_ACTIVE", inputValue: "0.55",
			expectedFloat: 55.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.SmActive },
		},
		{
			metricName: "DCGM_FI_PROF_DRAM_ACTIVE", inputValue: "0.30",
			expectedFloat: 30.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.DramActive },
		},
		{
			metricName: "DCGM_FI_PROF_PIPE_FP16_ACTIVE", inputValue: "0.50",
			expectedFloat: 50.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.Fp16Active },
		},
		{
			metricName: "DCGM_FI_PROF_PIPE_FP32_ACTIVE", inputValue: "0.50",
			expectedFloat: 50.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.Fp32Active },
		},
		{
			metricName: "DCGM_FI_DEV_POWER_USAGE", inputValue: "175",
			expectedFloat: 175.0,
			floatField:    func(gm k8s.GpuMetrics) *float64 { return gm.PowerW },
		},
	}

	allPresentByMetric := func() map[string]string {
		return map[string]string{
			"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
				{Pod: "pod-a", Service: "web", Value: "73"},
			}),
			"DCGM_FI_DEV_FB_USED": promResponse("DCGM_FI_DEV_FB_USED", []promSample{
				{Pod: "pod-a", Service: "web", Value: "2048"},
			}),
			"DCGM_FI_DEV_FB_FREE": promResponse("DCGM_FI_DEV_FB_FREE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "10000"},
			}),
			"DCGM_FI_DEV_FB_RESERVED": promResponse("DCGM_FI_DEV_FB_RESERVED", []promSample{
				{Pod: "pod-a", Service: "web", Value: "100"},
			}),
			"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.42"},
			}),
			"DCGM_FI_PROF_SM_ACTIVE": promResponse("DCGM_FI_PROF_SM_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.55"},
			}),
			"DCGM_FI_PROF_DRAM_ACTIVE": promResponse("DCGM_FI_PROF_DRAM_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.30"},
			}),
			"DCGM_FI_PROF_PIPE_FP16_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_FP16_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.50"},
			}),
			"DCGM_FI_PROF_PIPE_FP32_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_FP32_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.50"},
			}),
			"DCGM_FI_DEV_POWER_USAGE": promResponse("DCGM_FI_DEV_POWER_USAGE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "175"},
			}),
		}
	}

	for _, mc := range metrics {
		mc := mc
		t.Run(mc.metricName+"_present", func(t *testing.T) {
			byMetric := allPresentByMetric()
			srv := promServer(t, byMetric, nil)
			defer srv.Close()

			pc, err := k8s.NewPrometheusClient(srv.URL)
			require.NoError(t, err)

			got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
			require.NoError(t, err)
			require.Len(t, got, 1)
			gm := got["pod-a"]

			if mc.floatField != nil {
				p := mc.floatField(gm)
				require.NotNil(t, p, "metric %s present must populate field", mc.metricName)
				assert.InDelta(t, mc.expectedFloat, *p, 0.0001,
					"metric %s value (after rack-side scaling)", mc.metricName)
			} else if mc.intField != nil {
				p := mc.intField(gm)
				require.NotNil(t, p, "metric %s present must populate field", mc.metricName)
				assert.Equal(t, mc.expectedInt, *p,
					"metric %s value (after MiB→bytes)", mc.metricName)
			}
		})

		t.Run(mc.metricName+"_absent_other_metrics_unaffected", func(t *testing.T) {
			byMetric := allPresentByMetric()
			delete(byMetric, mc.metricName)
			srv := promServer(t, byMetric, nil)
			defer srv.Close()

			pc, err := k8s.NewPrometheusClient(srv.URL)
			require.NoError(t, err)

			got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
			require.NoError(t, err)
			require.Len(t, got, 1)
			gm := got["pod-a"]

			if mc.floatField != nil {
				assert.Nil(t, mc.floatField(gm),
					"metric %s absent must leave field nil", mc.metricName)
			} else if mc.intField != nil {
				assert.Nil(t, mc.intField(gm),
					"metric %s absent must leave field nil", mc.metricName)
			}

			if mc.metricName != "DCGM_FI_DEV_GPU_UTIL" {
				require.NotNil(t, gm.Util,
					"unrelated metric Util must remain populated when %s absent", mc.metricName)
				assert.Equal(t, 73.0, *gm.Util)
			} else {
				require.NotNil(t, gm.PowerW,
					"unrelated metric PowerW must remain populated when %s absent", mc.metricName)
				assert.Equal(t, 175.0, *gm.PowerW)
			}
		})
	}

	t.Run("all_10_metrics_present_all_populated", func(t *testing.T) {
		byMetric := allPresentByMetric()
		srv := promServer(t, byMetric, nil)
		defer srv.Close()

		pc, err := k8s.NewPrometheusClient(srv.URL)
		require.NoError(t, err)

		got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
		require.NoError(t, err)
		require.Len(t, got, 1)
		gm := got["pod-a"]

		assert.NotNil(t, gm.Util)
		assert.NotNil(t, gm.MemUsed)
		assert.NotNil(t, gm.MemTotal, "MemTotal derived from 3 FB_* present")
		assert.NotNil(t, gm.TensorActive)
		assert.NotNil(t, gm.SmActive)
		assert.NotNil(t, gm.DramActive)
		assert.NotNil(t, gm.Fp16Active)
		assert.NotNil(t, gm.Fp32Active)
		assert.NotNil(t, gm.PowerW)

		// Regression spot: fp16/fp32 raw 0.5 must produce 50.0
		// (×100 to align with util/sm/dram/tensor convention).
		assert.Equal(t, 50.0, *gm.Fp16Active,
			"fp16 raw 0.5 must become 50.0 (×100)")
		assert.Equal(t, 50.0, *gm.Fp32Active,
			"fp32 raw 0.5 must become 50.0 (×100)")
	})

	// Regression: assert ALL 5 active-pipe counters convert raw
	// 0.5 → 50.0 in lockstep. Guards against any setter dropping ×100.
	t.Run("active_counters_5x_convert_0.5_to_50.0", func(t *testing.T) {
		byMetric := map[string]string{
			"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.5"},
			}),
			"DCGM_FI_PROF_SM_ACTIVE": promResponse("DCGM_FI_PROF_SM_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.5"},
			}),
			"DCGM_FI_PROF_DRAM_ACTIVE": promResponse("DCGM_FI_PROF_DRAM_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.5"},
			}),
			"DCGM_FI_PROF_PIPE_FP16_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_FP16_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.5"},
			}),
			"DCGM_FI_PROF_PIPE_FP32_ACTIVE": promResponse("DCGM_FI_PROF_PIPE_FP32_ACTIVE", []promSample{
				{Pod: "pod-a", Service: "web", Value: "0.5"},
			}),
		}
		srv := promServer(t, byMetric, nil)
		defer srv.Close()

		pc, err := k8s.NewPrometheusClient(srv.URL)
		require.NoError(t, err)

		got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
		require.NoError(t, err)
		gm := got["pod-a"]

		require.NotNil(t, gm.TensorActive)
		require.NotNil(t, gm.SmActive)
		require.NotNil(t, gm.DramActive)
		require.NotNil(t, gm.Fp16Active)
		require.NotNil(t, gm.Fp32Active)
		assert.Equal(t, 50.0, *gm.TensorActive)
		assert.Equal(t, 50.0, *gm.SmActive)
		assert.Equal(t, 50.0, *gm.DramActive)
		assert.Equal(t, 50.0, *gm.Fp16Active)
		assert.Equal(t, 50.0, *gm.Fp32Active)
	})
}

// TestQueryGPUMetrics_EmptyPodLabel — sample with empty "pod" label
// must be silently skipped. Other samples in the same Vector populate
// correctly. No panic, no error.
func TestQueryGPUMetrics_EmptyPodLabel(t *testing.T) {
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "", Service: "web", Value: "99"},
			{Pod: "pod-real", Service: "web", Value: "50"},
		}),
	}
	srv := promServer(t, byMetric, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	require.NoError(t, err)
	require.Len(t, got, 1, "empty-pod sample must be skipped from result map")

	_, hasReal := got["pod-real"]
	assert.True(t, hasReal)
	_, hasEmpty := got[""]
	assert.False(t, hasEmpty, "empty pod key must not appear in the result map")
}

// TestQueryGPUMetrics_Timeout — server sleeps past 5s client deadline.
// Client returns context-deadline-exceeded within 5s.
func TestQueryGPUMetrics_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	start := time.Now()
	_, err = pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 6*time.Second,
		"timeout must trigger before the server's 6s sleep elapses (5s client deadline)")
}

// TestQueryGPUMetrics_NoServicesArg — empty services must still issue
// the query (filtered only by app) and parse the response.
func TestQueryGPUMetrics_NoServicesArg(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		capturedQuery = r.Form.Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	_, err = pc.QueryGPUMetrics(context.Background(), "app1", nil)
	require.NoError(t, err)
	assert.Contains(t, capturedQuery, `app="app1"`,
		"empty services must still send app filter")
	assert.NotContains(t, capturedQuery, "service=",
		"empty services must NOT add service regex filter")
}

// TestQueryGPUMetrics_ConcurrentSafe — race-test surface for callers.
func TestQueryGPUMetrics_ConcurrentSafe(t *testing.T) {
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "pod-a", Service: "web", Value: "50"},
		}),
	}
	srv := promServer(t, byMetric, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	const goroutines = 8
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = pc.QueryGPUMetrics(ctx, "app1", []string{"web"})
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

// TestQueryGPUMetrics_ServiceWithSpecialChars — kebab-case service names
// pass through the regex alternation safely.
func TestQueryGPUMetrics_ServiceWithSpecialChars(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		capturedQuery = r.Form.Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	_, err = pc.QueryGPUMetrics(context.Background(), "app1",
		[]string{"web-api", "inference-cuda", "worker-gpu"})
	require.NoError(t, err)
	assert.Contains(t, capturedQuery, `service=~"web-api|inference-cuda|worker-gpu"`,
		"service alternation must be pipe-joined inside a regex match")
}

// TestServiceGPUFieldsJSONNilRoundtrip — wire-boundary
// roundtrip for the rack-side Service struct's pointer-typed GPU fields.
//
// The contract: a rack-side Service with some GPU fields nil and others
// populated must JSON-marshal with `omitempty` stripping nil fields,
// then JSON-unmarshal back into a new Service struct preserving nil
// pointers for the missing fields. Decoders downstream (console mirror
// struct, SDK clients) rely on the nil round-trip semantics to
// distinguish "field absent on rack response" from "field present with
// zero value".
//
// This test covers the rack-side JSON → struct half of the boundary;
// the Vue side (resolver returning nil renders `—`) is covered in
// console3 tests.
func TestServiceGPUFieldsJSONNilRoundtrip(t *testing.T) {
	util := 73.5
	var memUsed int64 = 8 * 1024 * 1024 * 1024
	powerW := 175.0
	want := structs.Service{
		Name:       "vllm",
		Gpu:        1,
		GpuVendor:  "nvidia",
		GpuUtil:    &util,
		GpuMemUsed: &memUsed,
		GpuPowerW:  &powerW,
		// Nil fields — must NOT appear in JSON output (omitempty), must
		// round-trip back to nil:
		GpuMemTotal:     nil,
		GpuTensorActive: nil,
		GpuSmActive:     nil,
		GpuDramActive:   nil,
		GpuFp16Active:   nil,
		GpuFp32Active:   nil,
	}

	data, err := json.Marshal(want)
	require.NoError(t, err)
	body := string(data)

	// Populated fields appear with new (post-rename) JSON tags.
	require.Contains(t, body, `"gpu-util":73.5`)
	require.Contains(t, body, `"gpu-mem-used":8589934592`)
	require.Contains(t, body, `"gpu-power-w":175`)
	// Nil fields stripped by omitempty:
	require.NotContains(t, body, "gpu-mem-total")
	require.NotContains(t, body, "gpu-tensor-active")
	require.NotContains(t, body, "gpu-sm-active")
	require.NotContains(t, body, "gpu-dram-active")
	require.NotContains(t, body, "gpu-fp16-active")
	require.NotContains(t, body, "gpu-fp32-active")
	// And the OLD (pre-rename) tags MUST NOT appear at all — guards
	// against regression of the rename:
	require.NotContains(t, body, "gpu-util-avg")
	require.NotContains(t, body, "gpu-mem-used-avg")
	require.NotContains(t, body, "gpu-mem-total-avg")
	require.NotContains(t, body, "gpu-tensor-active-avg")
	require.NotContains(t, body, "gpu-sm-active-avg")
	require.NotContains(t, body, "gpu-dram-active-avg")
	require.NotContains(t, body, "gpu-fp16-active-avg")
	require.NotContains(t, body, "gpu-fp32-active-avg")
	require.NotContains(t, body, "gpu-power-w-avg")

	// Round-trip — populated fields preserved; nil preserved as nil
	// pointers. Load-bearing wire-boundary guarantee.
	var got structs.Service
	require.NoError(t, json.Unmarshal(data, &got))

	require.NotNil(t, got.GpuUtil)
	assert.Equal(t, util, *got.GpuUtil)
	require.NotNil(t, got.GpuMemUsed)
	assert.Equal(t, memUsed, *got.GpuMemUsed)
	require.NotNil(t, got.GpuPowerW)
	assert.Equal(t, powerW, *got.GpuPowerW)

	assert.Nil(t, got.GpuMemTotal,
		"nil pointer must survive marshal/unmarshal — not become &(zero)")
	assert.Nil(t, got.GpuTensorActive)
	assert.Nil(t, got.GpuSmActive)
	assert.Nil(t, got.GpuDramActive)
	assert.Nil(t, got.GpuFp16Active)
	assert.Nil(t, got.GpuFp32Active)
}
