package cli

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateAndMutateParams_BudgetRegex(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"plain number", "3", false},
		{"percentage", "10%", false},
		{"zero", "0", false},
		{"hundred percent", "100%", false},
		{"double percent", "10%%", true},
		{"text", "abc", true},
		{"empty", "", false}, // empty is allowed (not set)
		{"negative", "-1", true},
		{"decimal", "1.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":              "true",
				"karpenter_disruption_budget_nodes": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("budget=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_TaintFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"key=value:NoSchedule", "key=value:NoSchedule", false},
		{"key=:NoSchedule (empty value)", "key=:NoSchedule", false},
		{"key:NoSchedule (no value, no equals)", "key:NoSchedule", false},
		{"mixed format", "key1=val1:NoSchedule,key2:NoExecute", false},
		{"PreferNoSchedule", "gpu=true:PreferNoSchedule", false},
		{"NoExecute", "dedicated=build:NoExecute", false},
		{"invalid effect", "key:InvalidEffect", true},
		{"no colon", "keyvalue", true},
		{"empty key", ":NoSchedule", true},
		{"empty string", "", false}, // not set
		{"multiple commas", "a=b:NoSchedule,c=d:NoExecute,e:PreferNoSchedule", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":    "true",
				"karpenter_node_taints": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("taint=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_ReservedLabels(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		value   string
		wantErr bool
	}{
		// Workload labels
		{"workload: valid label", "karpenter_node_labels", "team=backend", false},
		{"workload: reserved convox.io/nodepool", "karpenter_node_labels", "convox.io/nodepool=custom", true},
		{"workload: reserved among others", "karpenter_node_labels", "ok=fine,convox.io/nodepool=x", true},
		{"workload: empty", "karpenter_node_labels", "", false},

		// Build labels
		{"build: valid label", "karpenter_build_node_labels", "cost-center=builds", false},
		{"build: reserved convox-build", "karpenter_build_node_labels", "convox-build=yes", true},
		{"build: reserved convox.io/nodepool", "karpenter_build_node_labels", "convox.io/nodepool=x", true},
		{"build: empty", "karpenter_build_node_labels", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled": "true",
				tt.param:            tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("%s=%q: got err=%v, wantErr=%v", tt.param, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterConfigBase64(t *testing.T) {
	// Valid config should be re-encoded as normalized base64
	config := map[string]interface{}{
		"nodePool": map[string]interface{}{
			"disruption": map[string]interface{}{
				"consolidationPolicy": "WhenEmpty",
			},
		},
	}
	raw, _ := json.Marshal(config)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"raw JSON", string(raw), false},
		{"base64 encoded", base64.StdEncoding.EncodeToString(raw), false},
		{"invalid JSON", "{bad", true},
		{"invalid base64", "not-base64!@#", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_config": tt.input,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("config=%q: got err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			// On success with non-empty input, result should be valid base64
			if err == nil && tt.input != "" {
				decoded, err := base64.StdEncoding.DecodeString(params["karpenter_config"])
				if err != nil {
					t.Errorf("output not valid base64: %v", err)
				}
				var out map[string]interface{}
				if err := json.Unmarshal(decoded, &out); err != nil {
					t.Errorf("decoded output not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterConfigSizeLimit(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"under limit (1KB)", 1024, false},
		{"just under limit (63KB)", 63 * 1024, false},
		{"over limit (65KB)", 65 * 1024, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a valid JSON object padded to the target size with a long string value
			padding := strings.Repeat("x", tt.size)
			config := `{"nodePool":{"description":"` + padding + `"}}`
			params := map[string]string{
				"karpenter_config": config,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if tt.wantErr && err == nil {
				t.Errorf("size=%d: expected error, got nil", tt.size)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("size=%d: unexpected error: %v", tt.size, err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), "exceeds maximum size") {
				t.Errorf("size=%d: expected size error, got: %v", tt.size, err)
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterConfigProtectedFields(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			"valid nodePool override",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"disruption": map[string]interface{}{"consolidationPolicy": "WhenEmpty"},
				},
			},
			false, "",
		},
		{
			"valid ec2NodeClass override",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"blockDeviceMappings": []interface{}{},
				},
			},
			false, "",
		},
		{
			"blocked: ec2NodeClass.role",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{"role": "bad"},
			},
			true, "ec2NodeClass.role is managed by Convox",
		},
		{
			"blocked: ec2NodeClass.subnetSelectorTerms",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{"subnetSelectorTerms": []interface{}{}},
			},
			true, "ec2NodeClass.subnetSelectorTerms is managed by Convox",
		},
		{
			"blocked: nodeClassRef",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"nodeClassRef": map[string]interface{}{"name": "x"},
						},
					},
				},
			},
			true, "nodeClassRef is managed by Convox",
		},
		{
			"unknown top-level key",
			map[string]interface{}{
				"unknown": "value",
			},
			true, "unknown top-level key",
		},
		{
			"ec2NodeClass not a map",
			map[string]interface{}{
				"ec2NodeClass": "not-a-map",
			},
			true, "ec2NodeClass must be a JSON object",
		},
		{
			"nodePool not a map",
			map[string]interface{}{
				"nodePool": "not-a-map",
			},
			true, "nodePool must be a JSON object",
		},
		// Reserved tag keys in ec2NodeClass.tags (FIX-014)
		{
			"reserved tag: Name",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"Name": "custom"},
				},
			},
			true, "reserved tag key",
		},
		{
			"reserved tag: Rack",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"Rack": "custom"},
				},
			},
			true, "reserved tag key",
		},
		{
			"reserved tag: name (lowercase)",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"name": "custom"},
				},
			},
			true, "reserved tag key",
		},
		{
			"reserved tag: rack (lowercase)",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"rack": "custom"},
				},
			},
			true, "reserved tag key",
		},
		{
			"reserved tag: RACK (uppercase)",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"RACK": "custom"},
				},
			},
			true, "reserved tag key",
		},
		{
			"valid tag: Environment (non-reserved)",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{
					"tags": map[string]interface{}{"Environment": "production"},
				},
			},
			false, "",
		},
		// Reserved labels in nodePool.template.metadata.labels
		{
			"reserved label: convox.io/nodepool in nodePool labels",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"convox.io/nodepool": "custom",
							},
						},
					},
				},
			},
			true, "Convox-reserved label",
		},
		{
			"valid label in nodePool labels",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"custom-label": "value",
							},
						},
					},
				},
			},
			false, "",
		},
		// Requirements validation
		{
			"blocked: empty requirements array",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"requirements": []interface{}{},
						},
					},
				},
			},
			true, "requirements must not be empty",
		},
		{
			"blocked: requirements not an array",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"requirements": "not-an-array",
						},
					},
				},
			},
			true, "requirements must be a JSON array",
		},
		{
			"valid: non-empty requirements array",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"requirements": []interface{}{
								map[string]interface{}{
									"key": "karpenter.sh/capacity-type", "operator": "In", "values": []interface{}{"on-demand"},
								},
							},
						},
					},
				},
			},
			false, "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.config)
			params := map[string]string{
				"karpenter_config": string(raw),
			}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestKarpenterNodePoolConfigParam_Validate(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name    string
		param   KarpenterNodePoolConfigParam
		wantErr bool
		errMsg  string
	}{
		{
			"minimal valid",
			KarpenterNodePoolConfigParam{Name: "test"},
			false, "",
		},
		{
			"empty name",
			KarpenterNodePoolConfigParam{Name: ""},
			true, "name is required",
		},
		{
			"reserved name: workload",
			KarpenterNodePoolConfigParam{Name: "workload"},
			true, "reserved",
		},
		{
			"reserved name: build",
			KarpenterNodePoolConfigParam{Name: "build"},
			true, "reserved",
		},
		{
			"valid taint key=value:Effect",
			KarpenterNodePoolConfigParam{Name: "test", Taints: strPtr("gpu=true:NoSchedule")},
			false, "",
		},
		{
			"valid taint key:Effect (no value)",
			KarpenterNodePoolConfigParam{Name: "test", Taints: strPtr("dedicated:NoSchedule")},
			false, "",
		},
		{
			"invalid taint effect",
			KarpenterNodePoolConfigParam{Name: "test", Taints: strPtr("key=val:BadEffect")},
			true, "invalid taint effect",
		},
		{
			"valid labels",
			KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("env=prod,team=backend")},
			false, "",
		},
		{
			"invalid label format",
			KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("noequalssign")},
			true, "invalid label",
		},
		{
			"valid weight 0",
			KarpenterNodePoolConfigParam{Name: "test", Weight: intPtr(0)},
			false, "",
		},
		{
			"valid weight 100",
			KarpenterNodePoolConfigParam{Name: "test", Weight: intPtr(100)},
			false, "",
		},
		{
			"invalid weight 101",
			KarpenterNodePoolConfigParam{Name: "test", Weight: intPtr(101)},
			true, "weight must be 0-100",
		},
		{
			"valid budget plain number",
			KarpenterNodePoolConfigParam{Name: "test", DisruptionBudgetNodes: strPtr("3")},
			false, "",
		},
		{
			"valid budget percentage",
			KarpenterNodePoolConfigParam{Name: "test", DisruptionBudgetNodes: strPtr("10%")},
			false, "",
		},
		{
			"invalid budget double percent",
			KarpenterNodePoolConfigParam{Name: "test", DisruptionBudgetNodes: strPtr("10%%")},
			true, "disruption_budget_nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.param.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterDoubleQuoteRejection(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		value   string
		wantErr bool
	}{
		{"label with quote in value", "karpenter_node_labels", `key=val"ue`, true},
		{"label without quote", "karpenter_node_labels", "key=value", false},
		{"taint with quote in value", "karpenter_node_taints", `key=val"ue:NoSchedule`, true},
		{"taint without quote", "karpenter_node_taints", "key=value:NoSchedule", false},
		{"build label with quote", "karpenter_build_node_labels", `key=val"ue`, true},
		{"build label without quote", "karpenter_build_node_labels", "key=value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled": "true",
				tt.param:            tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("%s=%q: got err=%v, wantErr=%v", tt.param, tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestKarpenterNodePoolConfigParam_DoubleQuoteRejection(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name    string
		param   KarpenterNodePoolConfigParam
		wantErr bool
	}{
		{"additional pool label with quote", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr(`key=val"ue`)}, true},
		{"additional pool label clean", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("key=value")}, false},
		{"additional pool taint with quote", KarpenterNodePoolConfigParam{Name: "test", Taints: strPtr(`key=val"ue:NoSchedule`)}, true},
		{"additional pool taint clean", KarpenterNodePoolConfigParam{Name: "test", Taints: strPtr("key=value:NoSchedule")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.param.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterNotAWS(t *testing.T) {
	// Karpenter validation should only run for AWS provider
	params := map[string]string{
		"karpenter_enabled": "true",
	}
	err := validateAndMutateParams(params, "gcp", map[string]string{})
	if err == nil {
		t.Error("expected error for non-AWS karpenter_enabled, got nil")
	}
}

// === Tests for karpenter_enabled gating ===

func TestValidateAndMutateParams_KarpenterEnabledGating(t *testing.T) {
	// Param value validations fire whenever a karpenter param is set,
	// regardless of whether karpenter_enabled is in the same call.
	alwaysValidatedCases := []struct {
		name   string
		params map[string]string
	}{
		{"invalid capacity_types", map[string]string{"karpenter_capacity_types": "invalid"}},
		{"invalid cpu_limit", map[string]string{"karpenter_cpu_limit": "-5"}},
		{"invalid memory_limit_gb", map[string]string{"karpenter_memory_limit_gb": "abc"}},
		{"invalid consolidate_after", map[string]string{"karpenter_consolidate_after": "bad"}},
		{"invalid node_expiry", map[string]string{"karpenter_node_expiry": "bad"}},
		{"invalid budget", map[string]string{"karpenter_disruption_budget_nodes": "abc"}},
		{"invalid taints", map[string]string{"karpenter_node_taints": "nocolon"}},
		{"reserved labels", map[string]string{"karpenter_node_labels": "convox.io/nodepool=x"}},
		{"enabled=false explicit", map[string]string{"karpenter_enabled": "false", "karpenter_capacity_types": "invalid"}},
	}
	for _, tt := range alwaysValidatedCases {
		t.Run("validated/"+tt.name, func(t *testing.T) {
			err := validateAndMutateParams(tt.params, "aws", map[string]string{})
			if err == nil {
				t.Errorf("expected error for invalid param, got nil")
			}
		})
	}

	// karpenter_config is validated OUTSIDE the karpenter_enabled block
	t.Run("karpenter_config still validated without enabled", func(t *testing.T) {
		params := map[string]string{"karpenter_config": "{bad"}
		err := validateAndMutateParams(params, "aws", map[string]string{})
		if err == nil {
			t.Error("expected error: karpenter_config validated even without karpenter_enabled=true")
		}
	})

	t.Run("additional_karpenter_nodepools_config still validated without enabled", func(t *testing.T) {
		params := map[string]string{"additional_karpenter_nodepools_config": "[bad]"}
		err := validateAndMutateParams(params, "aws", map[string]string{})
		if err == nil {
			t.Error("expected error: additional_karpenter_nodepools_config validated even without karpenter_enabled=true")
		}
	})
}

// === Tests for karpenter_arch ===

func TestValidateAndMutateParams_Arch(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid amd64", "amd64", false},
		{"valid arm64", "arm64", false},
		{"valid both", "amd64,arm64", false},
		{"valid with spaces", "amd64, arm64", false},
		{"invalid arch", "x86_64", true},
		{"typo amd65", "amd65", true},
		{"nonsense", "invalid", true},
		{"mixed valid/invalid", "amd64,x86_64", true},
		{"empty string", "", false},
		{"trailing comma", "amd64,", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"karpenter_arch": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("karpenter_arch=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_capacity_types and karpenter_build_capacity_types ===

func TestValidateAndMutateParams_CapacityTypes(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		value   string
		wantErr bool
	}{
		{"valid on-demand", "karpenter_capacity_types", "on-demand", false},
		{"valid spot", "karpenter_capacity_types", "spot", false},
		{"valid both", "karpenter_capacity_types", "on-demand,spot", false},
		{"valid with spaces", "karpenter_capacity_types", "on-demand, spot", false},
		{"invalid type", "karpenter_capacity_types", "reserved", true},
		{"mixed valid/invalid", "karpenter_capacity_types", "on-demand,invalid", true},
		{"empty string", "karpenter_capacity_types", "", false},
		{"case sensitive On-Demand", "karpenter_capacity_types", "On-Demand", true},
		{"trailing comma", "karpenter_capacity_types", "on-demand,", true},
		{"leading comma", "karpenter_capacity_types", ",on-demand", true},

		{"build: valid on-demand", "karpenter_build_capacity_types", "on-demand", false},
		{"build: valid spot", "karpenter_build_capacity_types", "spot", false},
		{"build: valid both", "karpenter_build_capacity_types", "on-demand,spot", false},
		{"build: invalid type", "karpenter_build_capacity_types", "reserved", true},
		{"build: empty string", "karpenter_build_capacity_types", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled": "true",
				tt.param:            tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("%s=%q: got err=%v, wantErr=%v", tt.param, tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_cpu_limit ===

func TestValidateAndMutateParams_CpuLimit(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid 1", "1", false},
		{"valid 100", "100", false},
		{"zero", "0", true},
		{"negative", "-1", true},
		{"not a number", "abc", true},
		{"decimal", "1.5", true},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":   "true",
				"karpenter_cpu_limit": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("cpu_limit=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_build_cpu_limit ===

func TestValidateAndMutateParams_BuildCpuLimit(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid 1", "1", false},
		{"valid 32", "32", false},
		{"zero", "0", true},
		{"negative", "-1", true},
		{"not a number", "abc", true},
		{"decimal", "1.5", true},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":        "true",
				"karpenter_build_cpu_limit": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("build_cpu_limit=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_build_memory_limit_gb ===

func TestValidateAndMutateParams_BuildMemoryLimitGb(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid 1", "1", false},
		{"valid 256", "256", false},
		{"zero", "0", true},
		{"negative", "-1", true},
		{"not a number", "abc", true},
		{"decimal", "1.5", true},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":               "true",
				"karpenter_build_memory_limit_gb": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("build_memory_limit_gb=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_memory_limit_gb ===

func TestValidateAndMutateParams_MemoryLimitGb(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid 1", "1", false},
		{"valid 512", "512", false},
		{"zero", "0", true},
		{"negative", "-1", true},
		{"not a number", "abc", true},
		{"decimal", "1.5", true},
		{"empty string", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":         "true",
				"karpenter_memory_limit_gb": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("memory_limit_gb=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_consolidate_after and karpenter_build_consolidate_after ===

func TestValidateAndMutateParams_ConsolidateAfter(t *testing.T) {
	tests := []struct {
		name    string
		param   string
		value   string
		wantErr bool
	}{
		{"valid 30s", "karpenter_consolidate_after", "30s", false},
		{"valid 5m", "karpenter_consolidate_after", "5m", false},
		{"valid 1h", "karpenter_consolidate_after", "1h", false},
		{"invalid: no unit", "karpenter_consolidate_after", "30", true},
		{"invalid: days", "karpenter_consolidate_after", "1d", true},
		{"invalid: text", "karpenter_consolidate_after", "abc", true},
		{"empty", "karpenter_consolidate_after", "", false},
		{"invalid: space", "karpenter_consolidate_after", "30 s", true},
		{"invalid: decimal", "karpenter_consolidate_after", "1.5h", true},

		{"build: valid 30s", "karpenter_build_consolidate_after", "30s", false},
		{"build: valid 5m", "karpenter_build_consolidate_after", "5m", false},
		{"build: invalid text", "karpenter_build_consolidate_after", "abc", true},
		{"build: empty", "karpenter_build_consolidate_after", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled": "true",
				tt.param:            tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("%s=%q: got err=%v, wantErr=%v", tt.param, tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Tests for karpenter_node_expiry ===

func TestValidateAndMutateParams_NodeExpiry(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid 720h", "720h", false},
		{"valid 1h", "1h", false},
		{"valid Never", "Never", false},
		{"invalid: minutes", "30m", true},
		{"invalid: seconds", "60s", true},
		{"invalid: text", "abc", true},
		{"invalid: never lowercase", "never", true},
		{"empty", "", false},
		{"invalid: no unit", "720", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"karpenter_enabled":     "true",
				"karpenter_node_expiry": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{"karpenter_auth_mode": "true"})
			if (err != nil) != tt.wantErr {
				t.Errorf("node_expiry=%q: got err=%v, wantErr=%v", tt.value, err, tt.wantErr)
			}
		})
	}
}

// === Additional karpenter_config protected field tests ===

func TestValidateAndMutateParams_KarpenterConfigMoreProtectedFields(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			"blocked: ec2NodeClass.instanceProfile",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{"instanceProfile": "custom"},
			},
			true, "ec2NodeClass.instanceProfile is managed by Convox",
		},
		{
			"blocked: ec2NodeClass.securityGroupSelectorTerms",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{"securityGroupSelectorTerms": []interface{}{}},
			},
			true, "ec2NodeClass.securityGroupSelectorTerms is managed by Convox",
		},
		{
			"ec2NodeClass.tags not a JSON object",
			map[string]interface{}{
				"ec2NodeClass": map[string]interface{}{"tags": "not-a-map"},
			},
			true, "ec2NodeClass.tags must be a JSON object",
		},
		{
			"nodePool.template not a map (hard error)",
			map[string]interface{}{
				"nodePool": map[string]interface{}{"template": "not-a-map"},
			},
			true, "nodePool.template must be a JSON object",
		},
		{
			"nodePool.template.spec not a map (hard error)",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"spec": "not-a-map",
					},
				},
			},
			true, "nodePool.template.spec must be a JSON object",
		},
		{
			"nodePool.template.metadata not a map (hard error)",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": "not-a-map",
					},
				},
			},
			true, "nodePool.template.metadata must be a JSON object",
		},
		{
			"nodePool.template.metadata.labels not a map (hard error)",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": "not-a-map",
						},
					},
				},
			},
			true, "nodePool.template.metadata.labels must be a JSON object",
		},
		{
			"nodePool.template.metadata.labels as array (hard error)",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": []interface{}{"a", "b"},
						},
					},
				},
			},
			true, "nodePool.template.metadata.labels must be a JSON object",
		},
		{
			"valid metadata with labels still passes",
			map[string]interface{}{
				"nodePool": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]interface{}{
								"custom-label": "value",
							},
						},
					},
				},
			},
			false, "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.config)
			params := map[string]string{
				"karpenter_config": string(raw),
			}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// === karpenter_config edge cases ===

func TestValidateAndMutateParams_KarpenterConfigEdgeCases(t *testing.T) {
	// JSON array instead of object (base64-encoded to bypass raw-JSON detection)
	t.Run("JSON array instead of object", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte(`[1,2,3]`))
		params := map[string]string{
			"karpenter_config": encoded,
		}
		err := validateAndMutateParams(params, "aws", map[string]string{})
		if err == nil {
			t.Error("expected error for JSON array, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "invalid karpenter_config JSON") {
			t.Errorf("expected JSON parse error, got: %v", err)
		}
	})

	// Exact boundary: 64KB
	t.Run("exactly 64KB", func(t *testing.T) {
		padding := strings.Repeat("x", 64*1024-30) // account for JSON wrapper overhead
		config := `{"nodePool":{"d":"` + padding + `"}}`
		if len(config) <= 64*1024 {
			params := map[string]string{"karpenter_config": config}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if err != nil && strings.Contains(err.Error(), "exceeds maximum size") {
				t.Errorf("config at/under 64KB should not fail size check: %v", err)
			}
		}
	})
}

// === Tests for additional_karpenter_nodepools_config ===

func TestValidateAndMutateParams_AdditionalKarpenterNodepoolsConfig(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{"valid single pool", `[{"name":"gpu"}]`, false, ""},
		{"valid multiple pools", `[{"name":"gpu"},{"name":"high-mem"}]`, false, ""},
		{"empty array", `[]`, false, ""},
		{"missing name", `[{"cpu_limit":4}]`, true, "name is required"},
		{"duplicate names", `[{"name":"gpu"},{"name":"gpu"}]`, true, "duplicate"},
		{"reserved name workload", `[{"name":"workload"}]`, true, "reserved"},
		{"reserved name build", `[{"name":"build"}]`, true, "reserved"},
		{"reserved name default", `[{"name":"default"}]`, true, "reserved"},
		{"reserved name system", `[{"name":"system"}]`, true, "reserved"},
		{"invalid JSON", `[not json]`, true, "invalid karpenter nodepools config"},
		{"invalid pool field", `[{"name":"gpu","cpu_limit":-1}]`, true, "cpu_limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"additional_karpenter_nodepools_config": tt.value,
			}
			err := validateAndMutateParams(params, "aws", map[string]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
			// On success with non-empty input, result should be base64 encoded
			if err == nil && tt.value != "" && tt.value != "[]" {
				decoded, derr := base64.StdEncoding.DecodeString(params["additional_karpenter_nodepools_config"])
				if derr != nil {
					t.Errorf("output not valid base64: %v", derr)
				}
				var out []interface{}
				if uerr := json.Unmarshal(decoded, &out); uerr != nil {
					t.Errorf("decoded output not valid JSON array: %v", uerr)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_AdditionalKarpenterNodepoolsConfigBase64(t *testing.T) {
	raw := `[{"name":"gpu"}]`
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))
	params := map[string]string{
		"additional_karpenter_nodepools_config": encoded,
	}
	err := validateAndMutateParams(params, "aws", map[string]string{})
	if err != nil {
		t.Errorf("expected no error for base64-encoded input, got: %v", err)
	}
	// Verify output is base64
	decoded, derr := base64.StdEncoding.DecodeString(params["additional_karpenter_nodepools_config"])
	if derr != nil {
		t.Fatalf("output not valid base64: %v", derr)
	}
	var out []interface{}
	if uerr := json.Unmarshal(decoded, &out); uerr != nil {
		t.Errorf("decoded output not valid JSON: %v", uerr)
	}
}

// === Non-AWS rejection for various karpenter_ params ===

func TestValidateAndMutateParams_KarpenterNotAWS_AllParams(t *testing.T) {
	karpenterParams := []string{
		"karpenter_enabled",
		"karpenter_capacity_types",
		"karpenter_cpu_limit",
		"karpenter_config",
		"karpenter_node_labels",
		"karpenter_node_taints",
	}
	for _, param := range karpenterParams {
		t.Run(param, func(t *testing.T) {
			params := map[string]string{param: "some-value"}
			err := validateAndMutateParams(params, "gcp", map[string]string{})
			if err == nil {
				t.Errorf("expected error for %s on non-AWS, got nil", param)
			}
			if err != nil && !strings.Contains(err.Error(), "only supported for AWS") {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}

	// additional_karpenter_nodepools_config should also be rejected on non-AWS
	t.Run("additional_karpenter_nodepools_config rejected on non-AWS", func(t *testing.T) {
		params := map[string]string{
			"additional_karpenter_nodepools_config": `[{"name":"gpu"}]`,
		}
		err := validateAndMutateParams(params, "gcp", map[string]string{})
		if err == nil {
			t.Error("expected error for additional_karpenter_nodepools_config on non-AWS, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "only supported for AWS") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

// === Pool name format regex ===

func TestKarpenterNodePoolConfigParam_NameFormat(t *testing.T) {
	tests := []struct {
		name     string
		poolName string
		wantErr  bool
	}{
		{"valid lowercase", "gpu", false},
		{"valid with dash", "high-mem", false},
		{"valid with numbers", "pool1", false},
		{"valid max length 63", strings.Repeat("a", 63), false},
		{"too long 64", strings.Repeat("a", 64), true},
		{"starts with number", "1pool", true},
		{"starts with dash", "-pool", true},
		{"uppercase", "GPU", true},
		{"contains underscore", "my_pool", true},
		{"contains dot", "my.pool", true},
		{"single char", "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := KarpenterNodePoolConfigParam{Name: tt.poolName}
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("name=%q: got err=%v, wantErr=%v", tt.poolName, err, tt.wantErr)
			}
		})
	}
}

// === All reserved pool names ===

func TestKarpenterNodePoolConfigParam_AllReservedNames(t *testing.T) {
	reserved := []string{"workload", "build", "default", "system"}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			p := KarpenterNodePoolConfigParam{Name: name}
			err := p.Validate()
			if err == nil {
				t.Errorf("name=%q should be reserved, got no error", name)
			}
			if err != nil && !strings.Contains(err.Error(), "reserved") {
				t.Errorf("expected 'reserved' in error, got: %v", err)
			}
		})
	}
}

// === Pool-level field validations ===

func TestKarpenterNodePoolConfigParam_PoolFields(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }

	tests := []struct {
		name    string
		param   KarpenterNodePoolConfigParam
		wantErr bool
		errMsg  string
	}{
		// CapacityTypes
		{"capacity: valid on-demand", KarpenterNodePoolConfigParam{Name: "test", CapacityTypes: strPtr("on-demand")}, false, ""},
		{"capacity: valid spot", KarpenterNodePoolConfigParam{Name: "test", CapacityTypes: strPtr("spot")}, false, ""},
		{"capacity: valid both", KarpenterNodePoolConfigParam{Name: "test", CapacityTypes: strPtr("on-demand,spot")}, false, ""},
		{"capacity: invalid", KarpenterNodePoolConfigParam{Name: "test", CapacityTypes: strPtr("reserved")}, true, "invalid capacity type"},

		// Arch
		{"arch: valid amd64", KarpenterNodePoolConfigParam{Name: "test", Arch: strPtr("amd64")}, false, ""},
		{"arch: valid arm64", KarpenterNodePoolConfigParam{Name: "test", Arch: strPtr("arm64")}, false, ""},
		{"arch: valid both", KarpenterNodePoolConfigParam{Name: "test", Arch: strPtr("amd64,arm64")}, false, ""},
		{"arch: invalid", KarpenterNodePoolConfigParam{Name: "test", Arch: strPtr("x86")}, true, "invalid arch"},

		// CpuLimit
		{"cpu: valid", KarpenterNodePoolConfigParam{Name: "test", CpuLimit: intPtr(4)}, false, ""},
		{"cpu: zero", KarpenterNodePoolConfigParam{Name: "test", CpuLimit: intPtr(0)}, true, "cpu_limit must be positive"},
		{"cpu: negative", KarpenterNodePoolConfigParam{Name: "test", CpuLimit: intPtr(-1)}, true, "cpu_limit must be positive"},

		// MemoryLimitGb
		{"memory: valid", KarpenterNodePoolConfigParam{Name: "test", MemoryLimitGb: intPtr(16)}, false, ""},
		{"memory: zero", KarpenterNodePoolConfigParam{Name: "test", MemoryLimitGb: intPtr(0)}, true, "memory_limit_gb must be positive"},
		{"memory: negative", KarpenterNodePoolConfigParam{Name: "test", MemoryLimitGb: intPtr(-1)}, true, "memory_limit_gb must be positive"},

		// ConsolidateAfter
		{"consolidate: valid 30s", KarpenterNodePoolConfigParam{Name: "test", ConsolidateAfter: strPtr("30s")}, false, ""},
		{"consolidate: valid 5m", KarpenterNodePoolConfigParam{Name: "test", ConsolidateAfter: strPtr("5m")}, false, ""},
		{"consolidate: valid 1h", KarpenterNodePoolConfigParam{Name: "test", ConsolidateAfter: strPtr("1h")}, false, ""},
		{"consolidate: invalid", KarpenterNodePoolConfigParam{Name: "test", ConsolidateAfter: strPtr("abc")}, true, "consolidate_after"},

		// NodeExpiry
		{"expiry: valid 720h", KarpenterNodePoolConfigParam{Name: "test", NodeExpiry: strPtr("720h")}, false, ""},
		{"expiry: valid Never", KarpenterNodePoolConfigParam{Name: "test", NodeExpiry: strPtr("Never")}, false, ""},
		{"expiry: invalid 30m", KarpenterNodePoolConfigParam{Name: "test", NodeExpiry: strPtr("30m")}, true, "node_expiry"},

		// ConsolidationPolicy
		{"policy: WhenEmpty", KarpenterNodePoolConfigParam{Name: "test", ConsolidationPolicy: strPtr("WhenEmpty")}, false, ""},
		{"policy: WhenEmptyOrUnderutilized", KarpenterNodePoolConfigParam{Name: "test", ConsolidationPolicy: strPtr("WhenEmptyOrUnderutilized")}, false, ""},
		{"policy: invalid", KarpenterNodePoolConfigParam{Name: "test", ConsolidationPolicy: strPtr("Always")}, true, "consolidation_policy"},

		// VolumeType
		{"volume: gp2", KarpenterNodePoolConfigParam{Name: "test", VolumeType: strPtr("gp2")}, false, ""},
		{"volume: gp3", KarpenterNodePoolConfigParam{Name: "test", VolumeType: strPtr("gp3")}, false, ""},
		{"volume: io1", KarpenterNodePoolConfigParam{Name: "test", VolumeType: strPtr("io1")}, false, ""},
		{"volume: io2", KarpenterNodePoolConfigParam{Name: "test", VolumeType: strPtr("io2")}, false, ""},
		{"volume: invalid", KarpenterNodePoolConfigParam{Name: "test", VolumeType: strPtr("standard")}, true, "volume_type"},

		// Disk
		{"disk: valid", KarpenterNodePoolConfigParam{Name: "test", Disk: intPtr(20)}, false, ""},
		{"disk: zero", KarpenterNodePoolConfigParam{Name: "test", Disk: intPtr(0)}, false, ""},
		{"disk: negative", KarpenterNodePoolConfigParam{Name: "test", Disk: intPtr(-1)}, true, "disk must be non-negative"},

		// Weight (supplementing existing tests)
		{"weight: negative", KarpenterNodePoolConfigParam{Name: "test", Weight: intPtr(-1)}, true, "weight must be 0-100"},

		// Labels edge cases
		{"labels: empty key", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("=value")}, true, "invalid label"},
		{"labels: empty value", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("key=")}, true, "invalid label"},
		{"labels: reserved convox.io/nodepool", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("convox.io/nodepool=custom")}, true, "Convox-reserved label"},
		{"labels: reserved among others", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("ok=fine,convox.io/nodepool=x")}, true, "Convox-reserved label"},
		{"labels: non-reserved ok", KarpenterNodePoolConfigParam{Name: "test", Labels: strPtr("team=backend,env=prod")}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.param.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// === KarpenterNodePools slice validation ===

func TestKarpenterNodePools_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pools   KarpenterNodePools
		wantErr bool
		errMsg  string
	}{
		{"empty slice", KarpenterNodePools{}, false, ""},
		{"single valid", KarpenterNodePools{{Name: "gpu"}}, false, ""},
		{"multiple valid", KarpenterNodePools{{Name: "gpu"}, {Name: "high-mem"}}, false, ""},
		{"duplicate names", KarpenterNodePools{{Name: "gpu"}, {Name: "gpu"}}, true, "duplicate"},
		{"one invalid", KarpenterNodePools{{Name: ""}}, true, "name is required"},
		{"valid then invalid", KarpenterNodePools{{Name: "gpu"}, {Name: ""}}, true, "name is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pools.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// === Tests for karpenter_auth_mode one-way lock and dependency ===

func TestValidateAndMutateParams_KarpenterAuthMode(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]string
		currentParams map[string]string
		wantErr       bool
		errMsg        string
	}{
		{
			"disabling karpenter_auth_mode once enabled is a hard error",
			map[string]string{"karpenter_auth_mode": "false"},
			map[string]string{"karpenter_auth_mode": "true"},
			true,
			"karpenter_auth_mode cannot be disabled once enabled",
		},
		{
			"enabling karpenter_enabled without karpenter_auth_mode anywhere is an error",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{},
			true,
			"karpenter_enabled=true requires karpenter_auth_mode=true",
		},
		{
			"enabling karpenter_enabled when current karpenter_auth_mode is false is an error",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "false"},
			true,
			"karpenter_enabled=true requires karpenter_auth_mode=true",
		},
		{
			"both karpenter_auth_mode=true and karpenter_enabled=true in same call is allowed",
			map[string]string{"karpenter_auth_mode": "true", "karpenter_enabled": "true"},
			map[string]string{},
			false,
			"",
		},
		{
			"karpenter_enabled=true with karpenter_auth_mode=true in same call succeeds when auth_mode already applied",
			map[string]string{"karpenter_auth_mode": "true", "karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true"},
			false,
			"",
		},
		{
			"karpenter_enabled=true when current karpenter_auth_mode is already true succeeds",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true"},
			false,
			"",
		},
		{
			"setting karpenter_auth_mode=true from false is allowed",
			map[string]string{"karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "false"},
			false,
			"",
		},
		{
			"setting karpenter_auth_mode=true when not previously set is allowed",
			map[string]string{"karpenter_auth_mode": "true"},
			map[string]string{},
			false,
			"",
		},
		{
			"karpenter_auth_mode=true is idempotent (already true, setting true again)",
			map[string]string{"karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "true"},
			false,
			"",
		},
		{
			"non-AWS rejection takes precedence over auth_mode check",
			map[string]string{"karpenter_auth_mode": "true"},
			map[string]string{},
			true,
			"only supported for AWS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := "aws"
			// The non-AWS test case
			if tt.name == "non-AWS rejection takes precedence over auth_mode check" {
				provider = "gcp"
			}
			err := validateAndMutateParams(tt.params, provider, tt.currentParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterNonDedicatedNodeGroups(t *testing.T) {
	dedicatedTrue := `[{"type":"m5.large","dedicated":true,"label":"gpu"}]`
	dedicatedFalse := `[{"type":"m5.large","dedicated":false}]`
	noDedicatedField := `[{"type":"m5.large"}]`
	mixedGroups := `[{"type":"m5.large","dedicated":true,"label":"gpu"},{"type":"c5.xlarge"}]`
	dedicatedTrueB64 := base64.StdEncoding.EncodeToString([]byte(dedicatedTrue))
	dedicatedFalseB64 := base64.StdEncoding.EncodeToString([]byte(dedicatedFalse))
	noDedicatedFieldB64 := base64.StdEncoding.EncodeToString([]byte(noDedicatedField))

	tests := []struct {
		name          string
		params        map[string]string
		currentParams map[string]string
		wantErr       bool
		errMsg        string
	}{
		{
			"enabling karpenter with non-dedicated current node groups is an error",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true", "additional_node_groups_config": dedicatedFalseB64},
			true,
			"dedicated=true",
		},
		{
			"enabling karpenter with dedicated current node groups succeeds",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true", "additional_node_groups_config": dedicatedTrueB64},
			false,
			"",
		},
		{
			"enabling karpenter with node groups missing dedicated field is an error",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true", "additional_node_groups_config": noDedicatedFieldB64},
			true,
			"dedicated=true",
		},
		{
			"enabling karpenter with dedicated=true in new config succeeds",
			map[string]string{
				"karpenter_enabled":            "true",
				"additional_node_groups_config": dedicatedTrue,
			},
			map[string]string{"karpenter_auth_mode": "true"},
			false,
			"",
		},
		{
			"enabling karpenter with dedicated=false in new config is an error",
			map[string]string{
				"karpenter_enabled":            "true",
				"additional_node_groups_config": dedicatedFalse,
			},
			map[string]string{"karpenter_auth_mode": "true"},
			true,
			"dedicated=true",
		},
		{
			"new config overrides current params (new config has dedicated=true)",
			map[string]string{
				"karpenter_enabled":            "true",
				"additional_node_groups_config": dedicatedTrue,
			},
			map[string]string{"karpenter_auth_mode": "true", "additional_node_groups_config": noDedicatedFieldB64},
			false,
			"",
		},
		{
			"mixed groups where one lacks dedicated is an error",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true", "additional_node_groups_config": base64.StdEncoding.EncodeToString([]byte(mixedGroups))},
			true,
			"dedicated=true",
		},
		{
			"no additional node groups with karpenter succeeds",
			map[string]string{"karpenter_enabled": "true"},
			map[string]string{"karpenter_auth_mode": "true"},
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAndMutateParams(tt.params, "aws", tt.currentParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterReenableValidation(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string]string
		currentParams map[string]string
		wantErr       bool
		errMsg        string
	}{
		{
			"stale invalid capacity_types in currentParams caught on re-enable",
			map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "true", "karpenter_capacity_types": "INVALID"},
			true,
			"invalid karpenter capacity type",
		},
		{
			"stale invalid cpu_limit in currentParams caught on re-enable",
			map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "true", "karpenter_cpu_limit": "-5"},
			true,
			"karpenter_cpu_limit must be a positive integer",
		},
		{
			"valid currentParams pass re-enable validation",
			map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "true", "karpenter_capacity_types": "on-demand"},
			false,
			"",
		},
		{
			"explicit param in call overrides stale currentParam",
			map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true", "karpenter_capacity_types": "spot"},
			map[string]string{"karpenter_auth_mode": "true", "karpenter_capacity_types": "INVALID"},
			false,
			"",
		},
		{
			"injected keys are not sent to params after validation",
			map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true"},
			map[string]string{"karpenter_auth_mode": "true", "karpenter_capacity_types": "on-demand"},
			false,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy params so we can check for cleanup
			paramsCopy := make(map[string]string)
			for k, v := range tt.params {
				paramsCopy[k] = v
			}
			err := validateAndMutateParams(paramsCopy, "aws", tt.currentParams)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}

	// Verify injected keys are cleaned up and NOT present in params after validation
	t.Run("injected keys cleaned up from params", func(t *testing.T) {
		params := map[string]string{"karpenter_enabled": "true", "karpenter_auth_mode": "true"}
		currentParams := map[string]string{"karpenter_auth_mode": "true", "karpenter_capacity_types": "on-demand", "karpenter_cpu_limit": "100"}
		err := validateAndMutateParams(params, "aws", currentParams)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := params["karpenter_capacity_types"]; exists {
			t.Error("injected key karpenter_capacity_types should have been removed from params after validation")
		}
		if _, exists := params["karpenter_cpu_limit"]; exists {
			t.Error("injected key karpenter_cpu_limit should have been removed from params after validation")
		}
	})
}

func TestCheckRackNameRegex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{"valid short name", "myrack", false, ""},
		{"valid with numbers", "rack123", false, ""},
		{"valid with hyphens", "my-rack-1", false, ""},
		{"valid 26 chars", "abcdefghijklmnopqrstuvwxyz", false, ""},
		{"too long 27 chars", "abcdefghijklmnopqrstuvwxyza", true, "26 characters"},
		{"starts with digit", "1rack", true, "must start with a lowercase letter"},
		{"starts with hyphen", "-rack", true, "must start with a lowercase letter"},
		{"uppercase letters", "MyRack", true, "must start with a lowercase letter"},
		{"special characters", "my_rack", true, "must start with a lowercase letter"},
		{"empty string", "", true, "must start with a lowercase letter"},
		{"single char", "a", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkRackNameRegex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("name=%q: got err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("name=%q: error %q does not contain %q", tt.input, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestAdditionalNodeGroups_Validate_AutoAssignIdsFromZero(t *testing.T) {
	ngs := AdditionalNodeGroups{
		{Type: "m5.large"},
		{Type: "c5.xlarge"},
	}
	if err := ngs.Validate(); err != nil {
		t.Fatal(err)
	}
	if ngs[0].Id == nil || *ngs[0].Id != 0 {
		t.Errorf("expected first id=0, got %v", ngs[0].Id)
	}
	if ngs[1].Id == nil || *ngs[1].Id != 1 {
		t.Errorf("expected second id=1, got %v", ngs[1].Id)
	}
}

func TestValidateAndMutateParams_PreserveExistingNodeGroupIds(t *testing.T) {
	id5 := 5
	lbl := "gpu"
	existingWithId := AdditionalNodeGroups{{Type: "g6.xlarge", Id: &id5, Dedicated: boolPtr(true), Label: &lbl}}
	existingWithIdJSON, _ := json.Marshal(existingWithId)
	existingWithIdB64 := base64.StdEncoding.EncodeToString(existingWithIdJSON)

	existingNoId := `[{"type":"g6.xlarge","dedicated":true,"label":"gpu"}]`
	existingNoIdB64 := base64.StdEncoding.EncodeToString([]byte(existingNoId))

	tests := []struct {
		name          string
		newConfig     string
		currentConfig string
		wantId        int
	}{
		{
			"preserves existing id from currentParams",
			`[{"type":"g6.xlarge","dedicated":true,"label":"gpu"}]`,
			existingWithIdB64,
			5,
		},
		{
			"legacy config without id gets 0-based index",
			`[{"type":"g6.xlarge","dedicated":true,"label":"gpu"}]`,
			existingNoIdB64,
			0,
		},
		{
			"no currentParams — auto-assigns 0-based",
			`[{"type":"g6.xlarge","dedicated":true,"label":"gpu"}]`,
			"",
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{
				"additional_node_groups_config": tt.newConfig,
			}
			currentParams := map[string]string{}
			if tt.currentConfig != "" {
				currentParams["additional_node_groups_config"] = tt.currentConfig
			}

			if err := validateAndMutateParams(params, "aws", currentParams); err != nil {
				t.Fatal(err)
			}

			// Decode the result
			decoded, err := base64.StdEncoding.DecodeString(params["additional_node_groups_config"])
			if err != nil {
				t.Fatal(err)
			}
			var result AdditionalNodeGroups
			if err := json.Unmarshal(decoded, &result); err != nil {
				t.Fatal(err)
			}
			if len(result) != 1 {
				t.Fatalf("expected 1 node group, got %d", len(result))
			}
			if result[0].Id == nil {
				t.Fatal("expected non-nil id")
			}
			if *result[0].Id != tt.wantId {
				t.Errorf("expected id=%d, got id=%d", tt.wantId, *result[0].Id)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
