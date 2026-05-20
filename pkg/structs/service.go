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

	Agent bool `json:"agent,omitempty"`

	// GPU telemetry — per-metric mean across pods. nil = metric unavailable, non-nil = real reading.
	GpuUtil     *float64 `json:"gpu-util,omitempty"`
	GpuMemUsed  *int64   `json:"gpu-mem-used,omitempty"`
	GpuMemTotal *int64   `json:"gpu-mem-total,omitempty"`

	// Extended DCGM profiling counters (percent or watts). Same nil semantics as GpuUtil.
	GpuTensorActive *float64 `json:"gpu-tensor-active,omitempty"`
	GpuSmActive     *float64 `json:"gpu-sm-active,omitempty"`
	GpuDramActive   *float64 `json:"gpu-dram-active,omitempty"`
	GpuFp16Active   *float64 `json:"gpu-fp16-active,omitempty"`
	GpuFp32Active   *float64 `json:"gpu-fp32-active,omitempty"`
	GpuPowerW       *float64 `json:"gpu-power-w,omitempty"`

	// Pointer tri-state: nil = pre-3.24.6 rack (unsupported), *false = off, *true = on.
	ScaleOverrideActive    *bool `json:"scale-override-active,omitempty"`
	TriggersOverrideActive *bool `json:"triggers-override-active,omitempty"` // same contract as ScaleOverrideActive
}

// ServiceAutoscaleState reports current autoscale state for a service.
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

// ServiceNlbPort is the wire shape for v2 NLB port config surfaced by CLI and Console.
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

// Trigger type constants shared across rack, SDK, GraphQL, and CLI.
const (
	TriggerTypeCPU            = "cpu"
	TriggerTypeMemory         = "memory"
	TriggerTypeGPUUtilization = "gpuUtilization"
	TriggerTypeQueueDepth     = "queueDepth"
)

// TriggerSpec describes a single autoscale trigger.
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
	if t.Type != TriggerTypeQueueDepth && t.Threshold > 100 {
		return fmt.Errorf("percent triggers must be <= 100")
	}
	return nil
}

// ServiceTriggersOptions carries the inputs for ServiceTriggersEnable.
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
