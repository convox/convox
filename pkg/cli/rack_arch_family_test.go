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

func TestValidateInstanceTypeList(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr string
	}{
		{"single", "m5.large", ""},
		{"empty", "", ""},
		{"same family", "m5.large,m5.xlarge", ""},
		{"same arch across families", "m5a.2xlarge,m5.2xlarge,m6a.2xlarge,m6i.2xlarge", ""},
		{"all arm", "m6g.large,c6g.large,t4g.large", ""},
		{"all gpu", "g5.xlarge,g4dn.xlarge", ""},
		{"whitespace and trailing comma", "m5.large, m5a.large,", ""},
		{"unknown family alongside known", "m5.large,mac2-m2.metal", ""},
		{"unknown family does not weaken the check", "c7i-flex.large,c7g.large,m5.large", "architecture"},
		{"arm mixed with amd", "m5.large,m6g.large", "architecture"},
		{"amd mixed with arm first", "c6g.large,c5.large", "architecture"},
		{"gpu mixed with cpu", "m5.large,g5.xlarge", "GPU"},
		{"cpu mixed with gpu first", "p3.2xlarge,m5.large", "GPU"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateInstanceTypeList("node_type", c.value)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("expected error containing %q, got: %v", c.wantErr, err)
			}
			if !strings.Contains(err.Error(), "node_type") {
				t.Errorf("expected error to name the parameter, got: %v", err)
			}
		})
	}
}

func TestValidateAndMutateParamsRejectsMixedArchNodeType(t *testing.T) {
	err := validateAndMutateParams(map[string]string{"node_type": "m5.large,m6g.large"}, "aws", map[string]string{"node_type": "m5.large"}, false)
	if err == nil {
		t.Fatalf("expected mixed architecture error, got nil")
	}
	if !strings.Contains(err.Error(), "node_type") {
		t.Errorf("expected error to name node_type, got: %v", err)
	}

	if err := validateAndMutateParams(map[string]string{"node_type": "m5a.2xlarge,m5.2xlarge"}, "aws", map[string]string{"node_type": "m5.large"}, false); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	err = validateAndMutateParams(map[string]string{"build_node_type": "m5.large,m5a.large"}, "aws", map[string]string{"node_type": "m5.large"}, false)
	if err == nil {
		t.Fatalf("expected build_node_type list rejection, got nil")
	}
	if !strings.Contains(err.Error(), "build_node_type takes a single instance type") {
		t.Errorf("expected single instance type error, got: %v", err)
	}

	if err := validateAndMutateParams(map[string]string{"node_type": "s-2vcpu-4gb,g-2vcpu-8gb"}, "do", map[string]string{"node_type": "s-2vcpu-4gb"}, true); err != nil {
		t.Errorf("aws-only check must not fire for other providers: %v", err)
	}
}

func TestArmMachineType(t *testing.T) {
	cases := []struct {
		provider    string
		machineType string
		want        bool
	}{
		{"gcp", "t2a-standard-4", true},
		{"gcp", "c4a-highmem-96-metal", true},
		{"gcp", "n4a-standard-8", true},
		{"gcp", "a4x-highgpu-4g", true},
		{"gcp", " C4A-STANDARD-4 ", true},
		{"gcp", "t2d-standard-4", false},
		{"gcp", "c4-standard-4", false},
		{"gcp", "c4d-standard-4", false},
		{"gcp", "n4-standard-4", false},
		{"gcp", "a4-highgpu-8g", false},
		{"gcp", "n1-standard-2", false},
		{"gcp", "Standard_D4ps_v5", false},
		{"azure", "Standard_D4ps_v5", true},
		{"azure", "Standard_D2pls_v6", true},
		{"azure", "Standard_D2plds_v5", true},
		{"azure", "Standard_E4pds_v5", true},
		{"azure", "Standard_B2pts_v2", true},
		{"azure", "standard_d4ps_v5", true},
		{"azure", "Standard_D4s_v5", false},
		{"azure", "Standard_D2ads_v6", false},
		{"azure", "Standard_DS2_v2", false},
		{"azure", "Standard_DS2_v2_Promo", false},
		{"azure", "Standard_NC4as_T4_v3", false},
		{"azure", "Standard_NP10s", false},
		{"azure", "t2a-standard-4", false},
		{"aws", "t2a-standard-4", false},
		{"aws", "Standard_D4ps_v5", false},
		{"do", "s-2vcpu-4gb", false},
		{"gcp", "", false},
	}

	for _, c := range cases {
		if got := armMachineType(c.provider, c.machineType); got != c.want {
			t.Errorf("armMachineType(%q, %q) = %v, want %v", c.provider, c.machineType, got, c.want)
		}
	}
}

func TestValidateAndMutateParamsArmNodeType(t *testing.T) {
	for provider, tc := range map[string]struct{ armType, impact string }{
		"gcp":   {"t2a-standard-4", "the rack goes offline"},
		"azure": {"Standard_D4ps_v5", "Elasticsearch"},
	} {
		armType := tc.armType
		t.Run(provider, func(t *testing.T) {
			err := validateAndMutateParams(map[string]string{"node_type": armType}, provider, map[string]string{}, false)
			if err == nil {
				t.Fatalf("expected Arm node_type to be refused")
			}
			for _, want := range []string{"node_type", "additional_node_groups_config", "--force"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error to contain %q, got: %v", want, err)
				}
			}

			var forcedErr error
			stderr := captureStderr(t, func() {
				forcedErr = validateAndMutateParams(map[string]string{"node_type": armType}, provider, map[string]string{}, true)
			})
			if forcedErr != nil {
				t.Fatalf("forced Arm node_type should pass, got: %v", forcedErr)
			}
			if !strings.Contains(stderr, "WARNING: node_type "+armType) || !strings.Contains(stderr, tc.impact) {
				t.Errorf("expected a WARNING naming the node_type and %q, got: %q", tc.impact, stderr)
			}

			if err := validateAndMutateParams(map[string]string{"node_disk": "100"}, provider, map[string]string{"node_type": armType}, false); err != nil {
				t.Errorf("a stored Arm node_type must not block other params: %v", err)
			}
		})
	}

	for provider, x86Type := range map[string]string{"gcp": "n2-standard-4", "azure": "Standard_D4s_v5"} {
		if err := validateAndMutateParams(map[string]string{"node_type": x86Type}, provider, map[string]string{}, false); err != nil {
			t.Errorf("%s: x86 node_type should pass, got: %v", provider, err)
		}
	}

	if err := validateAndMutateParams(map[string]string{"node_type": "t4g.large"}, "aws", map[string]string{}, false); err != nil {
		t.Errorf("aws Arm node_type should pass, got: %v", err)
	}
}
