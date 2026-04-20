package manifest_test

import (
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
)

func i64ptr(i int64) *int64 { return &i }

func TestServiceSecurityContextValidate(t *testing.T) {
	t.Run("empty is valid", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error for empty, got %v", err)
		}
	})

	t.Run("RuntimeDefault seccomp ok", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{SeccompProfile: "RuntimeDefault"}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Unconfined seccomp ok", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{SeccompProfile: "Unconfined"}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Localhost seccomp rejected with specific message", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{SeccompProfile: "Localhost"}
		err := sc.Validate()
		if err == nil {
			t.Fatal("expected error for Localhost, got nil")
		}
		if !strings.Contains(err.Error(), "Localhost is not supported") {
			t.Errorf("expected Localhost-specific error, got %v", err)
		}
	})

	t.Run("unknown seccomp rejected", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{SeccompProfile: "Runitme"}
		err := sc.Validate()
		if err == nil {
			t.Fatal("expected error for typo, got nil")
		}
		if !strings.Contains(err.Error(), "allowed: RuntimeDefault, Unconfined") {
			t.Errorf("expected allowed-values hint, got %v", err)
		}
	})

	t.Run("valid capability names accepted", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			Capabilities: &manifest.ServiceSecurityContextCapabilities{
				Add:  []string{"NET_BIND_SERVICE", "SYS_PTRACE"},
				Drop: []string{"ALL"},
			},
		}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("CAP_ prefix rejected with fix hint", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			Capabilities: &manifest.ServiceSecurityContextCapabilities{
				Add: []string{"CAP_NET_BIND_SERVICE"},
			},
		}
		err := sc.Validate()
		if err == nil {
			t.Fatal("expected error for CAP_ prefix, got nil")
		}
		if !strings.Contains(err.Error(), `use "NET_BIND_SERVICE"`) {
			t.Errorf("expected fix hint, got %v", err)
		}
	})

	t.Run("lowercase capability rejected", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			Capabilities: &manifest.ServiceSecurityContextCapabilities{
				Drop: []string{"all"},
			},
		}
		if err := sc.Validate(); err == nil {
			t.Error("expected error for lowercase cap")
		}
	})

	t.Run("runAsNonRoot true + runAsUser 0 rejected", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			RunAsNonRoot: boolptr(true),
			RunAsUser:    i64ptr(0),
		}
		err := sc.Validate()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "runAsNonRoot=true is incompatible with runAsUser=0") {
			t.Errorf("expected specific error, got %v", err)
		}
	})

	t.Run("runAsNonRoot true + runAsUser non-zero ok", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			RunAsNonRoot: boolptr(true),
			RunAsUser:    i64ptr(1000),
		}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("runAsNonRoot false + runAsUser 0 ok", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			RunAsNonRoot: boolptr(false),
			RunAsUser:    i64ptr(0),
		}
		if err := sc.Validate(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestServiceSecurityContextCapabilitiesHasCapabilities(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var c *manifest.ServiceSecurityContextCapabilities
		if c.HasCapabilities() {
			t.Error("nil should return false")
		}
	})

	t.Run("empty struct", func(t *testing.T) {
		c := &manifest.ServiceSecurityContextCapabilities{}
		if c.HasCapabilities() {
			t.Error("empty should return false")
		}
	})

	t.Run("empty slices", func(t *testing.T) {
		c := &manifest.ServiceSecurityContextCapabilities{Add: []string{}, Drop: []string{}}
		if c.HasCapabilities() {
			t.Error("empty slices should return false")
		}
	})

	t.Run("add only", func(t *testing.T) {
		c := &manifest.ServiceSecurityContextCapabilities{Add: []string{"NET_BIND_SERVICE"}}
		if !c.HasCapabilities() {
			t.Error("add non-empty should return true")
		}
	})

	t.Run("drop only", func(t *testing.T) {
		c := &manifest.ServiceSecurityContextCapabilities{Drop: []string{"ALL"}}
		if !c.HasCapabilities() {
			t.Error("drop non-empty should return true")
		}
	})
}

func TestServiceSecurityContextHasSecurityContext(t *testing.T) {
	t.Run("empty returns false", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{}
		if sc.HasSecurityContext() {
			t.Error("empty should return false")
		}
	})

	t.Run("empty capabilities pointer returns false", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{
			Capabilities: &manifest.ServiceSecurityContextCapabilities{},
		}
		if sc.HasSecurityContext() {
			t.Error("empty Capabilities struct should not count as configured")
		}
	})

	t.Run("seccomp only returns true", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{SeccompProfile: "RuntimeDefault"}
		if !sc.HasSecurityContext() {
			t.Error("seccomp set should return true")
		}
	})

	t.Run("runAsNonRoot false counts as configured", func(t *testing.T) {
		sc := manifest.ServiceSecurityContext{RunAsNonRoot: boolptr(false)}
		if !sc.HasSecurityContext() {
			t.Error("explicit false is still configuration")
		}
	})
}
