package structs

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
	// in K8s providers; svc.agent: true in convox.yml). Surfaces to Console3
	// so per-service UI affordances that don't apply to agents (e.g. the
	// scale-override toggle from item 23) can be hidden client-side. Zero
	// value preserves existing wire shape on pre-3.24.6 racks (decoded as
	// false; Console3 fallback shows the affordance). 3.24.6+ rack populates
	// from the manifest service definition.
	Agent bool `json:"agent,omitempty"`

	// GPU runtime telemetry aggregated as average across pods in the service.
	// Populated by provider/k8s/prometheus.go via a single batched Prom query
	// per ServiceList call. Pointer-typed so:
	//   nil          → "no data populator wired" (mixed-skew, prom unreachable,
	//                   no service has GPU > 0);
	//   non-nil zero → "real averaged reading at idle";
	//   non-nil > 0  → "real averaged reading under load."
	// See Process struct doc for the full state-disambiguation rationale.
	GpuUtilAvg     *float64 `json:"gpu-util-avg,omitempty"`
	GpuMemUsedAvg  *int64   `json:"gpu-mem-used-avg,omitempty"`
	GpuMemTotalAvg *int64   `json:"gpu-mem-total-avg,omitempty"`

	// Extended GPU runtime telemetry — DCGM profiling counters averaged across
	// pods in the service. Same pointer-tri-state semantics as GpuUtilAvg
	// above. Populated by the same batched Prom query in
	// provider/k8s/service.go alongside GpuUtilAvg/GpuMemUsedAvg/GpuMemTotalAvg.
	//
	//   GpuTensorActiveAvg → DCGM_FI_PROF_PIPE_TENSOR_ACTIVE × 100 (percent)
	//   GpuSmActiveAvg     → DCGM_FI_PROF_SM_ACTIVE × 100 (percent)
	//   GpuDramActiveAvg   → DCGM_FI_PROF_DRAM_ACTIVE × 100 (percent)
	//   GpuFp16ActiveAvg   → DCGM_FI_PROF_PIPE_FP16_ACTIVE (active fraction)
	//   GpuFp32ActiveAvg   → DCGM_FI_PROF_PIPE_FP32_ACTIVE (active fraction)
	//   GpuPowerWAvg       → DCGM_FI_DEV_POWER_USAGE (watts)
	GpuTensorActiveAvg *float64 `json:"gpu-tensor-active-avg,omitempty"`
	GpuSmActiveAvg     *float64 `json:"gpu-sm-active-avg,omitempty"`
	GpuDramActiveAvg   *float64 `json:"gpu-dram-active-avg,omitempty"`
	GpuFp16ActiveAvg   *float64 `json:"gpu-fp16-active-avg,omitempty"`
	GpuFp32ActiveAvg   *float64 `json:"gpu-fp32-active-avg,omitempty"`
	GpuPowerWAvg       *float64 `json:"gpu-power-w-avg,omitempty"`

	// ScaleOverrideActive reflects whether the service Deployment has
	// the convox.com/scale-override-active=true annotation set.
	// Populated by ServiceList/ServiceGet from the per-Deployment
	// metadata read path. When *true, future ReleasePromote calls
	// preserve the runtime replica count and skip yaml-declared
	// scale.count.min.
	//
	// Pointer-nullable per BC-02 mixed-skew reasoning: three distinct
	// states must be disambiguated on the wire so item 11's Console3
	// toggle UI can graceful-degrade against pre-3.24.6 racks:
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
