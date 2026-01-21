package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	ac "k8s.io/api/core/v1"
)

func TestServiceSecurityContextHelper(t *testing.T) {
	b := func(v bool) *bool { return &v }
	i := func(v int64) *int64 { return &v }

	t.Run("nil service returns nil", func(t *testing.T) {
		if sc := serviceSecurityContext(nil); sc != nil {
			t.Errorf("expected nil, got %+v", sc)
		}
	})

	t.Run("no config returns nil", func(t *testing.T) {
		s := &manifest.Service{}
		if sc := serviceSecurityContext(s); sc != nil {
			t.Errorf("expected nil, got %+v", sc)
		}
	})

	t.Run("privileged only", func(t *testing.T) {
		s := &manifest.Service{Privileged: true}
		sc := serviceSecurityContext(s)
		if sc == nil {
			t.Fatal("expected non-nil")
		}
		if sc.Privileged == nil || !*sc.Privileged {
			t.Error("expected Privileged=true")
		}
	})

	t.Run("full securityContext maps all fields", func(t *testing.T) {
		s := &manifest.Service{
			SecurityContext: manifest.ServiceSecurityContext{
				RunAsNonRoot:             b(true),
				RunAsUser:                i(1000),
				RunAsGroup:               i(1001),
				ReadOnlyRootFilesystem:   b(true),
				AllowPrivilegeEscalation: b(false),
				Capabilities: &manifest.ServiceSecurityContextCapabilities{
					Add:  []string{"NET_BIND_SERVICE"},
					Drop: []string{"ALL"},
				},
				SeccompProfile: "RuntimeDefault",
			},
		}
		sc := serviceSecurityContext(s)
		if sc == nil {
			t.Fatal("expected non-nil")
		}
		if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
			t.Error("RunAsNonRoot not mapped")
		}
		if sc.RunAsUser == nil || *sc.RunAsUser != 1000 {
			t.Error("RunAsUser not mapped")
		}
		if sc.RunAsGroup == nil || *sc.RunAsGroup != 1001 {
			t.Error("RunAsGroup not mapped")
		}
		if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
			t.Error("ReadOnlyRootFilesystem not mapped")
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Error("AllowPrivilegeEscalation should be false")
		}
		if sc.Capabilities == nil {
			t.Fatal("Capabilities should be set")
		}
		if len(sc.Capabilities.Add) != 1 || sc.Capabilities.Add[0] != ac.Capability("NET_BIND_SERVICE") {
			t.Errorf("Capabilities.Add not mapped: %+v", sc.Capabilities.Add)
		}
		if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != ac.Capability("ALL") {
			t.Errorf("Capabilities.Drop not mapped: %+v", sc.Capabilities.Drop)
		}
		if sc.SeccompProfile == nil || sc.SeccompProfile.Type != ac.SeccompProfileTypeRuntimeDefault {
			t.Errorf("SeccompProfile not mapped: %+v", sc.SeccompProfile)
		}
	})

	t.Run("explicit zero values preserved", func(t *testing.T) {
		s := &manifest.Service{
			SecurityContext: manifest.ServiceSecurityContext{
				RunAsNonRoot:             b(false),
				ReadOnlyRootFilesystem:   b(false),
				AllowPrivilegeEscalation: b(false),
				RunAsUser:                i(0),
			},
		}
		sc := serviceSecurityContext(s)
		if sc == nil {
			t.Fatal("expected non-nil")
		}
		if sc.RunAsNonRoot == nil || *sc.RunAsNonRoot {
			t.Error("RunAsNonRoot=false should be preserved")
		}
		if sc.ReadOnlyRootFilesystem == nil || *sc.ReadOnlyRootFilesystem {
			t.Error("ReadOnlyRootFilesystem=false should be preserved")
		}
		if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
			t.Error("AllowPrivilegeEscalation=false should be preserved")
		}
		if sc.RunAsUser == nil || *sc.RunAsUser != 0 {
			t.Error("RunAsUser=0 should be preserved")
		}
	})

	t.Run("empty capabilities does not produce Capabilities field", func(t *testing.T) {
		s := &manifest.Service{
			SecurityContext: manifest.ServiceSecurityContext{
				Capabilities:   &manifest.ServiceSecurityContextCapabilities{},
				SeccompProfile: "RuntimeDefault",
			},
		}
		sc := serviceSecurityContext(s)
		if sc == nil {
			t.Fatal("expected non-nil because seccomp is set")
		}
		if sc.Capabilities != nil {
			t.Errorf("empty Capabilities should not produce non-nil field: %+v", sc.Capabilities)
		}
	})

	t.Run("seccomp Unconfined maps", func(t *testing.T) {
		s := &manifest.Service{
			SecurityContext: manifest.ServiceSecurityContext{SeccompProfile: "Unconfined"},
		}
		sc := serviceSecurityContext(s)
		if sc == nil || sc.SeccompProfile == nil {
			t.Fatal("expected seccomp set")
		}
		if sc.SeccompProfile.Type != ac.SeccompProfileTypeUnconfined {
			t.Errorf("expected Unconfined, got %s", sc.SeccompProfile.Type)
		}
	})
}
