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
	Host     string    `json:"host"`
	Image    string    `json:"image"`
	Instance string    `json:"instance"`
	Memory   float64   `json:"memory"`
	Name     string    `json:"name"`
	Ports    []string  `json:"ports"`
	Release  string    `json:"release"`
	Started  time.Time `json:"started"`
	Status   string    `json:"status"`
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
}

func (p *Process) sortKey() string {
	return fmt.Sprintf("%s-%s-%s", p.App, p.Name, p.Id)
}

func (ps Processes) Less(i, j int) bool {
	return ps[i].sortKey() < ps[j].sortKey()
}
