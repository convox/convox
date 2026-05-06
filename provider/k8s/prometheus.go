package k8s

import (
	"context"
	"fmt"
	"time"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// GpuMetrics is the per-pod sample bundle returned by QueryGPUMetrics.
// Zero values on a key indicate "metric absent in the response Vector"
// (e.g. DCGM exporter is not yet emitting that field).
//
// Service carries the value of the DCGM `service` Prometheus label —
// the DCGM exporter in kubernetes mode mirrors the pod's K8s `service`
// label directly (plain key, no `label_` prefix). Used by service.go
// aggregation to bucket pods by service without a second Prom round-trip.
type GpuMetrics struct {
	Util     float64 // percent 0-100
	MemUsed  int64   // bytes (FB used)
	MemTotal int64   // bytes (FB total)
	Service  string  // value of Prom `service` label (pod's `service` K8s label)

	// Extended DCGM profiling counters. All optional — zero values mean the
	// dcgm-exporter chart isn't yet emitting that field (older default-
	// counters.csv) OR the GPU type doesn't support the metric (e.g. FP16
	// active on H100 where DCGM may not expose tensor-pipe FP16). Vue side
	// renders "no data" empty state when the corresponding pointer is nil
	// after the resolver decode.
	TensorActive float64 // percent 0-100 (DCGM_FI_PROF_PIPE_TENSOR_ACTIVE × 100)
	SmActive     float64 // percent 0-100 (DCGM_FI_PROF_SM_ACTIVE × 100)
	DramActive   float64 // percent 0-100 (DCGM_FI_PROF_DRAM_ACTIVE × 100)
	Fp16Active   float64 // active fraction 0-1 (DCGM_FI_PROF_PIPE_FP16_ACTIVE)
	Fp32Active   float64 // active fraction 0-1 (DCGM_FI_PROF_PIPE_FP32_ACTIVE)
	PowerW       float64 // watts (DCGM_FI_DEV_POWER_USAGE)
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
// Total queries per call: 3 (one per metric). Caller pays one Prom
// round-trip per metric, NOT one per pod. Lower latency, lower Prom
// load. See MG-4 OQ-5.
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

	// Issue one query per metric. Per-metric setter writes the parsed
	// SampleValue into the correct field on the per-pod accumulator.
	// Memory metrics convert MiB→bytes; util passes through as percent.
	//
	// Total framebuffer is DERIVED rather than scraped because the DCGM
	// exporter's default counters file (/etc/dcgm-exporter/default-counters.csv)
	// does NOT emit DCGM_FI_DEV_FB_TOTAL — only FB_USED, FB_FREE, FB_RESERVED.
	// Querying FB_TOTAL on the default config returns empty Vector, leaving
	// MemTotal=0 and the dashboard showing "<used> / 0 B". Sum of the three
	// emitted fields equals card capacity (FB_USED + FB_FREE + FB_RESERVED ==
	// total VRAM); we accumulate them into MemTotal in setter form so the
	// caller still gets a single int64 bytes total.
	queries := []struct {
		metric string
		set    func(*GpuMetrics, float64)
	}{
		{"DCGM_FI_DEV_GPU_UTIL", func(g *GpuMetrics, v float64) { g.Util = v }},
		{"DCGM_FI_DEV_FB_USED", func(g *GpuMetrics, v float64) { g.MemUsed = int64(v) * 1024 * 1024; g.MemTotal += int64(v) * 1024 * 1024 }},
		{"DCGM_FI_DEV_FB_FREE", func(g *GpuMetrics, v float64) { g.MemTotal += int64(v) * 1024 * 1024 }},
		{"DCGM_FI_DEV_FB_RESERVED", func(g *GpuMetrics, v float64) { g.MemTotal += int64(v) * 1024 * 1024 }},
		// Extended profiling counters. DCGM emits these as ratios in [0,1]
		// for *_ACTIVE metrics; we multiply by 100 here to align with the
		// percent convention already used for GPU_UTIL. PowerW passes through.
		{"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE", func(g *GpuMetrics, v float64) { g.TensorActive = v * 100 }},
		{"DCGM_FI_PROF_SM_ACTIVE", func(g *GpuMetrics, v float64) { g.SmActive = v * 100 }},
		{"DCGM_FI_PROF_DRAM_ACTIVE", func(g *GpuMetrics, v float64) { g.DramActive = v * 100 }},
		{"DCGM_FI_PROF_PIPE_FP16_ACTIVE", func(g *GpuMetrics, v float64) { g.Fp16Active = v }},
		{"DCGM_FI_PROF_PIPE_FP32_ACTIVE", func(g *GpuMetrics, v float64) { g.Fp32Active = v }},
		{"DCGM_FI_DEV_POWER_USAGE", func(g *GpuMetrics, v float64) { g.PowerW = v }},
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

	return out, nil
}
