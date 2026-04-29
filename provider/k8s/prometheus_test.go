package k8s_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promResponse renders a Prometheus-API instant-query JSON response for
// the given metric, mapping each (pod, label_service, value) tuple to a
// model.Vector sample. Used by the httptest server below.
func promResponse(metric string, samples []promSample) string {
	results := []string{}
	now := time.Now().Unix()
	for _, s := range samples {
		labels := fmt.Sprintf(`"__name__":%q,"pod":%q,"label_app":"app1"`, metric, s.Pod)
		if s.Service != "" {
			labels += fmt.Sprintf(`,"label_service":%q`, s.Service)
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
		// q looks like: DCGM_FI_DEV_GPU_UTIL{label_app="app1",label_service=~"web|inf"}
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

// TestNewPrometheusClient_EmptyHostReturnsNil — the "no PROMETHEUS_URL
// configured" path. Spec §"NewPrometheusClient returns nil when host is
// empty — callers must nil-check the result."
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

// TestQueryGPUMetrics_TypicalSample — happy path. Three pods, one util
// query, one mem-used query, one mem-total query. Result map has correct
// per-pod aggregation and Service field populated from label_service.
func TestQueryGPUMetrics_TypicalSample(t *testing.T) {
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
		"DCGM_FI_DEV_FB_TOTAL": promResponse("DCGM_FI_DEV_FB_TOTAL", []promSample{
			{Pod: "pod-a", Service: "web", Value: "8192"},
			{Pod: "pod-b", Service: "web", Value: "8192"},
			{Pod: "pod-c", Service: "inference", Value: "8192"},
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

	// Util passes through as-is (percent).
	assert.Equal(t, 70.0, got["pod-a"].Util)
	assert.Equal(t, 85.0, got["pod-b"].Util)
	assert.Equal(t, 0.0, got["pod-c"].Util)

	// Memory in MiB → bytes (×1024×1024).
	assert.Equal(t, int64(1024*1024*1024), got["pod-a"].MemUsed)
	assert.Equal(t, int64(2048*1024*1024), got["pod-b"].MemUsed)
	assert.Equal(t, int64(512*1024*1024), got["pod-c"].MemUsed)
	assert.Equal(t, int64(8192*1024*1024), got["pod-a"].MemTotal)

	// Service mirrored from label_service.
	assert.Equal(t, "web", got["pod-a"].Service)
	assert.Equal(t, "web", got["pod-b"].Service)
	assert.Equal(t, "inference", got["pod-c"].Service)

	// Single batched query per metric (not one per pod). 3 metrics → 3
	// queries. The "lower latency, lower Prom load" contract from the
	// spec.
	assert.Equal(t, int64(3), atomic.LoadInt64(&queryCount),
		"QueryGPUMetrics must issue exactly one Prom round-trip per metric")
}

// TestQueryGPUMetrics_PartialMetricSet — DCGM emits a Vector for util
// but errors / empty for memory queries. Per-metric fail-soft on a
// partial response is the customer-facing reliability deliverable
// (R1 BLOCK-3-IT7 / R2 MR-09 / MR-19).
//
// Util populates; mem-used / mem-total stay zero (default); the call
// returns no error so the upstream caller's struct fields with the util
// pointer set still surface to the wire.
func TestQueryGPUMetrics_PartialMetricSet(t *testing.T) {
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "pod-a", Service: "web", Value: "73"},
		}),
		// FB_USED and FB_TOTAL not in the map → server returns empty Vector.
	}
	srv := promServer(t, byMetric, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	got, err := pc.QueryGPUMetrics(context.Background(), "app1", []string{"web"})
	require.NoError(t, err, "partial-metric response must NOT propagate error")
	require.Len(t, got, 1)

	assert.Equal(t, 73.0, got["pod-a"].Util)
	// MemUsed and MemTotal stay at the GpuMetrics zero value because the
	// Vector for those metrics was empty. Caller (process.go / service.go)
	// then writes nil pointers out, omitempty strips the JSON keys, Vue
	// renders "no data" empty state — exactly per the spec contract.
	assert.Equal(t, int64(0), got["pod-a"].MemUsed)
	assert.Equal(t, int64(0), got["pod-a"].MemTotal)
}

// TestQueryGPUMetrics_EmptyPodLabel — a sample with empty "pod" label
// (e.g., a non-pod-attributed exporter target leaking into the same Prom
// job) must be silently skipped. Other samples in the same Vector
// populate correctly. No panic, no error.
func TestQueryGPUMetrics_EmptyPodLabel(t *testing.T) {
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": promResponse("DCGM_FI_DEV_GPU_UTIL", []promSample{
			{Pod: "", Service: "web", Value: "99"}, // malformed — must be skipped
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

	// The empty-pod sample never makes it into the keyspace — only the
	// real pod is present.
	_, hasReal := got["pod-real"]
	assert.True(t, hasReal)
	_, hasEmpty := got[""]
	assert.False(t, hasEmpty, "empty pod key must not appear in the result map")
}

// TestQueryGPUMetrics_Timeout — server sleeps past the 5s client deadline.
// Client returns context-deadline-exceeded within 5s; result map is the
// partial accumulator (empty in this case, but the contract is "return
// what we have, error explains why").
func TestQueryGPUMetrics_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the 5s deadline. Spec: "Timeout: server sleeps
		// 6s; client returns context-deadline-exceeded error within 5s."
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

// TestQueryGPUMetrics_NoServicesArg — when called with no services
// filter, must still issue the query (filtered only by label_app) and
// parse the response. Tests the empty-services branch of the filter
// builder.
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
	assert.Contains(t, capturedQuery, `label_app="app1"`,
		"empty services must still send label_app filter")
	assert.NotContains(t, capturedQuery, "label_service",
		"empty services must NOT add label_service regex filter")
}

// TestQueryGPUMetrics_ConcurrentSafe — race-test surface for callers that
// invoke ServiceList and ProcessList concurrently with the same
// PrometheusClient. The promapi client is safe for concurrent use; this
// test pins that contract so a regression that introduces a non-thread-
// safe internal cache is caught at -race time.
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

// TestQueryGPUMetrics_ServiceWithSpecialChars — service names matching
// the convox.yml pattern ([a-z0-9-]+) are safe in the label_service
// regex alternation. This test pins that no escaping is needed for the
// canonical character set; if a future scheme introduces dots / slashes
// that break PromQL, the regression surfaces here.
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

	// Convox service names are kebab-case lowercase per convox.yml schema.
	_, err = pc.QueryGPUMetrics(context.Background(), "app1",
		[]string{"web-api", "inference-cuda", "worker-gpu"})
	require.NoError(t, err)
	assert.Contains(t, capturedQuery, `label_service=~"web-api|inference-cuda|worker-gpu"`,
		"service alternation must be pipe-joined inside a regex match")
}
