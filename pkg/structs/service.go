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
