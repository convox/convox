package k8s

import (
	"strings"
)

const (
	SandboxLabel   = "convox.com/sandbox"
	SandboxIDLabel = "convox.com/sandbox-id"
)

func (p *Provider) runtimeClassAllowed(name string) bool {
	for _, rc := range p.SandboxRuntimeClasses {
		if rc == name {
			return true
		}
	}
	return false
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
