package cli

import (
	"strings"
	"testing"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"karpenter_enabled", "karpenter_enbled", 1},
		{"node_type", "node_tyoe", 1},
		{"tags", "tgas", 2},
		{"idle_timeout", "banana", 12},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSuggestParam(t *testing.T) {
	known := map[string]bool{
		"karpenter_enabled": true,
		"node_type":         true,
		"idle_timeout":      true,
		"tags":              true,
	}
	tests := []struct {
		key  string
		want string
	}{
		{"karpenter_enbled", "karpenter_enabled"},
		{"node_tyoe", "node_type"},
		{"tgas", "tags"},
		{"banana", ""},
		{"idle_timeoutt", "idle_timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := suggestParam(tt.key, known)
			if got != tt.want {
				t.Errorf("suggestParam(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestValidateAndMutateParams_V2RackSkipsValidation(t *testing.T) {
	v2CurrentParams := map[string]string{
		"HighAvailability": "true",
		"Private":          "false",
		"BuildMemory":      "2048",
		"InstanceType":     "t3.medium",
	}
	params := map[string]string{"BuildMemory": "4096"}
	err := validateAndMutateParams(params, "aws", v2CurrentParams, false)
	if err != nil {
		t.Errorf("V2 rack param should pass, got: %v", err)
	}

	params2 := map[string]string{"Release": "20260412"}
	err2 := validateAndMutateParams(params2, "aws", v2CurrentParams, false)
	if err2 != nil {
		t.Errorf("V2 managed-equivalent param should pass, got: %v", err2)
	}

	v3CurrentParams := map[string]string{
		"high_availability": "true",
		"idle_timeout":      "3600",
	}
	params3 := map[string]string{"banana": "value"}
	err3 := validateAndMutateParams(params3, "aws", v3CurrentParams, false)
	if err3 == nil {
		t.Fatal("V3 rack should still reject unknown params")
	}
}

func TestValidateAndMutateParams_ManagedParam(t *testing.T) {
	params := map[string]string{"image": "custom"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for managed param without --force")
	}
	if !strings.Contains(err.Error(), "managed internally") {
		t.Errorf("error %q should mention 'managed internally'", err.Error())
	}

	params2 := map[string]string{"image": "custom"}
	err2 := validateAndMutateParams(params2, "aws", map[string]string{}, true)
	if err2 != nil {
		t.Errorf("--force should bypass managed guard, got: %v", err2)
	}
}

func TestValidateAndMutateParams_UnknownParam(t *testing.T) {
	params := map[string]string{"karpenter_enbled": "true"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for unknown param")
	}
	if !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("error %q should mention 'unknown parameter'", err.Error())
	}
	if !strings.Contains(err.Error(), "karpenter_enabled") {
		t.Errorf("error %q should suggest 'karpenter_enabled'", err.Error())
	}

	params2 := map[string]string{"totally_fake": "value"}
	err2 := validateAndMutateParams(params2, "aws", map[string]string{}, true)
	if err2 != nil {
		t.Errorf("--force should bypass known-key check, got: %v", err2)
	}
}

func TestValidateAndMutateParams_UnknownParamNoSuggestion(t *testing.T) {
	params := map[string]string{"zzzzzzzzz": "value"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for unknown param")
	}
	if strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("error should NOT have suggestion for distant match")
	}
	if !strings.Contains(err.Error(), "sudo convox update") {
		t.Errorf("error should mention 'sudo convox update'")
	}
}

func TestValidateAndMutateParams_EmptyKey(t *testing.T) {
	params := map[string]string{"": "value"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "parameter name cannot be empty") {
		t.Errorf("error %q should mention empty parameter name", err.Error())
	}
}

func TestValidateAndMutateParams_SyncTfNow(t *testing.T) {
	params := map[string]string{"sync_tf_now": "1"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err != nil {
		t.Errorf("sync_tf_now should be accepted, got: %v", err)
	}
}

func TestValidateAndMutateParams_UnknownProvider(t *testing.T) {
	params := map[string]string{"anything": "value"}
	err := validateAndMutateParams(params, "", map[string]string{}, false)
	if err != nil {
		t.Errorf("empty provider should skip key check, got: %v", err)
	}
}

func TestValidateAndMutateParams_KarpenterOnNonAWS_SkipsSpellcheck(t *testing.T) {
	params := map[string]string{"karpenter_enabled": "true"}
	err := validateAndMutateParams(params, "gcp", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error for karpenter on GCP")
	}
	if !strings.Contains(err.Error(), "only supported for AWS") {
		t.Errorf("error %q should say 'only supported for AWS', not 'unknown parameter'", err.Error())
	}
}

func TestValidateAndMutateParams_ManagedParamOnWrongProvider(t *testing.T) {
	params := map[string]string{"k8s_version": "1.30"}
	err := validateAndMutateParams(params, "metal", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "managed internally") {
		t.Errorf("metal has no k8s_version — should get 'unknown parameter', not 'managed internally'")
	}
}

func TestValidateAndMutateParams_DOInstallOnly(t *testing.T) {
	for _, param := range []string{"access_id", "secret_key", "token"} {
		params := map[string]string{param: "value"}
		err := validateAndMutateParams(params, "do", map[string]string{}, false)
		if err == nil {
			t.Errorf("DO credential %s should be install-only", param)
		}
		if err != nil && !strings.Contains(err.Error(), "can only be set during rack installation") {
			t.Errorf("error for %s: %q should mention install-only", param, err.Error())
		}
	}
}

func TestValidateAndMutateParams_ImdsHttpTokens(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{"optional is valid", "optional", false, ""},
		{"required is valid", "required", false, ""},
		{"junk rejected", "banana", true, "must be 'optional' or 'required'"},
		{"OPTIONAL rejected (case-sensitive)", "OPTIONAL", true, "must be 'optional' or 'required'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"imds_http_tokens": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateAndMutateParams_NodeCapacityType(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"on_demand valid", "on_demand", false},
		{"spot valid", "spot", false},
		{"mixed valid", "mixed", false},
		{"ON_DEMAND valid (case-insensitive)", "ON_DEMAND", false},
		{"MIXED valid (case-insensitive)", "MIXED", false},
		{"junk rejected", "banana", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"node_capacity_type": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAndMutateParams_AccessLogRetention(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"7 valid", "7", false},
		{"30 valid", "30", false},
		{"junk rejected", "abc", true},
		{"float rejected", "7.5", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"access_log_retention_in_days": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateAndMutateParams_KarpenterNodeVolumeType(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"gp2 valid", "gp2", false},
		{"gp3 valid", "gp3", false},
		{"io1 valid", "io1", false},
		{"io2 valid", "io2", false},
		{"GP3 rejected (case-sensitive)", "GP3", true},
		{"st1 rejected", "st1", true},
		{"junk rejected", "banana", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"karpenter_node_volume_type": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
