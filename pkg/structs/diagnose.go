package structs

import "time"

type AppDiagnoseOptions struct {
	Checks   *string `flag:"checks,c" query:"checks"`
	Services *string `flag:"service,s" query:"services"`

	AgeThreshold *int  `flag:"age,A" query:"age" default:"300"`
	All          *bool `flag:"all" query:"all"`
	Lines        *int  `flag:"lines,n" query:"lines" default:"200"`
	Events       *bool `query:"events" default:"true"`
	Previous     *bool `query:"previous" default:"true"`
	Describe     *bool `flag:"describe" query:"describe"`
}

type AppDiagnosticReport struct {
	Namespace string               `json:"namespace"`
	Rack      string               `json:"rack"`
	App       string               `json:"app"`
	Timestamp time.Time            `json:"timestamp"`
	Overview  *DiagnosticOverview  `json:"overview,omitempty"`
	InitPods  []DiagnosticInitPod  `json:"initPods,omitempty"`
	Pods      []DiagnosticPod      `json:"pods,omitempty"`
	Summary   *DiagnosticSummary   `json:"summary,omitempty"`
}

type DiagnosticSummary struct {
	Total     int `json:"total"`
	Unhealthy int `json:"unhealthy"`
	NotReady  int `json:"notReady"`
	New       int `json:"new"`
	Healthy   int `json:"healthy"`
}

type DiagnosticOverview struct {
	Services []DiagnosticServiceStatus `json:"services"`
	Events   []DiagnosticEvent         `json:"events,omitempty"`
}

type DiagnosticServiceStatus struct {
	Name            string `json:"name"`
	DesiredReplicas int    `json:"desiredReplicas"`
	ReadyReplicas   int    `json:"readyReplicas"`
	UpdatedReplicas int    `json:"updatedReplicas"`
	Status          string `json:"status"`
	StallReason     string `json:"stallReason,omitempty"`
	Agent           bool   `json:"agent,omitempty"`
}

type DiagnosticEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Object    string    `json:"object"`
	Message   string    `json:"message"`
	Hint      string    `json:"hint,omitempty"`
}

type DiagnosticInitPod struct {
	Name           string                `json:"name"`
	Service        string                `json:"service"`
	Phase          string                `json:"phase"`
	InitContainers []DiagnosticContainer `json:"initContainers"`
}

type DiagnosticContainer struct {
	Name  string `json:"name"`
	State string `json:"state"`
	Logs  string `json:"logs,omitempty"`
}

type DiagnosticPod struct {
	Name           string            `json:"name"`
	Service        string            `json:"service"`
	Phase          string            `json:"phase"`
	Ready          string            `json:"ready"`
	AgeSeconds     int               `json:"ageSeconds"`
	Restarts       int               `json:"restarts"`
	Classification string            `json:"classification"`
	StateDetail    string            `json:"stateDetail,omitempty"`
	Hint           string            `json:"hint,omitempty"`
	Logs           string            `json:"logs,omitempty"`
	PreviousLogs   string            `json:"previousLogs,omitempty"`
	Events         []DiagnosticEvent `json:"events,omitempty"`
	Describe       string            `json:"describe,omitempty"`
}
