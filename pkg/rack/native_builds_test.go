package rack

import "testing"

func TestInertNativeBuildWarning(t *testing.T) {
	tests := []struct {
		name string
		vars map[string]string
		warn bool
	}{
		{"unset", map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true"}, false},
		{"off", map[string]string{nativeBuildParam: "false"}, false},
		{"on with both", map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true"}, false},
		{"stored as 1", map[string]string{nativeBuildParam: "1", "karpenter_enabled": "true", "build_node_enabled": "1"}, false},
		{"no karpenter", map[string]string{nativeBuildParam: "true", "karpenter_enabled": "false", "build_node_enabled": "true"}, true},
		{"no build node", map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true"}, true},
		{"on as 1 with nothing", map[string]string{nativeBuildParam: "1"}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := inertNativeBuildWarning(test.vars); (got != "") != test.warn {
				t.Errorf("warning = %q, want warn %t", got, test.warn)
			}
		})
	}
}
