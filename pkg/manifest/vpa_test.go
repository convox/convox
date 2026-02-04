package manifest_test

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
)

func strptr(s string) *string { return &s }
func boolptr(b bool) *bool    { return &b }

func TestVPAValidate(t *testing.T) {
	t.Run("missing UpdateMode", func(t *testing.T) {
		v := &manifest.VPA{}
		err := v.Validate()
		if err == nil {
			t.Errorf("expected error for missing UpdateMode, got nil")
		}
	})

	t.Run("valid modes", func(t *testing.T) {
		modes := []string{"Off", "Initial", "Recreate"}
		for _, mode := range modes {
			v := &manifest.VPA{UpdateMode: mode}
			err := v.Validate()
			if err != nil {
				t.Errorf("expected no error for mode %s, got %v", mode, err)
			}
		}
	})

	t.Run("invalid mode", func(t *testing.T) {
		v := &manifest.VPA{UpdateMode: "Invalid"}
		err := v.Validate()
		if err == nil {
			t.Errorf("expected error for invalid mode, got nil")
		}
	})

	t.Run("cpuOnly and memOnly both true", func(t *testing.T) {
		v := &manifest.VPA{CpuOnly: boolptr(true), MemOnly: boolptr(true)}
		err := v.Validate()
		if err == nil {
			t.Errorf("expected error when both cpuOnly and memOnly are true, got nil")
		}
	})

	t.Run("cpuOnly true, memOnly false", func(t *testing.T) {
		v := &manifest.VPA{UpdateMode: "Off", CpuOnly: boolptr(true), MemOnly: boolptr(false)}
		err := v.Validate()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("memOnly true, cpuOnly false", func(t *testing.T) {
		v := &manifest.VPA{UpdateMode: "Off", CpuOnly: boolptr(false), MemOnly: boolptr(true)}
		err := v.Validate()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}
