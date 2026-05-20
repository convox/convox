package k8s

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/validator"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Nil pointer = metric absent for this pod (DCGM version skew).
type GpuMetrics struct {
	Util     *float64
	MemUsed  *int64
	MemTotal *int64 // derived: USED+FREE+RESERVED; nil unless all three present
	Service  string

	TensorActive *float64
	SmActive     *float64
	DramActive   *float64
	Fp16Active   *float64
	Fp32Active   *float64
	PowerW       *float64

	memTotalParts uint8
	memTotalRaw   int64
}

const (
	memTotalUsedBit     uint8 = 1 << 0
	memTotalFreeBit     uint8 = 1 << 1
	memTotalReservedBit uint8 = 1 << 2
	memTotalAllPresent        = memTotalUsedBit | memTotalFreeBit | memTotalReservedBit
)

var memTotalNilCount int64

func MemTotalNilCount() int64 {
	return atomic.LoadInt64(&memTotalNilCount)
}

func ResetMemTotalNilCount() {
	atomic.StoreInt64(&memTotalNilCount, 0)
}

type PrometheusClient struct {
	host string
	api  promv1.API
}

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

func (pc *PrometheusClient) QueryGPUMetrics(ctx context.Context, app string, services []string) (map[string]GpuMetrics, error) {
	if pc == nil {
		return nil, nil
	}

	out := map[string]GpuMetrics{}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// DCGM exporter emits K8s pod labels as plain Prom labels (no label_ prefix).
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

	// FB_TOTAL not in default DCGM counters; derive from USED+FREE+RESERVED.
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
		// *_ACTIVE counters: DCGM [0,1] ratio -> percent.
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
			if gm.Service == "" {
				gm.Service = string(sample.Metric[model.LabelName("service")])
			}
			out[pod] = gm
		}
	}

	for pod, gm := range out {
		if gm.memTotalParts == memTotalAllPresent {
			total := gm.memTotalRaw
			gm.MemTotal = &total
		} else if gm.memTotalParts != 0 {
			// Partial FB_* set — drop MemTotal to avoid understating capacity.
			atomic.AddInt64(&memTotalNilCount, 1)
		}
		out[pod] = gm
	}

	return out, nil
}

// 3 timeouts in 60s -> open for 30s.
var promCircuitBreaker = newPromBreaker(3, 60*time.Second, 30*time.Second)

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

func (b *promBreaker) Allowed() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.openedAt.IsZero() && b.nowFn().Before(b.openedAt.Add(b.cooldown)) {
		return false
	}
	if !b.openedAt.IsZero() && !b.nowFn().Before(b.openedAt.Add(b.cooldown)) {
		b.openedAt = time.Time{}
		b.trips = 0
	}
	return true
}

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

func ValidatePrometheusURL(raw string) error {
	if raw == "" {
		return nil
	}
	if err := validator.ValidateExternalURL(raw, nil); err != nil {
		return fmt.Errorf("prometheus_url: %s", err)
	}
	return nil
}

type gpuMetricSpec struct {
	prom  string
	wire  string
	scale float64
}

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

func GpuRangeWireNames() []string {
	n := make([]string, 0, len(gpuRangeQueries))
	for _, s := range gpuRangeQueries {
		n = append(n, s.wire)
	}
	return n
}

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
