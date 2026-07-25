package cli

import (
	"strings"
	"testing"
)

func TestFamilyArch(t *testing.T) {
	cases := []struct {
		family string
		want   string
	}{
		{"c6g", "arm64"},
		{"c6gd", "arm64"},
		{"c7gn", "arm64"},
		{"t4g", "arm64"},
		{"m8g", "arm64"},
		{"x2gd", "arm64"},
		{"im4gn", "arm64"},
		{"is4gen", "arm64"},
		{"g5g", "arm64"},
		{"hpc7g", "arm64"},
		{"a1", "arm64"},
		{"c5", "amd64"},
		{"m7i", "amd64"},
		{"t3a", "amd64"},
		{"g4dn", "amd64"},
		{"g5", "amd64"},
		{"x2idn", "amd64"},
		{"inf2", "amd64"},
		{"C6G", "arm64"},
		{" c6g ", "arm64"},
		{"mac2-m2", ""},
		{"u-6tb1", ""},
		{"", ""},
	}

	for _, c := range cases {
		if got := familyArch(c.family); got != c.want {
			t.Errorf("familyArch(%q) = %q, want %q", c.family, got, c.want)
		}
	}
}

func TestInstanceArch(t *testing.T) {
	cases := []struct {
		instanceType string
		want         string
	}{
		{"t4g.large", "arm64"},
		{"m5.xlarge", "amd64"},
		{"t3.medium,t3.large", "amd64"},
		{"t4g.medium,m5.large", "arm64"},
		{"", ""},
		{"weird", ""},
	}

	for _, c := range cases {
		if got := instanceArch(c.instanceType); got != c.want {
			t.Errorf("instanceArch(%q) = %q, want %q", c.instanceType, got, c.want)
		}
	}
}

func TestValidateKarpenterArchFamilies(t *testing.T) {
	cases := []struct {
		name    string
		params  map[string]string
		current map[string]string
		wantErr bool
	}{
		{
			name:    "workload conflict",
			params:  map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64", "karpenter_instance_families": "c6g"},
			wantErr: true,
		},
		{
			name:    "workload conflict arm",
			params:  map[string]string{"karpenter_enabled": "true", "karpenter_arch": "arm64", "karpenter_instance_families": "c5,m5"},
			wantErr: true,
		},
		{
			name:   "workload multi arch matches",
			params: map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64,arm64", "karpenter_instance_families": "c6g"},
		},
		{
			name:   "workload one family matches",
			params: map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64", "karpenter_instance_families": "c6g,c5"},
		},
		{
			name:   "unknown family skipped",
			params: map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64", "karpenter_instance_families": "mac2-m2"},
		},
		{
			name:    "build conflict via build_node_type",
			params:  map[string]string{"karpenter_enabled": "true", "build_node_type": "m5.large", "karpenter_build_instance_families": "c6g"},
			wantErr: true,
		},
		{
			name:    "build conflict via node_type fallback",
			params:  map[string]string{"karpenter_enabled": "true", "node_type": "t4g.large", "karpenter_build_instance_families": "c5"},
			wantErr: true,
		},
		{
			name:   "build arch underivable skipped",
			params: map[string]string{"karpenter_enabled": "true", "karpenter_build_instance_families": "c6g"},
		},
		{
			name:   "build family matches",
			params: map[string]string{"karpenter_enabled": "true", "build_node_type": "t4g.large", "karpenter_build_instance_families": "c6g,c7g"},
		},
		{
			name:    "karpenter disabled skips",
			params:  map[string]string{"karpenter_arch": "amd64", "karpenter_instance_families": "c6g"},
			current: map[string]string{"karpenter_enabled": "false"},
		},
		{
			name:    "stale conflict caught on enable",
			params:  map[string]string{"karpenter_enabled": "true"},
			current: map[string]string{"karpenter_arch": "amd64", "karpenter_instance_families": "c6g"},
			wantErr: true,
		},
		{
			name:    "conflict on already-enabled rack",
			params:  map[string]string{"karpenter_instance_families": "c6g"},
			current: map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64"},
			wantErr: true,
		},
		{
			name:    "workload arch derived from node_type",
			params:  map[string]string{"karpenter_enabled": "true", "karpenter_instance_families": "c6g"},
			current: map[string]string{"node_type": "m5.large"},
			wantErr: true,
		},
		{
			name:    "no relevant keys in call",
			params:  map[string]string{"karpenter_cpu_limit": "50"},
			current: map[string]string{"karpenter_enabled": "true", "karpenter_arch": "amd64", "karpenter_instance_families": "c6g"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateKarpenterArchFamilies(c.params, c.current)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAndMutateParamsArchFamilyConflict(t *testing.T) {
	params := map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true", "karpenter_arch": "amd64", "karpenter_build_instance_families": "c6g"}
	current := map[string]string{"build_node_type": "m5.large"}
	err := validateAndMutateParams(params, "aws", current, false)
	if err == nil {
		t.Fatalf("expected arch/family conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "karpenter_build_instance_families") {
		t.Errorf("expected arch/family conflict error, got: %v", err)
	}

	params = map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true", "karpenter_arch": "amd64,arm64", "karpenter_build_instance_families": "c6g"}
	current = map[string]string{"build_node_type": "c6g.large"}
	if err := validateAndMutateParams(params, "aws", current, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
