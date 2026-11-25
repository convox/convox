package k8s

import (
	"strings"

	ac "k8s.io/api/core/v1"
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

// sandboxDroppedCapabilities are removed from sandbox containers. Raw sockets allow packet crafting and
// spoofing on the node's network; nothing the sandbox images or agents rely on needs them.
var sandboxDroppedCapabilities = []ac.Capability{"NET_RAW"}

// hardenSandboxPod tightens a sandbox pod without changing what code inside it can do as the sandbox user:
// no Kubernetes API token is mounted, the runtime's default seccomp profile filters dangerous syscalls, and
// raw sockets are dropped. Privilege escalation stays allowed, because sudo is part of the image contract.
func hardenSandboxPod(s *ac.PodSpec) {
	off := false
	s.AutomountServiceAccountToken = &off
	if s.SecurityContext == nil {
		s.SecurityContext = &ac.PodSecurityContext{}
	}
	s.SecurityContext.SeccompProfile = &ac.SeccompProfile{Type: ac.SeccompProfileTypeRuntimeDefault}
	for i := range s.Containers {
		c := &s.Containers[i]
		if c.SecurityContext == nil {
			c.SecurityContext = &ac.SecurityContext{}
		}
		if c.SecurityContext.Capabilities == nil {
			c.SecurityContext.Capabilities = &ac.Capabilities{}
		}
		for _, cap := range sandboxDroppedCapabilities {
			if !hasCapability(c.SecurityContext.Capabilities.Drop, cap) {
				c.SecurityContext.Capabilities.Drop = append(c.SecurityContext.Capabilities.Drop, cap)
			}
		}
	}
}

func hasCapability(list []ac.Capability, c ac.Capability) bool {
	for _, x := range list {
		if x == c {
			return true
		}
	}
	return false
}
