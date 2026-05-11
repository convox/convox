package structs

import "fmt"

type Service struct {
	Count     int                    `json:"count"`
	Cpu       int                    `json:"cpu"`
	Domain    string                 `json:"domain"`
	Gpu       int                    `json:"gpu"`
	GpuVendor string                 `json:"gpu-vendor"`
	Memory    int                    `json:"memory"`
	Name      string                 `json:"name"`
	Nlb       []ServiceNlbPort       `json:"nlb"`
	Ports     []ServicePort          `json:"ports"`
	Min       *int                   `json:"min,omitempty"`
	Max       *int                   `json:"max,omitempty"`
	ColdStart *bool                  `json:"cold-start,omitempty"`
	Autoscale *ServiceAutoscaleState `json:"autoscale,omitempty"`

	// Agent reports whether the service runs as a per-node agent (DaemonSet
	// in K8s providers; svc.agent: true in convox.yml). Surfaces to Console
	// so per-service UI affordances that don't apply to agents (e.g. the
	// scale-override toggle) can be hidden client-side. Zero value
	// preserves existing wire shape on pre-3.24.6 racks (decoded as false;
	// the Console fallback shows the affordance). 3.24.6+ racks populate
	// from the manifest service definition.
	Agent bool `json:"agent,omitempty"`

	// GPU runtime telemetry — per-metric mean across pods in the service at
	// the most recent scrape. Populated by provider/k8s/prometheus.go via
	// a single batched Prom query per ServiceList call. Pointer-typed so:
	//   nil          → "no pod reported this metric" (mixed-skew with
	//                   pre-3.24.6 rack, prom unreachable, no GPU service,
	//                   or DCGM exporter version skew leaving this metric
	//                   absent across every pod in the service);
	//   non-nil zero → "real averaged reading at idle";
	//   non-nil > 0  → "real averaged reading under load."
	// See Process struct doc for the full state-disambiguation rationale.
	//
	// Naming note: dropped the `_avg` suffix in 3.24.6 (the field name read
	// as a temporal average; reality is spatial across pods at one instant).
	// 3.24.6 RC builds are the only pre-public exposure; no external
	// consumer existed yet, so the rename is free in this window.
	GpuUtil     *float64 `json:"gpu-util,omitempty"`
	GpuMemUsed  *int64   `json:"gpu-mem-used,omitempty"`
	GpuMemTotal *int64   `json:"gpu-mem-total,omitempty"`

	// Extended GPU runtime telemetry — DCGM profiling counters averaged across
	// pods in the service. Same pointer-tri-state semantics as GpuUtil
	// above. Populated by the same batched Prom query in
	// provider/k8s/service.go alongside GpuUtil/GpuMemUsed/GpuMemTotal.
	//
	//   GpuTensorActive → DCGM_FI_PROF_PIPE_TENSOR_ACTIVE × 100 (percent)
	//   GpuSmActive     → DCGM_FI_PROF_SM_ACTIVE × 100 (percent)
	//   GpuDramActive   → DCGM_FI_PROF_DRAM_ACTIVE × 100 (percent)
	//   GpuFp16Active   → DCGM_FI_PROF_PIPE_FP16_ACTIVE × 100 (percent)
	//   GpuFp32Active   → DCGM_FI_PROF_PIPE_FP32_ACTIVE × 100 (percent)
	//   GpuPowerW       → DCGM_FI_DEV_POWER_USAGE (watts)
	GpuTensorActive *float64 `json:"gpu-tensor-active,omitempty"`
	GpuSmActive     *float64 `json:"gpu-sm-active,omitempty"`
	GpuDramActive   *float64 `json:"gpu-dram-active,omitempty"`
	GpuFp16Active   *float64 `json:"gpu-fp16-active,omitempty"`
	GpuFp32Active   *float64 `json:"gpu-fp32-active,omitempty"`
	GpuPowerW       *float64 `json:"gpu-power-w,omitempty"`

	// ScaleOverrideActive reflects whether the service Deployment has
	// the convox.com/scale-override-active=true annotation set.
	// Populated by ServiceList/ServiceGet from the per-Deployment
	// metadata read path. When *true, future ReleasePromote calls
	// preserve the runtime replica count and skip yaml-declared
	// scale.count.min.
	//
	// Pointer-nullable per cross-version-skew reasoning: three distinct
	// states must be disambiguated on the wire so the Console toggle
	// UI can graceful-degrade against pre-3.24.6 racks:
	//   nil    = "rack does not support scale-override" (pre-3.24.6
	//            rack returned the Service struct without this field;
	//            Go decode leaves the pointer nil). Vue side reads
	//            null and disables the toggle with version-gated
	//            tooltip.
	//   *false = "rack supports it; override is currently OFF". Vue
	//            renders the OFF-state hint correctly.
	//   *true  = "rack supports it; override is currently ON". Vue
	//            renders the ON-state banner.
	//
	// omitempty strips when nil so old SDK clients (3.24.5 and
	// earlier) that decode the Service payload see no extra field
	// (additive, BC-safe). New SDK callers populate via the
	// ServiceList / ServiceGet enrichment path.
	//
	// 3.24.6+ rack code MUST always populate this pointer (never
	// leave nil). The nil value is reserved as the wire-signal for
	// pre-3.24.6 rack ("feature not supported").
	ScaleOverrideActive *bool `json:"scale-override-active,omitempty"`

	// TriggersOverrideActive reflects whether the service Deployment
	// carries the convox.com/triggers-override-active=true annotation,
	// meaning the autoscaler was configured through the Console rather
	// than the manifest. Pointer-nullable on the same three-state wire
	// contract as ScaleOverrideActive — nil signals a pre-3.24.6 rack
	// (Console disables the affordance with a version-gated tooltip);
	// *false signals "rack supports it, override is OFF"; *true signals
	// "override is ON, autoscaler driven by Console writes through
	// service_triggers_enable/disable/threshold_set."
	//
	// 3.24.6+ rack code ALWAYS populates this pointer (never nil) so
	// the Console version-gate signal stays unambiguous.
	TriggersOverrideActive *bool `json:"triggers-override-active,omitempty"`
}

// ServiceAutoscaleState is the wire shape returned by the rack for the
// "autoscale" portion of a service description. It mirrors the user-supplied
// `scale.autoscale` block from convox.yml but reports CURRENT state rather
// than configured intent — Enabled reflects whether KEDA ScaledObjects are
// in place, and the *Threshold pointers are nil when the matching trigger
// is unconfigured.
type ServiceAutoscaleState struct {
	Enabled        bool   `json:"enabled"`
	CpuThreshold   *int   `json:"cpu-threshold,omitempty"`
	MemThreshold   *int   `json:"mem-threshold,omitempty"`
	GpuThreshold   *int   `json:"gpu-threshold,omitempty"`
	QueueThreshold *int   `json:"queue-threshold,omitempty"`
	MetricName     string `json:"metric-name,omitempty"`
	CustomTriggers int    `json:"custom-triggers,omitempty"`
}

type Services []Service

// ServiceNlbPort corresponds to manifest.ServiceNLBPort in the v2 repo. Naming
// diverges intentionally to match the pkg/structs casing convention (Cpu, Gpu,
// Nlb) rather than the manifest package's all-caps initialism style. v3 has no
// NLB manifest schema today; this wire shape exists so v3 CLI (and Console,
// which vendors this package) can surface NLB info returned by v2 racks.
//
// CrossZone and PreserveClientIP are pointers so nil signals "no per-port
// override — inherit the NLB-level setting," distinct from a non-nil false
// which is a meaningful explicit override. AllowCIDR omitempty + len-based
// display gating means nil slice, JSON null, and empty slice all render as
// no-bracket equivalently.
type ServiceNlbPort struct {
	ContainerPort    int      `json:"container-port"`
	Port             int      `json:"port"`
	Protocol         string   `json:"protocol"`
	Scheme           string   `json:"scheme"`
	Certificate      string   `json:"certificate"`
	CrossZone        *bool    `json:"cross-zone,omitempty"`
	AllowCIDR        []string `json:"allow-cidr,omitempty"`
	PreserveClientIP *bool    `json:"preserve-client-ip,omitempty"`
}

type ServicePort struct {
	Balancer    int    `json:"balancer"`
	Certificate string `json:"certificate"`
	Container   int    `json:"container"`
}

type ServiceUpdateOptions struct {
	Count     *int    `flag:"count" param:"count"`
	Cpu       *int    `flag:"cpu" param:"cpu"`
	Gpu       *int    `flag:"gpu" param:"gpu"`
	GpuVendor *string `flag:"gpu-vendor" param:"gpu-vendor"`
	Memory    *int    `flag:"memory" param:"memory"`
	Min       *int    `flag:"min" param:"min"`
	Max       *int    `flag:"max" param:"max"`
}

// Canonical wire-form trigger types shared between rack handler, SDK,
// GraphQL enum, and CLI. Persisted in the KEDA ScaledObject trigger
// .name field as `convox-<type>` so reads can disambiguate Console-driven
// triggers from manifest-driven ones.
const (
	TriggerTypeCPU            = "cpu"
	TriggerTypeMemory         = "memory"
	TriggerTypeGPUUtilization = "gpuUtilization"
	TriggerTypeQueueDepth     = "queueDepth"
)

// TriggerSpec describes a single autoscale trigger requested by the
// Console-driven triggers override surface.
type TriggerSpec struct {
	Type      string  `json:"type"`
	Threshold float64 `json:"threshold"`
}

func (t TriggerSpec) Validate() error {
	switch t.Type {
	case TriggerTypeCPU, TriggerTypeMemory, TriggerTypeGPUUtilization, TriggerTypeQueueDepth:
	default:
		return fmt.Errorf("unknown trigger type %q", t.Type)
	}
	if t.Threshold <= 0 {
		return fmt.Errorf("threshold must be positive")
	}
	// queueDepth is an absolute count (>=1), not a percent; CPU/Memory/GPU
	// utilization are all percentages and capped at 100.
	if t.Type != TriggerTypeQueueDepth && t.Threshold > 100 {
		return fmt.Errorf("percent triggers must be <= 100")
	}
	return nil
}

// ServiceTriggersOptions carries the inputs for ServiceTriggersEnable.
// The caller supplies desired bounds + trigger set; the rack handler
// validates, runs KEDA + GPU preflight, and materializes the matching CRD
// (native HPA when all triggers are cpu/memory; KEDA ScaledObject when any
// trigger is gpuUtilization or queueDepth).
type ServiceTriggersOptions struct {
	Min      int           `json:"min"`
	Max      int           `json:"max"`
	Triggers []TriggerSpec `json:"triggers"`
}

func (o ServiceTriggersOptions) Validate() error {
	if o.Min < 0 {
		return fmt.Errorf("min must be >= 0")
	}
	if o.Max < 1 {
		return fmt.Errorf("max must be >= 1")
	}
	if o.Max < o.Min {
		return fmt.Errorf("max must be >= min")
	}
	if len(o.Triggers) == 0 {
		return fmt.Errorf("at least one trigger is required")
	}
	seen := map[string]bool{}
	for _, t := range o.Triggers {
		if err := t.Validate(); err != nil {
			return err
		}
		if seen[t.Type] {
			return fmt.Errorf("duplicate trigger type %q", t.Type)
		}
		seen[t.Type] = true
	}
	return nil
}
