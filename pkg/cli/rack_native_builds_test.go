package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateNativeBuilds(t *testing.T) {
	ready := map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small"}

	pools := filepath.Join(t.TempDir(), "pools.json")
	require.NoError(t, os.WriteFile(pools, []byte(`[{"name":"arm","arch":"arm64","instance_families":"c6g","instance_sizes":"large"}]`), 0600))

	tests := []struct {
		name    string
		params  map[string]string
		current map[string]string
		err     string
	}{
		{name: "prerequisites current", params: map[string]string{nativeBuildParam: "true"}, current: ready},
		{name: "prerequisites in call", params: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "karpenter_auth_mode": "true", "build_node_enabled": "true"}, current: map[string]string{"node_capacity_type": "ON_DEMAND"}},
		{name: "no karpenter", params: map[string]string{nativeBuildParam: "true"}, current: map[string]string{"build_node_enabled": "true"}, err: "requires karpenter_enabled=true and build_node_enabled=true"},
		{name: "no build node", params: map[string]string{nativeBuildParam: "true"}, current: map[string]string{"karpenter_enabled": "true"}, err: "requires karpenter_enabled=true and build_node_enabled=true"},
		{name: "build node stored as 1", params: map[string]string{nativeBuildParam: "true"}, current: map[string]string{"karpenter_enabled": "true", "build_node_enabled": "1"}},
		{name: "off needs nothing", params: map[string]string{nativeBuildParam: "false"}, current: map[string]string{}},
		{name: "unrelated call on an inert rack", params: map[string]string{"node_disk": "40"}, current: map[string]string{nativeBuildParam: "true"}},
		{name: "prerequisite removed later", params: map[string]string{"build_node_enabled": "false"}, current: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true"}},
		{
			name:    "families miss a workload arch",
			params:  map[string]string{nativeBuildParam: "true"},
			current: map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_arch": "amd64,arm64", "karpenter_build_instance_families": "c5,m5"},
			err:     "has no arm64 family",
		},
		{
			name:    "non canonical true",
			params:  map[string]string{nativeBuildParam: "1"},
			current: map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_arch": "arm64", "karpenter_build_instance_families": "c5"},
			err:     "has no arm64 family",
		},
		{
			name:    "families cover both",
			params:  map[string]string{nativeBuildParam: "True"},
			current: map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_arch": "amd64,arm64", "karpenter_build_instance_families": "c5,c6g"},
		},
		{
			name:    "families narrowed on an opted in rack",
			params:  map[string]string{"karpenter_build_instance_families": "c5"},
			current: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_arch": "amd64,arm64"},
			err:     "has no arm64 family",
		},
		{
			name:    "additional pool arch",
			params:  map[string]string{nativeBuildParam: "true"},
			current: map[string]string{"karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_build_instance_families": "c5", "additional_karpenter_nodepools_config": `[{"name":"arm","arch":"arm64"}]`},
			err:     "has no arm64 family",
		},
		{
			name:    "additional pool as a file",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools},
			current: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_build_instance_families": "c5"},
			err:     "has no arm64 family",
		},
		{
			name:    "node type changes the workload arch",
			params:  map[string]string{"node_type": "t4g.small"},
			current: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "build_node_type": "t3.large", "karpenter_build_instance_families": "c5,m5"},
			err:     "has no arm64 family",
		},
		{name: "non canonical true without prerequisites", params: map[string]string{nativeBuildParam: "1"}, current: map[string]string{}, err: "requires karpenter_enabled=true and build_node_enabled=true"},
		{
			name:    "off skips the family rule",
			params:  map[string]string{nativeBuildParam: "false", "karpenter_build_instance_families": "c5"},
			current: map[string]string{nativeBuildParam: "true", "karpenter_enabled": "true", "build_node_enabled": "true", "node_type": "t3.small", "karpenter_arch": "amd64,arm64"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAndMutateParams(test.params, "aws", test.current, false)
			if test.err == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.err)
		})
	}
}

func TestNativeBuildsNonAWS(t *testing.T) {
	err := validateAndMutateParams(map[string]string{nativeBuildParam: "true"}, "gcp", map[string]string{}, false)
	require.ErrorContains(t, err, "karpenter parameters are only supported for AWS racks")
}

func TestNativeBuildsWired(t *testing.T) {
	declared := func(path, name string) bool {
		for _, block := range parseHCLBody(t, path).Blocks {
			if block.Type == "variable" && len(block.Labels) == 1 && block.Labels[0] == name {
				return true
			}
		}
		return false
	}

	for path, name := range map[string]string{
		"../../terraform/system/aws/variables.tf":  nativeBuildParam,
		"../../terraform/cluster/aws/variables.tf": "karpenter_build_native_archs",
		"../../terraform/rack/aws/variables.tf":    "build_native_arch",
		"../../terraform/api/aws/variables.tf":     "build_native_arch",
	} {
		if !declared(path, name) {
			t.Errorf("%s: no variable %q", path, name)
		}
	}

	passes := func(path, module, arg string) {
		for _, block := range parseHCLBody(t, path).Blocks {
			if block.Type == "module" && len(block.Labels) == 1 && block.Labels[0] == module {
				if _, ok := block.Body.Attributes[arg]; ok {
					return
				}
			}
		}
		t.Errorf("%s: module %q does not pass %s", path, module, arg)
	}

	passes("../../terraform/system/aws/main.tf", "cluster", "karpenter_build_native_archs")
	passes("../../terraform/system/aws/main.tf", "rack", "build_native_arch")
	passes("../../terraform/rack/aws/main.tf", "api", "build_native_arch")

	for path, want := range map[string]string{
		"../../terraform/api/aws/main.tf":                   "BUILD_NATIVE_ARCH",
		"../../terraform/cluster/aws/karpenter_nodepool.tf": "var.karpenter_build_native_archs",
	} {
		if !strings.Contains(string(mustReadFile(t, path)), want) {
			t.Errorf("%s: no %s", path, want)
		}
	}
}
