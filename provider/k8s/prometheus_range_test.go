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

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// promRangeServer returns an httptest server that handles
// /api/v1/query_range requests with a per-metric Matrix payload.
// Metrics not in the map return an empty Matrix.
//
// The Prometheus client v1 sends QueryRange POSTs with form-urlencoded
// body containing `query`, `start`, `end`, `step`. The handler extracts
// the leading metric name from the `query` and looks up `byMetric`.
//
// `requestCount` is incremented atomically per range call so tests can
// assert "exactly N round-trips".
func promRangeServer(t *testing.T, byMetric map[string]string, requestCount *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestCount != nil {
			atomic.AddInt64(requestCount, 1)
		}
		_ = r.ParseForm()
		q := r.Form.Get("query")
		metric := q
		if i := strings.Index(q, "{"); i >= 0 {
			metric = q[:i]
		}
		w.Header().Set("Content-Type", "application/json")
		if body, has := byMetric[metric]; has {
			_, _ = w.Write([]byte(body))
			return
		}
		// Default: empty Matrix.
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
}

// rangeMatrix renders a Matrix payload with the given service+timestamp+value
// triples. Each entry produces one matrix series with the labels
// {__name__=metric, app=app1, service=svc} and the given (ts, val) points.
func rangeMatrix(metric string, samples []rangeSample) string {
	results := []string{}
	for _, s := range samples {
		labels := fmt.Sprintf(`"__name__":%q,"app":"app1","service":%q`, metric, s.Service)
		points := []string{}
		for _, p := range s.Points {
			points = append(points, fmt.Sprintf(`[%d,%q]`, p.Time, p.Value))
		}
		results = append(results, fmt.Sprintf(
			`{"metric":{%s},"values":[%s]}`,
			labels, strings.Join(points, ","),
		))
	}
	return fmt.Sprintf(
		`{"status":"success","data":{"resultType":"matrix","result":[%s]}}`,
		strings.Join(results, ","),
	)
}

type rangeSample struct {
	Service string
	Points  []rangePoint
}

type rangePoint struct {
	Time  int64
	Value string
}

// TestQueryGPURange_NilReceiverShortCircuits — calling on a nil receiver
// must return (nil, nil), preserving the fail-soft posture of the
// instant query path.
func TestQueryGPURange_NilReceiverShortCircuits(t *testing.T) {
	var pc *k8s.PrometheusClient
	got, err := pc.QueryGPURange(context.Background(), "app1", []string{"web"}, structs.MetricsOptions{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestQueryGPURange_EmptyMatrix — Prom reachable, no series for any
// metric. Returns map with one (empty) bucket per wire-name.
func TestQueryGPURange_EmptyMatrix(t *testing.T) {
	srv := promRangeServer(t, map[string]string{}, nil)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	got, err := pc.QueryGPURange(context.Background(), "app1", []string{"web"}, opts)
	require.NoError(t, err)
	require.NotNil(t, got)
	// One bucket per wire-name (8 metrics today; assert the count matches
	// GpuRangeWireNames).
	require.Len(t, got, len(k8s.GpuRangeWireNames()))
	for _, w := range k8s.GpuRangeWireNames() {
		bucket, has := got[w]
		require.True(t, has, "missing bucket for wire-name %q", w)
		require.Empty(t, bucket)
	}
}

// TestQueryGPURange_TypicalSample — happy path. Two services, one
// series each, three points per series, GPU_UTIL only. Asserts:
// - One Prom query per metric (8 total)
// - Per-service bucketing by the `service` label
// - Wire-name maps to the right Prom metric
// - Average / Min / Max all populated equally (range-query semantics)
func TestQueryGPURange_TypicalSample(t *testing.T) {
	t1 := time.Now().Truncate(time.Second).Unix()
	byMetric := map[string]string{
		"DCGM_FI_DEV_GPU_UTIL": rangeMatrix("DCGM_FI_DEV_GPU_UTIL", []rangeSample{
			{Service: "web", Points: []rangePoint{
				{Time: t1, Value: "70"},
				{Time: t1 + 30, Value: "75"},
			}},
			{Service: "inf", Points: []rangePoint{
				{Time: t1, Value: "20"},
				{Time: t1 + 30, Value: "25"},
			}},
		}),
	}
	var queryCount int64
	srv := promRangeServer(t, byMetric, &queryCount)
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	got, err := pc.QueryGPURange(context.Background(), "app1", []string{"web", "inf"}, opts)
	require.NoError(t, err)

	// One QueryRange call per wire-name.
	require.EqualValues(t, len(k8s.GpuRangeWireNames()), atomic.LoadInt64(&queryCount))

	utilByService := got["gpu-util"]
	require.Len(t, utilByService, 2)
	require.Len(t, utilByService["web"], 2)
	require.Len(t, utilByService["inf"], 2)
	assert.Equal(t, 70.0, utilByService["web"][0].Average)
	assert.Equal(t, 75.0, utilByService["web"][1].Average)
	assert.Equal(t, 20.0, utilByService["inf"][0].Average)
	assert.Equal(t, 25.0, utilByService["inf"][1].Average)
	// Range queries set min/max equal to the sample value (no aggregation
	// happening here).
	assert.Equal(t, utilByService["web"][0].Average, utilByService["web"][0].Minimum)
	assert.Equal(t, utilByService["web"][0].Average, utilByService["web"][0].Maximum)
}

// TestQueryGPURange_RegexAlternation — verifies the service filter is
// emitted as service=~"svc1|svc2" (regex alternation) and that the
// caller's pre-validated names land in the query verbatim. Without this
// the batched endpoint would issue N×M Prom queries instead of N.
func TestQueryGPURange_RegexAlternation(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// Capture only the GPU_UTIL query (fires first per gpuRangeQueries
		// order).
		q := r.Form.Get("query")
		if strings.HasPrefix(q, "DCGM_FI_DEV_GPU_UTIL") && capturedQuery == "" {
			capturedQuery = q
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	_, err = pc.QueryGPURange(context.Background(), "app1", []string{"web", "inf", "api"}, opts)
	require.NoError(t, err)
	require.NotEmpty(t, capturedQuery, "did not capture any GPU_UTIL query")
	require.Contains(t, capturedQuery, `app="app1"`)
	require.Contains(t, capturedQuery, `service=~"web|inf|api"`)
}

// TestServiceMetricsCircuitBreaker — 3 consecutive Prom timeouts within
// the trip window opens the breaker; subsequent calls fail fast with
// ErrPromCircuitOpen without hitting Prom. Resetting the breaker after
// the cooldown re-arms it.
func TestServiceMetricsCircuitBreaker(t *testing.T) {
	// Server that always errors (500).
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)

	// Start with a fresh breaker — package-level singleton may be in any
	// state from prior tests. Reset clears it.
	k8s.ResetPromCircuitBreakerForTest()
	t.Cleanup(k8s.ResetPromCircuitBreakerForTest)

	opts := structs.MetricsOptions{
		Start:  options.Time(time.Now().Add(-30 * time.Minute)),
		End:    options.Time(time.Now()),
		Period: options.Int64(30),
	}

	// Three consecutive failures opens the breaker. Each call returns
	// the underlying Prom error (5xx → http error → first fail).
	for i := 0; i < 3; i++ {
		_, err := pc.QueryGPURange(context.Background(), "app1", []string{"web"}, opts)
		require.Error(t, err, "call %d should error", i)
	}
	hitsAfter3 := atomic.LoadInt64(&hits)
	require.Greater(t, hitsAfter3, int64(0), "Prom should have been hit at least once")

	// Fourth call: breaker open → ErrPromCircuitOpen returned without
	// hitting Prom.
	_, err = pc.QueryGPURange(context.Background(), "app1", []string{"web"}, opts)
	require.Error(t, err)
	require.ErrorIs(t, err, k8s.ErrPromCircuitOpen, "expected ErrPromCircuitOpen got %v", err)
	require.Equal(t, hitsAfter3, atomic.LoadInt64(&hits), "Prom must NOT be hit while breaker is open")
}
