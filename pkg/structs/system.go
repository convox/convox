package structs

import "io"

type System struct {
	Count      int               `json:"count"`
	Domain     string            `json:"domain"`
	Name       string            `json:"name"`
	Outputs    map[string]string `json:"outputs,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Provider   string            `json:"provider"`
	RackDomain string            `json:"rack-domain"`
	Region     string            `json:"region"`
	Status     string            `json:"status"`
	Type       string            `json:"type"`
	Version    string            `json:"version"`
}

type SystemInstallOptions struct {
	Id         *string
	Name       *string `flag:"name,n"`
	Parameters map[string]string
	Raw        *bool   `flag:"raw"`
	Version    *string `flag:"version,v"`
}

type SystemProcessesOptions struct {
	All *bool `flag:"all,a" query:"all"`
}

type SystemUninstallOptions struct {
	Force *bool `flag:"force,f"`
	Input io.Reader
}

type SystemUpdateOptions struct {
	Count      *int              `param:"count"`
	Force      *bool             `param:"force"`
	Parameters map[string]string `param:"parameters"`
	Type       *string           `param:"type"`
	Version    *string           `param:"version"`
}

type SystemJwtOptions struct {
	Role           *string `param:"role"`
	DurationInHour *string `param:"durationInHour"`
}

type SystemJwt struct {
	Token string `json:"token"`
}

type RackData struct {
	Host string `json:"host"`
}

type Runtime struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

type Runtimes []Runtime

type RuntimeAttachOptions struct {
	Runtime *string `param:"runtime"`
}

type WorkflowResp struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type WorkflowListResp struct {
	Oid       string         `json:"oid"`
	Workflows []WorkflowResp `json:"workflows"`
}

type WorkflowCustomRunOptions struct {
	App    *string `param:"app" flag:"app,a"`
	Branch *string `param:"branch" flag:"branch"`
	Commit *string `param:"commit" flag:"commit"`
	Title  *string `param:"title" flag:"title"`
}

type WorkflowCustomRunResp struct {
	JobID string `json:"jod_id"`
}
