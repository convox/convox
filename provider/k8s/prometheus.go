package k8s

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/convox/convox/pkg/structs"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// GpuMetrics is the per-pod sample bundle returned by QueryGPUMetrics.
// Numeric metric fields are pointer-typed so a nil pointer signals
// "metric absent for this pod in the response Vector" (per-metric per-pod
// presence — DCGM exporter version skew can leave any of the 10 metrics
// missing for a subset of pods). Aggregation code in enrichGpuTelemetry
// must skip nil values per metric and accumulate per-metric counts so a
// service whose pods report e.g. Util but not Tensor cannot pull a zero
// into the Tensor average.
//
// Service carries the value of the DCGM `service` Prometheus label —
// the DCGM exporter in kubernetes mode mirrors the pod's K8s `service`
// label directly (plain key, no `label_` prefix). Used by service.go
// aggregation to bucket pods by service without a second Prom round-trip.
type GpuMetrics struct {
	Util     *float64 // percent 0-100
	MemUsed  *int64   // bytes (FB used)
	MemTotal *int64   // bytes (FB total — derived; nil unless USED+FREE+RESERVED all present)
	Service  string   // value of Prom `service` label (pod's `service` K8s label)

	// Extended DCGM profiling counters. All optional — nil pointer means the
	// dcgm-exporter chart isn't yet emitting that field (older default-
	// counters.csv) OR the GPU type doesn't support the metric (e.g. FP16
	// active on H100 where DCGM may not expose tensor-pipe FP16). Vue side
	// renders "no data" empty state when the corresponding pointer is nil
	// after the resolver decode.
	TensorActive *float64 // percent 0-100 (DCGM_FI_PROF_PIPE_TENSOR_ACTIVE × 100)
	SmActive     *float64 // percent 0-100 (DCGM_FI_PROF_SM_ACTIVE × 100)
	DramActive   *float64 // percent 0-100 (DCGM_FI_PROF_DRAM_ACTIVE × 100)
	Fp16Active   *float64 // percent 0-100 (DCGM_FI_PROF_PIPE_FP16_ACTIVE × 100)
	Fp32Active   *float64 // percent 0-100 (DCGM_FI_PROF_PIPE_FP32_ACTIVE × 100)
	PowerW       *float64 // watts (DCGM_FI_DEV_POWER_USAGE)

	// memTotalParts tracks per-pod presence of the three FB_* series that
	// are summed into MemTotal. MemTotal is set to nil unless all three
	// (USED, FREE, RESERVED) are present; otherwise the derived total
	// would understate card capacity by reserved bytes (typically 1-2 GiB
	// on H100/A100). Internal accounting only — not on the wire.
	memTotalParts uint8
	// memTotalRaw accumulates the running sum (in bytes) of the three FB_*
	// series; promoted to MemTotal pointer only when memTotalParts ==
	// memTotalAllPresent.
	memTotalRaw int64
}

// memTotalPart bitmask values for tracking which of the three derived
// FB_* series have been observed for a given pod. MemTotal is only
// reliable when all three bits are set.
const (
	memTotalUsedBit     uint8 = 1 << 0
	memTotalFreeBit     uint8 = 1 << 1
	memTotalReservedBit uint8 = 1 << 2
	memTotalAllPresent        = memTotalUsedBit | memTotalFreeBit | memTotalReservedBit
)

// memTotalNilCount is an internal counter incremented every time
// QueryGPUMetrics finalizes a pod whose MemTotal is dropped because not
// all three FB_* series were present in the scrape result. Surfaces
// DCGM exporter version skew / CSV-misconfig to operator telemetry.
// Read via MemTotalNilCount() for tests and rack-team observability.
var memTotalNilCount int64

// MemTotalNilCount returns the current value of the internal
// memTotalNilCount counter. Tests and rack-team observability surfaces
// (e.g. provider/k8s/telemetry.go event stream) consult this to detect
// DCGM exporter version skew / partial-scrape conditions. Reset only
// via tests with ResetMemTotalNilCount.
func MemTotalNilCount() int64 {
	return atomic.LoadInt64(&memTotalNilCount)
}

// ResetMemTotalNilCount zeroes the internal counter. Test-only helper.
func ResetMemTotalNilCount() {
	atomic.StoreInt64(&memTotalNilCount, 0)
}

// PrometheusClient is a thin wrapper around the v1 query API. It is safe
// to be nil — every public method short-circuits to a zero-value /
// no-error return when the receiver is nil. This mirrors the
// MetricScraperClient nil-tolerance pattern in metric_scraper.go and
// matches the fail-soft posture of MetricsClient at process.go:155-160.
type PrometheusClient struct {
	host string
	api  promv1.API
}

// NewPrometheusClient returns nil when host is empty — callers must
// nil-check the result. A bad URL returns (nil, err); the caller logs
// and treats nil as "no client".
func NewPrometheusClient(host string) (*PrometheusClient, error) {
	if host == "" {
		return nil, nil
	}
	c, err := promapi.NewClient(promapi.Config{Address: host})
	if err != nil {
		return nil, err
	}
	return &PrometheusClient{host: host, api: promv1.NewAPI(c)}, nil
}

// QueryGPUMetrics issues ONE instant query per metric across all pods
// in the given app+service set, parses the resulting model.Vector, and
// returns a pod-keyed map. The pod name comes from the DCGM exporter's
// "pod" label (set when kubernetes.enablePodLabels=true in the chart
// values — see terraform/cluster/aws/dcgm.tf).
//
// Returns (empty-map, nil) when no samples are found — distinguishes
// "Prometheus reachable, no data yet" from a transport error.
//
// Total queries per call: 10 (one per metric — util, fb-used, fb-free,
// fb-reserved, tensor-active, sm-active, dram-active, fp16-active,
// fp32-active, power-usage). Caller pays one Prom
// round-trip per metric, NOT one per pod. Lower latency, lower Prom
// load. See MG-4 OQ-5.
//
// Per-metric per-pod presence: each setter allocates a fresh pointer
// per call and stores it on the GpuMetrics struct. Pods that did NOT
// report a sample for a given metric end up with a nil pointer for
// that field. enrichGpuTelemetry must skip nil pointers per metric so
// the per-metric average isn't pulled toward zero by absent samples.
//
// Memory total derivation: DCGM does NOT emit DCGM_FI_DEV_FB_TOTAL on
// the default-counters.csv. Total is derived as USED + FREE + RESERVED.
// If any of the three is missing for a pod, MemTotal is set to nil and
// memTotalNilCount is incremented (operator telemetry hook).
func (pc *PrometheusClient) QueryGPUMetrics(ctx context.Context, app string, services []string) (map[string]GpuMetrics, error) {
	if pc == nil {
		return nil, nil
	}

	out := map[string]GpuMetrics{}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Build app + service filter from the input. The DCGM exporter in
	// kubernetes mode (configured via gpu_observability_enable=true) emits
	// the K8s pod labels DIRECTLY as Prometheus labels — plain `app=`,
	// `service=`, `pod=`, `namespace=` — with no `label_` prefix. The earlier
	// claim that `enablePodLabels=true` adds `label_app`/`label_service` was
	// wrong: that prefix is a kube-state-metrics convention, not a DCGM one.
	// Verified against DCGM chart 4.8.1 emitting:
	//   DCGM_FI_DEV_GPU_UTIL{app="...", service="...", pod="...", ...}
	filter := fmt.Sprintf(`app=%q`, app)
	if len(services) > 0 {
		// services -> regex alternation
		alt := ""
		for i, s := range services {
			if i > 0 {
				alt += "|"
			}
			alt += s
		}
		filter += fmt.Sprintf(`,service=~%q`, alt)
	}

	// Issue one query per metric. Per-metric setter allocates a fresh
	// pointer and assigns it on the per-pod accumulator — required to
	// avoid loop-variable capture. Memory metrics convert MiB→bytes; util
	// passes through as percent.
	//
	// Total framebuffer is DERIVED rather than scraped because the DCGM
	// exporter's default counters file (/etc/dcgm-exporter/default-counters.csv)
	// does NOT emit DCGM_FI_DEV_FB_TOTAL — only FB_USED, FB_FREE, FB_RESERVED.
	// Querying FB_TOTAL on the default config returns empty Vector. The
	// derivation accumulates per-pod into memTotalRaw + memTotalParts; the
	// finalize loop below promotes those into MemTotal only when all three
	// bits are set, otherwise sets MemTotal=nil and increments
	// memTotalNilCount.
	queries := []struct {
		metric string
		set    func(*GpuMetrics, float64)
	}{
		{"DCGM_FI_DEV_GPU_UTIL", func(g *GpuMetrics, v float64) {
			vCopy := v
			g.Util = &vCopy
		}},
		{"DCGM_FI_DEV_FB_USED", func(g *GpuMetrics, v float64) {
			b := int64(v) * 1024 * 1024
			vCopy := b
			g.MemUsed = &vCopy
			g.memTotalRaw += b
			g.memTotalParts |= memTotalUsedBit
		}},
		{"DCGM_FI_DEV_FB_FREE", func(g *GpuMetrics, v float64) {
			g.memTotalRaw += int64(v) * 1024 * 1024
			g.memTotalParts |= memTotalFreeBit
		}},
		{"DCGM_FI_DEV_FB_RESERVED", func(g *GpuMetrics, v float64) {
			g.memTotalRaw += int64(v) * 1024 * 1024
			g.memTotalParts |= memTotalReservedBit
		}},
		// Extended profiling counters. DCGM emits these as ratios in [0,1]
		// for *_ACTIVE metrics; we multiply by 100 here to align with the
		// percent convention already used for GPU_UTIL across the wire.
		// PowerW passes through.
		{"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", func(g *GpuMetrics, v float64) {
			vCopy := v * 100
			g.TensorActive = &vCopy
		}},
		{"DCGM_FI_PROF_SM_ACTIVE", func(g *GpuMetrics, v float64) {
			vCopy := v * 100
			g.SmActive = &vCopy
		}},
		{"DCGM_FI_PROF_DRAM_ACTIVE", func(g *GpuMetrics, v float64) {
			vCopy := v * 100
			g.DramActive = &vCopy
		}},
		{"DCGM_FI_PROF_PIPE_FP16_ACTIVE", func(g *GpuMetrics, v float64) {
			vCopy := v * 100
			g.Fp16Active = &vCopy
		}},
		{"DCGM_FI_PROF_PIPE_FP32_ACTIVE", func(g *GpuMetrics, v float64) {
			vCopy := v * 100
			g.Fp32Active = &vCopy
		}},
		{"DCGM_FI_DEV_POWER_USAGE", func(g *GpuMetrics, v float64) {
			vCopy := v
			g.PowerW = &vCopy
		}},
	}

	for _, q := range queries {
		expr := fmt.Sprintf(`%s{%s}`, q.metric, filter)
		val, _, err := pc.api.Query(ctx, expr, time.Now())
		if err != nil {
			return out, err
		}
		vec, ok := val.(model.Vector)
		if !ok {
			continue
		}
		for _, sample := range vec {
			pod := string(sample.Metric[model.LabelName("pod")])
			if pod == "" {
				continue
			}
			gm := out[pod]
			q.set(&gm, float64(sample.Value))
			// Always populate Service from the DCGM `service` label (the
			// exporter mirrors K8s pod labels directly — plain key, no
			// `label_` prefix). Empty when the pod has no `service` K8s
			// label, which is the case for non-Convox-managed pods scraped
			// through the same Prom job.
			if gm.Service == "" {
				gm.Service = string(sample.Metric[model.LabelName("service")])
			}
			out[pod] = gm
		}
	}

	// Finalize MemTotal per pod: only promote the running sum into the
	// MemTotal pointer when all three FB_* parts were observed. Otherwise
	// MemTotal stays nil and memTotalNilCount increments — operator
	// telemetry surfaces DCGM CSV/version skew via that counter.
	for pod, gm := range out {
		if gm.memTotalParts == memTotalAllPresent {
			total := gm.memTotalRaw
			gm.MemTotal = &total
		} else if gm.memTotalParts != 0 {
			// At least one FB_* sample arrived but not the full set —
			// drop MemTotal so UI renders "—" rather than an
			// understated value. memTotalParts==0 is "no FB_* at all"
			// which is just the no-data path; only count the partial
			// case as version skew.
			atomic.AddInt64(&memTotalNilCount, 1)
		}
		out[pod] = gm
	}

	return out, nil
}

// promCircuitBreaker tracks consecutive Prometheus QueryRange timeouts.
// On 3 timeouts within 60s the breaker opens for 30s — subsequent
// QueryGPURange calls return ErrPromCircuitOpen without hitting Prom.
// Package-level so tests can override via setPromCircuitBreakerForTest.
//
// Shared by ServiceMetrics and MetricsByService handlers — a single
// flapping Prom backend short-circuits both endpoints' calls so the
// rack doesn't pile pending QueryRange goroutines while waiting for an
// unreachable backend.
var promCircuitBreaker = newPromBreaker(3, 60*time.Second, 30*time.Second)

// ErrPromCircuitOpen is returned by QueryGPURange when the circuit
// breaker is open. Callers (controllers / impls) translate this into
// an empty []Metric / []ServiceMetricsRow with a "prometheusTimedOut"
// flag at the resolver layer per CLUSTER-1 1E.
var ErrPromCircuitOpen = fmt.Errorf("prometheus circuit breaker open: backend unreachable")

type promBreaker struct {
	mu          sync.Mutex
	trips       int
	tripWindow  time.Duration
	cooldown    time.Duration
	threshold   int
	lastFailure time.Time
	openedAt    time.Time
	nowFn       func() time.Time
}

func newPromBreaker(threshold int, window, cooldown time.Duration) *promBreaker {
	return &promBreaker{
		threshold:  threshold,
		tripWindow: window,
		cooldown:   cooldown,
		nowFn:      time.Now,
	}
}

// Allowed reports whether a query may proceed. When the breaker is open
// (opened-At + cooldown > now) it returns false. Any caller observing a
// false return should fail fast with ErrPromCircuitOpen so the request
// path can degrade gracefully.
func (b *promBreaker) Allowed() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.openedAt.IsZero() && b.nowFn().Before(b.openedAt.Add(b.cooldown)) {
		return false
	}
	// Cooldown elapsed — clear so the breaker re-arms; trip counter resets
	// on the next successful or failed call's Record/Reset.
	if !b.openedAt.IsZero() && !b.nowFn().Before(b.openedAt.Add(b.cooldown)) {
		b.openedAt = time.Time{}
		b.trips = 0
	}
	return true
}

// RecordFailure increments the trip counter; opens the breaker once the
// threshold of consecutive failures is reached within the trip window.
// A failure outside the trip window resets the counter to 1.
func (b *promBreaker) RecordFailure() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.nowFn()
	if !b.lastFailure.IsZero() && now.Sub(b.lastFailure) > b.tripWindow {
		b.trips = 0
	}
	b.trips++
	b.lastFailure = now
	if b.trips >= b.threshold {
		b.openedAt = now
	}
}

// RecordSuccess resets the trip counter — a single successful call
// closes any half-open state. Called by QueryGPURange on a non-error
// Prom response.
func (b *promBreaker) RecordSuccess() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trips = 0
	b.lastFailure = time.Time{}
	b.openedAt = time.Time{}
}

// Reset clears all breaker state — test helper only.
func (b *promBreaker) Reset() {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trips = 0
	b.lastFailure = time.Time{}
	b.openedAt = time.Time{}
}

// ValidatePrometheusURL re-validates a stored prometheus_url against the
// SSRF allowlist defined in pkg/cli/rack.go:1857-1885 (param-validation
// layer). On rack startup the provider re-applies these checks to the
// stored value so a hostile pre-F-01 storage write OR a manually-edited
// configmap can't slip past. Returns nil on accept; an error otherwise.
//
// Keep this in sync with the param-set validator. F-SEC-26.
func ValidatePrometheusURL(raw string) error {
	if raw == "" {
		return nil // empty is allowed → PromClient stays nil
	}
	parsed, err := url.Parse(raw)
	scheme := ""
	if parsed != nil {
		scheme = strings.ToLower(parsed.Scheme)
	}
	if err != nil || scheme == "" || (scheme != "http" && scheme != "https") {
		return fmt.Errorf("prometheus_url: only http:// and https:// schemes are accepted")
	}
	if parsed.Host == "" {
		return fmt.Errorf("prometheus_url: must be a valid URL with scheme and host (e.g. http://prom.example.com:9090)")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("prometheus_url: private/loopback/link-local hosts are not allowed (see docs for SSRF protection)")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("prometheus_url: private/loopback/link-local hosts are not allowed (see docs for SSRF protection)")
		}
	}
	return nil
}

// gpuMetricSpec carries the PromQL metric name and the response-side
// metric name (the value of `Metric.Name` on the wire — the rack-side
// projection from DCGM idempotent counter to a stable telemetry vocab).
// fp16/fp32/sm/dram/tensor active counters are scaled ×100 to match the
// percent convention used elsewhere in the wire (see QueryGPUMetrics
// pre-existing scaling — we mirror the same per-metric transform here).
type gpuMetricSpec struct {
	prom    string  // PromQL metric name
	wire    string  // [Metric.Name] on the wire
	scale   float64 // multiplier (1.0 default; 100 for *_ACTIVE counters; (1<<20) for FB_*)
}

// gpuRangeQueries lists the metrics emitted by QueryGPURange in stable
// wire-name order. cpu/memory rows are NOT included here — they are
// container-level metrics derived from kubelet/cAdvisor (out of scope
// for the GPU range path; per-service CPU/memory is handled by V2's
// existing AppMetrics pattern, which V3 does not yet implement).
//
// fp16/fp32/sm/dram/tensor scale by 100 to match the percent convention
// already established by QueryGPUMetrics for the *_ACTIVE counters.
// FB_USED scales by 1<<20 (MiB→bytes) for parity with QueryGPUMetrics.
var gpuRangeQueries = []gpuMetricSpec{
	{prom: "DCGM_FI_DEV_GPU_UTIL", wire: "gpu-util", scale: 1.0},
	{prom: "DCGM_FI_DEV_FB_USED", wire: "gpu-mem-used", scale: float64(int64(1) << 20)},
	{prom: "DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", wire: "gpu-tensor-active", scale: 100.0},
	{prom: "DCGM_FI_PROF_SM_ACTIVE", wire: "gpu-sm-active", scale: 100.0},
	{prom: "DCGM_FI_PROF_DRAM_ACTIVE", wire: "gpu-dram-active", scale: 100.0},
	{prom: "DCGM_FI_PROF_PIPE_FP16_ACTIVE", wire: "gpu-fp16-active", scale: 100.0},
	{prom: "DCGM_FI_PROF_PIPE_FP32_ACTIVE", wire: "gpu-fp32-active", scale: 100.0},
	{prom: "DCGM_FI_DEV_POWER_USAGE", wire: "gpu-power-w", scale: 1.0},
}

// GpuRangeWireNames returns the ordered list of wire names emitted by
// QueryGPURange — exposed for tests asserting the wire vocabulary
// matches what the resolver / Vue chart expect.
func GpuRangeWireNames() []string {
	n := make([]string, 0, len(gpuRangeQueries))
	for _, s := range gpuRangeQueries {
		n = append(n, s.wire)
	}
	return n
}

// QueryGPURange issues one Prom QueryRange per GPU metric over a
// regex-alternation service filter, producing per-metric per-service
// time-series. Returns one structs.Metric per requested wire-name
// regardless of whether the underlying Prom query returned data — empty
// values are preserved so the caller can package one row per requested
// metric / service.
//
// `services` may be empty (matches all services for app); when non-empty
// it MUST be pre-sanitised by the caller (NameValidator) so the regex
// alternation can't be jail-broken via meta-chars.
//
// The breaker check is at-the-edge: a closed breaker proceeds; an open
// breaker fails fast with ErrPromCircuitOpen. On per-call timeout we
// record the failure; on full success we record success. Partial
// success (some metrics return, some err) does not record success — we
// surface the first error to the caller and let the breaker count the
// failure.
//
// Result shape: returns map[wireName]map[service][]MetricValue. Caller
// (impl in service.go / app.go) projects this into structs.Metric or
// structs.ServiceMetricsRow rows. Each MetricValue carries Time and
// Average — Min/Max/Sum/Count are zero for range queries (Prom range
// query returns one sample per timestamp; min/max are aggregations
// over multiple samples that we don't run here).
func (pc *PrometheusClient) QueryGPURange(ctx context.Context, app string, services []string, opts structs.MetricsOptions) (map[string]map[string][]structs.MetricValue, error) {
	if pc == nil {
		return nil, nil
	}
	if !promCircuitBreaker.Allowed() {
		return nil, ErrPromCircuitOpen
	}

	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	step := 30 * time.Second
	if opts.Start != nil {
		start = *opts.Start
	}
	if opts.End != nil {
		end = *opts.End
	}
	if opts.Period != nil && *opts.Period > 0 {
		step = time.Duration(*opts.Period) * time.Second
	}

	range_ := promv1.Range{Start: start, End: end, Step: step}

	filter := fmt.Sprintf(`app=%q`, app)
	if len(services) > 0 {
		alt := ""
		for i, s := range services {
			if i > 0 {
				alt += "|"
			}
			alt += s
		}
		filter += fmt.Sprintf(`,service=~%q`, alt)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out := map[string]map[string][]structs.MetricValue{}
	hadAnySuccess := false

	for _, spec := range gpuRangeQueries {
		spec := spec
		expr := fmt.Sprintf(`%s{%s}`, spec.prom, filter)
		val, _, err := pc.api.QueryRange(ctx, expr, range_)
		if err != nil {
			promCircuitBreaker.RecordFailure()
			return nil, err
		}
		matrix, ok := val.(model.Matrix)
		if !ok {
			// Empty or unexpected type → empty bucket for this metric. Don't
			// flag it as a failure (Prom returned 200, just no data).
			out[spec.wire] = map[string][]structs.MetricValue{}
			hadAnySuccess = true
			continue
		}
		byService := map[string][]structs.MetricValue{}
		for _, series := range matrix {
			service := string(series.Metric[model.LabelName("service")])
			values := make([]structs.MetricValue, 0, len(series.Values))
			for _, sp := range series.Values {
				v := float64(sp.Value) * spec.scale
				values = append(values, structs.MetricValue{
					Time:    sp.Timestamp.Time(),
					Average: v,
					Minimum: v,
					Maximum: v,
				})
			}
			byService[service] = append(byService[service], values...)
		}
		out[spec.wire] = byService
		hadAnySuccess = true
	}

	if hadAnySuccess {
		promCircuitBreaker.RecordSuccess()
	}

	return out, nil
}
