package structs

import (
	"fmt"
	"time"
)

type Process struct {
	Id string `json:"id"`

	App      string    `json:"app"`
	Command  string    `json:"command"`
	Cpu      float64   `json:"cpu"`
	Gpu      int       `json:"gpu"`
	Host     string    `json:"host"`
	Image    string    `json:"image"`
	Instance string    `json:"instance"`
	Memory   float64   `json:"memory"`
	Name     string    `json:"name"`
	Ports    []string  `json:"ports"`
	Release  string    `json:"release"`
	Started  time.Time `json:"started"`
	Status   string    `json:"status"`

	// Service identifier — the convox.yml service name this pod was launched
	// for. Populated at the same processFromPod site that populates Name; both
	// read pd.ObjectMeta.Labels["service"]. Exists alongside Name (rather than
	// replacing it) because Name is wire-stable across older rack versions and
	// CLI column code paths; Service is an explicit named field that the
	// Console3 service-detail chart filter (`process.service === serviceName`)
	// compiles against without overloading Name's existing role. Empty string
	// when the pod has no service label (system pods, build pods); BC-safe
	// value-typed — an absent JSON key decodes as "" which Console3 / Vue
	// treat as "no service identity," same path as system / build pods.
	Service string `json:"service,omitempty"`

	// GPU runtime telemetry, populated by provider/k8s/prometheus.go when the
	// rack's prometheus_url points to a reachable Prometheus AND the pod has a
	// GPU resource request. Pointer-typed so:
	//   nil          → "no data populator wired" (3.24.5 rack on the wire,
	//                   prometheus_url unset, DCGM not deployed, query timeout,
	//                   pod has no GPU);
	//   non-nil zero → "real reading at idle on a real GPU";
	//   non-nil > 0  → "real reading under load."
	// Mixed-skew safe: 3.24.5 rack returning JSON without these keys decodes
	// as nil on the 3.24.6 client side; resolver returns GraphQL null, Vue
	// renders "no data" empty state.
	GpuUtil     *float64 `json:"gpu-util,omitempty"`      // percent 0-100
	GpuMemUsed  *int64   `json:"gpu-mem-used,omitempty"`  // bytes (DCGM_FI_DEV_FB_USED)
	GpuMemTotal *int64   `json:"gpu-mem-total,omitempty"` // bytes (derived: FB_USED + FB_FREE + FB_RESERVED — DCGM default-counters.csv does not emit FB_TOTAL)

	// Extended per-pod DCGM profiling counters. Same pointer-tri-state
	// semantics as GpuUtil. Populated by provider/k8s/process.go alongside
	// the existing util/mem fields. Per-pod (not aggregated like service-
	// side Avg fields).
	GpuTensorActive *float64 `json:"gpu-tensor-active,omitempty"` // percent 0-100 (DCGM_FI_PROF_PIPE_TENSOR_ACTIVE × 100)
	GpuSmActive     *float64 `json:"gpu-sm-active,omitempty"`     // percent 0-100 (DCGM_FI_PROF_SM_ACTIVE × 100)
	GpuDramActive   *float64 `json:"gpu-dram-active,omitempty"`   // percent 0-100 (DCGM_FI_PROF_DRAM_ACTIVE × 100)
	GpuFp16Active   *float64 `json:"gpu-fp16-active,omitempty"`   // percent 0-100 (DCGM_FI_PROF_PIPE_FP16_ACTIVE × 100)
	GpuFp32Active   *float64 `json:"gpu-fp32-active,omitempty"`   // percent 0-100 (DCGM_FI_PROF_PIPE_FP32_ACTIVE × 100)
	GpuPowerW       *float64 `json:"gpu-power-w,omitempty"`       // watts (DCGM_FI_DEV_POWER_USAGE)
}

type Processes []Process

type ProcessExecOptions struct {
	Entrypoint   *bool `header:"Entrypoint"`
	Height       *int  `header:"Height"`
	Tty          *bool `header:"Tty" default:"true"`
	Width        *int  `header:"Width"`
	DisableStdin *bool `header:"Disable-Stdin"`
}

type ProcessListOptions struct {
	Release *string `flag:"release" query:"release"`
	Service *string `flag:"service,s" query:"service"`
}

type ProcessRunOptions struct {
	Command          *string           `header:"Command"`
	Cpu              *int              `flag:"cpu" header:"Cpu"`
	CpuLimit         *int              `flag:"cpu-limit" header:"Cpu-Limit"`
	Environment      map[string]string `header:"Environment"`
	Gpu              *int              `flag:"gpu" header:"Gpu"`
	GpuVendor        *string           `flag:"gpu-vendor" header:"Gpu-Vendor"`
	Height           *int              `header:"Height"`
	Image            *string           `header:"Image"`
	Memory           *int              `flag:"memory" header:"Memory"`
	MemoryLimit      *int              `flag:"memory-limit" header:"Memory-Limit"`
	Release          *string           `flag:"release" header:"Release"`
	Volumes          map[string]string `header:"Volumes"`
	UseServiceVolume *bool             `flag:"use-service-volume" header:"Use-Service-Volume"`
	Width            *int              `header:"Width"`
	Privileged       *bool             `header:"Privileged"`
	NodeLabels       *string           `flag:"node-labels" header:"Node-Labels"`
	SystemCritical   *bool             `flag:"system-critical" header:"System-Critical"`
	IsBuild          bool
	BuildArch        *string
}

func (p *Process) sortKey() string {
	return fmt.Sprintf("%s-%s-%s", p.App, p.Name, p.Id)
}

func (ps Processes) Less(i, j int) bool {
	return ps[i].sortKey() < ps[j].sortKey()
}
