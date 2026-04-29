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
// Service carries the value of the DCGM `label_service` Prometheus label
// (set when the chart's kubernetes.enablePodLabels=true mirrors the K8s
// pod label `service` onto every metric). Used by service.go aggregation
// to bucket pods by service without a second Prom round-trip.
type GpuMetrics struct {
	Util     float64 // percent 0-100
	MemUsed  int64   // bytes (FB used)
	MemTotal int64   // bytes (FB total)
	Service  string  // value of Prom `label_service` (pod's `service` K8s label)
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

	// Build label_app + label_service filter from the input. The DCGM
	// chart's enablePodLabels=true emits "label_app" and "label_service"
	// metric labels mirroring the K8s pod labels.
	filter := fmt.Sprintf(`label_app="%s"`, app)
	if len(services) > 0 {
		// services -> regex alternation
		alt := ""
		for i, s := range services {
			if i > 0 {
				alt += "|"
			}
			alt += s
		}
		filter += fmt.Sprintf(`,label_service=~"%s"`, alt)
	}

	// Issue one query per metric. Per-metric setter writes the parsed
	// SampleValue into the correct field on the per-pod accumulator.
	// Memory metrics convert MiB→bytes; util passes through as percent.
	queries := []struct {
		metric string
		set    func(*GpuMetrics, float64)
	}{
		{"DCGM_FI_DEV_GPU_UTIL", func(g *GpuMetrics, v float64) { g.Util = v }},
		{"DCGM_FI_DEV_FB_USED", func(g *GpuMetrics, v float64) { g.MemUsed = int64(v) * 1024 * 1024 }},
		{"DCGM_FI_DEV_FB_TOTAL", func(g *GpuMetrics, v float64) { g.MemTotal = int64(v) * 1024 * 1024 }},
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
			// Always populate Service from the DCGM `label_service` mirror
			// (chart sets enablePodLabels=true). Empty when the pod has no
			// `service` K8s label, which is the case for non-Convox-managed
			// pods scraped through the same Prom job.
			if gm.Service == "" {
				gm.Service = string(sample.Metric[model.LabelName("label_service")])
			}
			out[pod] = gm
		}
	}

	return out, nil
}
