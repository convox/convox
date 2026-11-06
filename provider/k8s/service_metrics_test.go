package k8s_test

import (
	"context"
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

// TestServiceMetrics_Impl_NoPromClientReturnsEmpty — the V3 fail-soft
// posture: when PROMETHEUS_URL is unset, every call returns an empty
// Metrics slice with nil error so the resolver can render an empty
// chart without surfacing an error to the operator.
func TestServiceMetrics_Impl_NoPromClientReturnsEmpty(t *testing.T) {
	p := &k8s.Provider{}
	got, err := p.ServiceMetrics("app1", "web", structs.MetricsOptions{})
	require.NoError(t, err)
	require.Equal(t, structs.Metrics{}, got)
}

// TestServiceMetrics_Impl_HappyPath — Prom returns a Matrix; the impl
// projects to one Metric row per wire-name in stable order. Asserts:
// - One row per wire-name (8 rows today)
// - Wire-names match GpuRangeWireNames()
// - Values are populated for the queried service (web)
func TestServiceMetrics_Impl_HappyPath(t *testing.T) {
	t1 := time.Now().Truncate(time.Second).Unix()
	body := `{"status":"success","data":{"resultType":"matrix","result":[
		{"metric":{"__name__":"DCGM_FI_DEV_GPU_UTIL","app":"app1","service":"web"},
		 "values":[[` + itoa(t1) + `,"73.5"],[` + itoa(t1+30) + `,"75.0"]]}
	]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form.Get("query")
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(q, "DCGM_FI_DEV_GPU_UTIL") {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	p := &k8s.Provider{PromClient: pc}

	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	got, err := p.ServiceMetrics("app1", "web", opts)
	require.NoError(t, err)

	// Wire-name ordering matches GpuRangeWireNames.
	wireNames := k8s.GpuRangeWireNames()
	require.Len(t, got, len(wireNames))
	for i, m := range got {
		assert.Equal(t, wireNames[i], m.Name, "row %d wire-name drift", i)
	}

	// gpu-util has the values we returned.
	require.Len(t, got[0].Values, 2)
	assert.Equal(t, 73.5, got[0].Values[0].Average)
	assert.Equal(t, 75.0, got[0].Values[1].Average)
}

// TestMetricsByService_Impl_HappyPath — batched per-service shape:
// one ServiceMetricsRow per requested service; each row has one Metric
// per wire-name; web has two GPU_UTIL points; inf has a separate set.
func TestMetricsByService_Impl_HappyPath(t *testing.T) {
	t1 := time.Now().Truncate(time.Second).Unix()
	body := `{"status":"success","data":{"resultType":"matrix","result":[
		{"metric":{"__name__":"DCGM_FI_DEV_GPU_UTIL","app":"app1","service":"web"},
		 "values":[[` + itoa(t1) + `,"73.5"]]},
		{"metric":{"__name__":"DCGM_FI_DEV_GPU_UTIL","app":"app1","service":"inf"},
		 "values":[[` + itoa(t1) + `,"22.0"]]}
	]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		q := r.Form.Get("query")
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(q, "DCGM_FI_DEV_GPU_UTIL") {
			_, _ = w.Write([]byte(body))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	p := &k8s.Provider{PromClient: pc}

	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	got, err := p.MetricsByService("app1", []string{"web", "inf"}, opts)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "web", got[0].Name)
	assert.Equal(t, "inf", got[1].Name)
	require.NotEmpty(t, got[0].Metrics)
	require.NotEmpty(t, got[1].Metrics)
	// gpu-util is wire-name index 0 in GpuRangeWireNames.
	require.Equal(t, "gpu-util", got[0].Metrics[0].Name)
	require.NotEmpty(t, got[0].Metrics[0].Values)
	assert.Equal(t, 73.5, got[0].Metrics[0].Values[0].Average)
	require.Equal(t, "gpu-util", got[1].Metrics[0].Name)
	require.NotEmpty(t, got[1].Metrics[0].Values)
	assert.Equal(t, 22.0, got[1].Metrics[0].Values[0].Average)
}

// TestMetricsByService_Impl_BatchedRegexAlternation — verifies that
// MetricsByService issues exactly N PromQL QueryRange calls (one per
// metric), NOT N×M. Catches the half-built-endpoint regression where
// a future refactor might fan out to per-service queries.
func TestMetricsByServiceBatched(t *testing.T) {
	var queryCount int64
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&queryCount, 1)
		_ = r.ParseForm()
		q := r.Form.Get("query")
		// Capture the GPU_UTIL filter to assert the regex alternation.
		if strings.HasPrefix(q, "DCGM_FI_DEV_GPU_UTIL") && capturedQuery == "" {
			capturedQuery = q
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	p := &k8s.Provider{PromClient: pc}

	now := time.Now()
	opts := structs.MetricsOptions{
		Start:  options.Time(now.Add(-30 * time.Minute)),
		End:    options.Time(now),
		Period: options.Int64(30),
	}
	got, err := p.MetricsByService("app1", []string{"web", "inf", "api"}, opts)
	require.NoError(t, err)

	// Exactly one query per wire-name (8 today). NOT N services × M metrics.
	require.EqualValues(t, len(k8s.GpuRangeWireNames()), atomic.LoadInt64(&queryCount),
		"expected one query per wire-name (got %d, wire-names=%d)",
		queryCount, len(k8s.GpuRangeWireNames()))

	// The captured GPU_UTIL query carries regex alternation.
	require.Contains(t, capturedQuery, `service=~"web|inf|api"`)

	// Response shape: one row per requested service even though Prom
	// returned no data — rows are emitted with empty Metrics for the
	// "requested but empty" UI case.
	require.Len(t, got, 3)
	for i, name := range []string{"web", "inf", "api"} {
		assert.Equal(t, name, got[i].Name)
		require.NotNil(t, got[i].Metrics)
		// Each requested service has one Metric row per wire-name (with
		// empty Values).
		require.Len(t, got[i].Metrics, len(k8s.GpuRangeWireNames()))
	}
}

// TestMetricsByService_Impl_NoPromClient — fail-soft when Prom is
// unconfigured: returns one empty row per requested service. Catches
// the "requested but empty vs not requested" distinction the UI relies
// on.
func TestMetricsByService_Impl_NoPromClient(t *testing.T) {
	p := &k8s.Provider{}
	got, err := p.MetricsByService("app1", []string{"web", "inf"}, structs.MetricsOptions{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "web", got[0].Name)
	assert.Equal(t, "inf", got[1].Name)
	assert.Equal(t, structs.Metrics{}, got[0].Metrics)
	assert.Equal(t, structs.Metrics{}, got[1].Metrics)
}

// TestMetricsByService_Impl_EmptyServices — defensive: zero services
// returns empty slice and never hits Prom. Caller (the controller)
// already prevents this; impl handles it gracefully.
func TestMetricsByService_Impl_EmptyServices(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	pc, err := k8s.NewPrometheusClient(srv.URL)
	require.NoError(t, err)
	p := &k8s.Provider{PromClient: pc}
	got, err := p.MetricsByService("app1", nil, structs.MetricsOptions{})
	require.NoError(t, err)
	require.Equal(t, []structs.ServiceMetricsRow{}, got)
	require.EqualValues(t, 0, atomic.LoadInt64(&hits), "Prom must not be hit when services is empty")
}

// TestProviderInitialize_RejectsStoredInvalidPrometheusURL — SSRF
// startup re-validation. A stored value that bypassed the param-set
// validator (e.g. a manually edited configmap) is re-checked on
// Initialize; PromClient is dropped to nil when the URL is hostile.
//
// This test exercises the helper directly (ValidatePrometheusURL) —
// the full Initialize path goes through a configmap fetch / k8s API
// which is out of scope for a unit test. Test cases here cover only
// inputs that do NOT require live DNS: IP literals, the
// *.svc.cluster.local allowlist, scheme rejection, and reserved
// names. DNS-resolution behaviour is exercised in pkg/validator with
// a stubbed resolver — see pkg/validator/ssrf_test.go.
func TestProviderInitialize_RejectsStoredInvalidPrometheusURL(t *testing.T) {
	cases := []struct {
		name      string
		raw       string
		shouldErr bool
	}{
		{"empty", "", false},
		{"in_cluster_suffix", "http://prom.kube-system.svc.cluster.local:9090", false},
		{"in_cluster_paid_recipe", "http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090", false},
		{"file_scheme", "file:///etc/passwd", true},
		{"localhost", "http://localhost:9090", true},
		{"loopback_ip", "http://127.0.0.1:9090", true},
		{"private_ip", "http://10.0.0.1:9090", true},
		{"link_local", "http://169.254.169.254:80", true}, // AWS metadata
		{"missing_scheme", "prom.example.com:9090", true},
		{"unspecified", "http://0.0.0.0:9090", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := k8s.ValidatePrometheusURL(c.raw)
			if c.shouldErr {
				require.Error(t, err, "expected %q to be rejected", c.raw)
			} else {
				require.NoError(t, err, "expected %q to pass", c.raw)
			}
		})
	}
}

// itoa is a small helper to convert int64 to string in test fixtures
// (avoids importing strconv just for one-line use).
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// TestServiceMetricsBoundsValidation is a sanity-only test asserting
// the helpers exposed for tests work — bounds enforcement itself is
// covered by the controller-level tests in pkg/api/service_metrics_test.go.
// Keeping a smoke-level test here ensures the validate path compiles
// and the wire-names list is non-empty.
func TestServiceMetricsBoundsValidation(t *testing.T) {
	require.NotEmpty(t, k8s.GpuRangeWireNames())
	// Sentinel: gpu-util is the first wire-name (chart code depends on
	// this for the default chart axis label).
	require.Equal(t, "gpu-util", k8s.GpuRangeWireNames()[0])
}

// silence unused-import noise from refactors that drop assertions
var _ = context.Background
