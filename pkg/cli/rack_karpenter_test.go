package cli

import (
	"encoding/base64"
	"encoding/json"
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
			err := validateAndMutateParams(params, "aws")
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
			err := validateAndMutateParams(params, "aws")
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
			err := validateAndMutateParams(params, "aws")
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
			err := validateAndMutateParams(params, "aws")
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.config)
			params := map[string]string{
				"karpenter_config": string(raw),
			}
			err := validateAndMutateParams(params, "aws")
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
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
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterNotAWS(t *testing.T) {
	// Karpenter validation should only run for AWS provider
	params := map[string]string{
		"karpenter_enabled": "true",
	}
	err := validateAndMutateParams(params, "gcp")
	if err == nil {
		t.Error("expected error for non-AWS karpenter_enabled, got nil")
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
