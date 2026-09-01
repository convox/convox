package cli

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return string(out)
}

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

func TestValidateAndMutateParams_KarpenterNodeOS(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		want    string
	}{
		{"al2023 valid", "al2023", false, "al2023"},
		{"bottlerocket valid", "bottlerocket", false, "bottlerocket"},
		{"BOTTLEROCKET normalized", "BOTTLEROCKET", false, "bottlerocket"},
		{"AL2023 normalized", "AL2023", false, "al2023"},
		{"junk rejected", "ubuntu", true, ""},
		{"empty rejected", "", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"karpenter_node_os": tt.value}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.value, err)
			}
			if got := params["karpenter_node_os"]; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateAndMutateParams_WebhookSigningKey_AllProviders(t *testing.T) {
	for _, provider := range []string{"aws", "gcp", "azure", "do", "metal", "local"} {
		t.Run(provider, func(t *testing.T) {
			params := map[string]string{"webhook_signing_key": "deadbeefcafe"}
			err := validateAndMutateParams(params, provider, map[string]string{}, false)
			if err != nil {
				t.Errorf("webhook_signing_key should pass on %s, got: %v", provider, err)
			}
		})
	}
}

func TestValidateAndMutateParams_PrometheusUrl_AwsAccepted(t *testing.T) {
	// Use the in-cluster suffix so the test does not depend on a live
	// resolver — the SSRF guard short-circuits *.svc.cluster.local
	// hostnames before the DNS lookup. End-to-end DNS-resolution
	// behaviour is exercised in pkg/validator with a stubbed resolver.
	params := map[string]string{"prometheus_url": "http://prom.kube-system.svc.cluster.local:9090"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err != nil {
		t.Errorf("prometheus_url should pass on aws, got: %v", err)
	}
}

func TestValidateAndMutateParams_PrometheusUrl_NonAwsRejected(t *testing.T) {
	for _, provider := range []string{"gcp", "azure", "do", "metal", "local"} {
		t.Run(provider, func(t *testing.T) {
			params := map[string]string{"prometheus_url": "http://prom.kube-system.svc.cluster.local:9090"}
			err := validateAndMutateParams(params, provider, map[string]string{}, false)
			if err == nil {
				t.Fatalf("prometheus_url should be rejected on %s (only declared in AWS Terraform)", provider)
			}
			if !strings.Contains(err.Error(), "unknown parameter") {
				t.Errorf("error %q should mention 'unknown parameter'", err.Error())
			}
		})
	}
}

func TestValidateAndMutateParams_NetworkPolicyEnable_NonAwsRejected(t *testing.T) {
	for _, provider := range []string{"gcp", "azure", "do", "metal", "local"} {
		t.Run(provider, func(t *testing.T) {
			params := map[string]string{"network_policy_enable": "true"}
			err := validateAndMutateParams(params, provider, map[string]string{}, false)
			if err == nil {
				t.Fatalf("network_policy_enable should be rejected on %s (only declared in AWS Terraform)", provider)
			}
			if !strings.Contains(err.Error(), "unknown parameter") {
				t.Errorf("error %q should mention 'unknown parameter'", err.Error())
			}
		})
	}
}

func TestValidateAndMutateParams_UnknownParamSpellcheckIntact(t *testing.T) {
	// Regression guard: adding entries to KnownParams maps must not weaken
	// the spellcheck path that rejects unknown keys.
	params := map[string]string{"foo_bar_baz": "value"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("expected unknown-parameter error for 'foo_bar_baz'")
	}
	if !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("error %q should mention 'unknown parameter'", err.Error())
	}
}

func TestValidateAndMutateParams_BoolParam_AcceptsParseBoolForms(t *testing.T) {
	for _, v := range []string{"true", "false", "1", "0", "t", "f", "T", "F", "True", "False", "TRUE", "FALSE"} {
		t.Run(v, func(t *testing.T) {
			params := map[string]string{"cost_tracking_enable": v}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if err != nil {
				t.Errorf("cost_tracking_enable=%q should pass strconv.ParseBool, got: %v", v, err)
			}
		})
	}
}

func TestValidateAndMutateParams_BoolParam_RejectsNonCanonical(t *testing.T) {
	for _, v := range []string{"invalid", "yes", "no", "on", "off", "2", "TrUe"} {
		t.Run(v, func(t *testing.T) {
			params := map[string]string{"cost_tracking_enable": v}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if err == nil {
				t.Fatalf("cost_tracking_enable=%q should be rejected", v)
			}
			if !strings.Contains(err.Error(), "must be 'true' or 'false'") {
				t.Errorf("error %q should mention \"must be 'true' or 'false'\"", err.Error())
			}
			if !strings.Contains(err.Error(), v) {
				t.Errorf("error %q should include offending value %q", err.Error(), v)
			}
		})
	}
}

func TestValidateAndMutateParams_BoolParam_EmptySkipsSweep(t *testing.T) {
	// Empty values bypass the bool-sweep — they fall through to existing
	// empty-string rules below the sweep. The sweep itself must not produce
	// "must be 'true' or 'false'" for empty input.
	params := map[string]string{"cost_tracking_enable": ""}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err != nil && strings.Contains(err.Error(), "must be 'true' or 'false'") {
		t.Errorf("empty cost_tracking_enable hit bool sweep instead of empty-string rule; err: %v", err)
	}
}

func TestValidateAndMutateParams_BoolParam_AwsCoverage(t *testing.T) {
	// Every AWS-listed boolParam accepts canonical 'true' and rejects garbage.
	// ecr_docker_hub_cache=true triggers a dependency rule (requires
	// docker_hub_username/password); it's covered by a dedicated test below.
	for _, k := range []string{
		"build_node_enabled", "build_node_minimal_role_enabled", "buildkit_host_path_cache_enable",
		"convox_domain_tls_cert_disable",
		"cost_tracking_enable", "deploy_extra_nlb", "disable_convox_resolver",
		"disable_image_manifest_cache", "ebs_volume_encryption_enabled",
		"ecr_immutable_tags_enabled", "ecr_scan_on_push_enable", "efs_csi_driver_enable", "fluentd_disable",
		"gpu_tag_enable", "imds_tags_enable", "internal_router", "contour_internal_tls",
		"karpenter_consolidation_enabled", "keda_enable", "network_policy_enable",
		"pod_identity_agent_enable",
		"pod_imds_block_enabled", "seccomp_default_enabled", "system_readonly_rootfs_enabled",
		"telemetry", "vpa_enable",
	} {
		t.Run(k, func(t *testing.T) {
			params := map[string]string{k: "true"}
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if err != nil {
				t.Errorf("%s=true should pass, got: %v", k, err)
			}
			params2 := map[string]string{k: "garbage"}
			err2 := validateAndMutateParams(params2, "aws", map[string]string{}, false)
			if err2 == nil {
				t.Errorf("%s=garbage should be rejected", k)
			}
			if err2 != nil && !strings.Contains(err2.Error(), "must be 'true' or 'false'") {
				t.Errorf("%s=garbage error %q should mention \"must be 'true' or 'false'\"", k, err2.Error())
			}
		})
	}
}

func TestValidateAndMutateParams_CostTrackingEnable_AzureCoverage(t *testing.T) {
	params := map[string]string{"cost_tracking_enable": "true"}
	if err := validateAndMutateParams(params, "azure", map[string]string{}, false); err != nil {
		t.Errorf("cost_tracking_enable=true should pass for azure, got: %v", err)
	}
	params2 := map[string]string{"cost_tracking_enable": "garbage"}
	if err := validateAndMutateParams(params2, "azure", map[string]string{}, false); err == nil {
		t.Errorf("cost_tracking_enable=garbage should be rejected for azure")
	}
	params3 := map[string]string{"cost_tracking_enable": "true"}
	if err := validateAndMutateParams(params3, "do", map[string]string{}, false); err == nil {
		t.Errorf("cost_tracking_enable should stay unknown for do provider")
	}
	params4 := map[string]string{"cost_tracking_enable": "true"}
	if err := validateAndMutateParams(params4, "gcp", map[string]string{}, false); err == nil {
		t.Errorf("cost_tracking_enable should stay unknown for gcp provider")
	}
}

func TestValidateAndMutateParams_BoolParam_EcrDockerHubCacheDepsAndType(t *testing.T) {
	// ecr_docker_hub_cache=garbage is rejected by the bool sweep AFTER the
	// dependency check; with deps satisfied, =true passes and =garbage
	// rejects with the bool-sweep message.
	deps := map[string]string{
		"ecr_docker_hub_cache": "true",
		"docker_hub_username":  "u",
		"docker_hub_password":  "p",
	}
	if err := validateAndMutateParams(deps, "aws", map[string]string{}, false); err != nil {
		t.Errorf("ecr_docker_hub_cache=true with deps should pass, got: %v", err)
	}
	depsBad := map[string]string{
		"ecr_docker_hub_cache": "garbage",
		"docker_hub_username":  "u",
		"docker_hub_password":  "p",
	}
	err := validateAndMutateParams(depsBad, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("ecr_docker_hub_cache=garbage should be rejected")
	}
	if !strings.Contains(err.Error(), "must be 'true' or 'false'") {
		t.Errorf("error %q should mention \"must be 'true' or 'false'\"", err.Error())
	}
}

func TestValidateAndMutateParams_BoolParam_AzureFilesEnable(t *testing.T) {
	// azure_files_enable is azure-only-bool; rejected on aws via spellcheck.
	params := map[string]string{"azure_files_enable": "true"}
	err := validateAndMutateParams(params, "azure", map[string]string{}, false)
	if err != nil {
		t.Errorf("azure_files_enable=true should pass on azure, got: %v", err)
	}
	params2 := map[string]string{"azure_files_enable": "garbage"}
	err2 := validateAndMutateParams(params2, "azure", map[string]string{}, false)
	if err2 == nil {
		t.Fatal("azure_files_enable=garbage should be rejected on azure")
	}
	if !strings.Contains(err2.Error(), "must be 'true' or 'false'") {
		t.Errorf("error %q should mention \"must be 'true' or 'false'\"", err2.Error())
	}
}

func TestValidateAndMutateParams_BoolParam_KarpenterEnabledStillUsesExistingValidator(t *testing.T) {
	// karpenter_enabled is type=string in aws/system but has its own
	// validation block above the bool sweep. Verify the existing message
	// (without the (got %q) suffix) is still produced — i.e., bool sweep
	// did not absorb karpenter_enabled.
	params := map[string]string{"karpenter_enabled": "garbage"}
	err := validateAndMutateParams(params, "aws", map[string]string{}, false)
	if err == nil {
		t.Fatal("karpenter_enabled=garbage should be rejected")
	}
	if !strings.Contains(err.Error(), "must be 'true' or 'false'") {
		t.Errorf("error %q should mention \"must be 'true' or 'false'\"", err.Error())
	}
	if strings.Contains(err.Error(), "(got") {
		t.Errorf("karpenter_enabled error %q should be the existing validator's message, not the bool sweep's", err.Error())
	}
}

// TestValidateRackParams_GPUObservability_RequiresDevicePlugin asserts the
// cross-validation rule that gpu_observability_enable=true requires
// nvidia_device_plugin_enable=true (set in the same call OR already enabled
// on the rack). The DCGM exporter relies on the device plugin's
// /var/lib/kubelet/pod-resources/ socket for pod->GPU attribution; without
// the plugin the exporter pods schedule but emit metrics with no pod labels.
func TestValidateRackParams_GPUObservability_RequiresDevicePlugin(t *testing.T) {
	t.Run("rejects when device plugin is off and not being enabled", func(t *testing.T) {
		params := map[string]string{"gpu_observability_enable": "true"}
		current := map[string]string{"nvidia_device_plugin_enable": "false"}
		err := validateAndMutateParams(params, "aws", current, false)
		if err == nil {
			t.Fatal("gpu_observability_enable=true with device plugin off should be rejected")
		}
		if !strings.Contains(err.Error(), "requires nvidia_device_plugin_enable=true") {
			t.Errorf("error %q should mention 'requires nvidia_device_plugin_enable=true'", err.Error())
		}
	})

	t.Run("accepts when same-call enables both", func(t *testing.T) {
		params := map[string]string{
			"gpu_observability_enable":    "true",
			"nvidia_device_plugin_enable": "true",
		}
		err := validateAndMutateParams(params, "aws", map[string]string{}, false)
		if err != nil {
			t.Errorf("setting both gpu_observability_enable=true and nvidia_device_plugin_enable=true in one call should pass, got: %v", err)
		}
	})

	t.Run("accepts when device plugin already enabled on rack", func(t *testing.T) {
		params := map[string]string{"gpu_observability_enable": "true"}
		current := map[string]string{"nvidia_device_plugin_enable": "true"}
		err := validateAndMutateParams(params, "aws", current, false)
		if err != nil {
			t.Errorf("gpu_observability_enable=true with device plugin already on should pass, got: %v", err)
		}
	})

	t.Run("accepts when gpu_observability_enable already true (idempotent re-set)", func(t *testing.T) {
		// If the rack already has gpu_observability_enable=true, re-setting it does
		// not re-validate the device-plugin precondition (the rule fires only on
		// transitions from off to on; staying on is a no-op for this rule).
		params := map[string]string{"gpu_observability_enable": "true"}
		current := map[string]string{
			"gpu_observability_enable":    "true",
			"nvidia_device_plugin_enable": "false",
		}
		err := validateAndMutateParams(params, "aws", current, false)
		if err != nil {
			t.Errorf("re-setting gpu_observability_enable=true while already enabled should not re-trigger the precondition, got: %v", err)
		}
	})

	t.Run("gcp accepts without device plugin (GKE manages it)", func(t *testing.T) {
		// GCP has no nvidia_device_plugin_enable param; GKE installs the device
		// plugin natively, so the precondition must not apply.
		params := map[string]string{"gpu_observability_enable": "true"}
		err := validateAndMutateParams(params, "gcp", map[string]string{}, false)
		if err != nil {
			t.Errorf("gcp gpu_observability_enable=true should be accepted without nvidia_device_plugin_enable, got: %v", err)
		}
	})

	t.Run("disable gpu_observability_enable does not require device plugin", func(t *testing.T) {
		params := map[string]string{"gpu_observability_enable": "false"}
		current := map[string]string{
			"gpu_observability_enable":    "true",
			"nvidia_device_plugin_enable": "false",
		}
		err := validateAndMutateParams(params, "aws", current, false)
		if err != nil {
			t.Errorf("setting gpu_observability_enable=false should pass regardless of device plugin state, got: %v", err)
		}
	})
}

// TestValidateAndMutateParams_PrometheusUrl_SSRF exercises the
// param-level SSRF guard via the real validateAndMutateParams entry
// point. Only inputs that do NOT require live DNS are covered here:
// IP literals, the *.svc.cluster.local allowlist, and scheme rejection.
// DNS-resolution behaviour is exercised in pkg/validator with a
// stubbed resolver — see pkg/validator/ssrf_test.go.
func TestValidateAndMutateParams_PrometheusUrl_SSRF(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{"empty value accepted (clear)", "", false, ""},
		{"in-cluster suffix accepted", "http://prom.kube-system.svc.cluster.local:9090", false, ""},
		{"in-cluster paid recipe accepted", "http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090", false, ""},
		{"file:// rejected", "file:///etc/passwd", true, "http or https"},
		{"gopher:// rejected", "gopher://x", true, "http or https"},
		{"link-local 169.254 rejected", "https://169.254.169.254/", true, "non-routable"},
		{"private 10.x rejected", "http://10.0.0.1", true, "non-routable"},
		{"private 192.168 rejected", "http://192.168.1.1", true, "non-routable"},
		{"loopback localhost rejected", "http://localhost", true, "non-routable"},
		{"loopback 127.0.0.1 rejected", "http://127.0.0.1", true, "non-routable"},
		{"cgnat 100.64 rejected", "http://100.64.0.1", true, "non-routable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"prometheus_url": tt.value}
			// Use a V3-rack context (empty current params + known aws provider).
			err := validateAndMutateParams(params, "aws", map[string]string{}, false)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.value, err)
			}
			if tt.wantErr && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("error %q should contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestAdditionalNodeGroupsConfigFieldRoundTrip(t *testing.T) {
	in := `[{"id":200,"type":"g2-standard-8","disk":100,"disk_type":"pd-ssd","capacity_type":"SPOT","min_size":0,"max_size":3,"label":"gpu-workers","zones":"us-east1-b,us-east1-c","gpu_type":"nvidia-l4","gpu_count":2},{"id":300,"type":"ct5lp-hightpu-8t","min_size":0,"max_size":1,"zones":"us-west4-a","tpu_topology":"2x4"}]`

	params := map[string]string{"additional_node_groups_config": in}
	if err := validateAndMutateParams(params, "gcp", map[string]string{}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(params["additional_node_groups_config"])
	if err != nil {
		t.Fatalf("expected base64 output: %v", err)
	}

	// every provider-specific field must survive the validate/marshal round-trip
	for _, field := range []string{`"disk_type":"pd-ssd"`, `"zones":"us-east1-b,us-east1-c"`, `"gpu_type":"nvidia-l4"`, `"gpu_count":2`, `"tpu_topology":"2x4"`, `"capacity_type":"SPOT"`, `"min_size":0`} {
		if !strings.Contains(string(decoded), field) {
			t.Errorf("field %s stripped by param round-trip: %s", field, decoded)
		}
	}
}

func TestAdditionalNodeGroupsConfigGpuValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{"gpu_count without gpu_type", `[{"id":1,"type":"g2-standard-8","gpu_count":1}]`, "gpu_type is required"},
		{"zero gpu_count", `[{"id":1,"type":"g2-standard-8","gpu_type":"nvidia-l4","gpu_count":0}]`, "invalid gpu_count"},
		{"negative gpu_count", `[{"id":1,"type":"g2-standard-8","gpu_type":"nvidia-l4","gpu_count":-1}]`, "invalid gpu_count"},
		{"valid gpu pool", `[{"id":1,"type":"g2-standard-8","gpu_type":"nvidia-l4","gpu_count":1}]`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"additional_node_groups_config": tt.config}
			err := validateAndMutateParams(params, "gcp", map[string]string{}, false)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAdditionalNodeGroupsConfigTpuValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{"bad topology format", `[{"id":1,"type":"ct5lp-hightpu-8t","tpu_topology":"2*4"}]`, "invalid tpu_topology"},
		{"zero topology dimension", `[{"id":1,"type":"ct5lp-hightpu-8t","tpu_topology":"0x0"}]`, "invalid tpu_topology"},
		{"topology with gpu_type", `[{"id":1,"type":"ct5lp-hightpu-8t","gpu_type":"nvidia-l4","tpu_topology":"2x4"}]`, "cannot be combined"},
		{"topology on non-tpu machine", `[{"id":1,"type":"g2-standard-8","tpu_topology":"2x4"}]`, "requires a TPU machine type"},
		{"valid 2d topology", `[{"id":1,"type":"ct5lp-hightpu-8t","tpu_topology":"2x4"}]`, ""},
		{"valid 3d topology", `[{"id":1,"type":"ct5p-hightpu-4t","tpu_topology":"2x2x2"}]`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]string{"additional_node_groups_config": tt.config}
			err := validateAndMutateParams(params, "gcp", map[string]string{}, false)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateAndMutateParams_CloudwatchDisableWarnings(t *testing.T) {
	const warnA = "cloudwatch_disable stops Convox"
	const warnB = "disabling fluentd stops application logs"
	const warnC = "cloudwatch_disable also covers the rack system log group"

	cases := []struct {
		name         string
		params       map[string]string
		current      map[string]string
		provider     string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "A fires: cloudwatch_disable=true, fluentd enabled",
			params:       map[string]string{"cloudwatch_disable": "true"},
			wantContains: []string{warnA},
			wantAbsent:   []string{warnB},
		},
		{
			name:         "A fires for non-canonical bool form",
			params:       map[string]string{"cloudwatch_disable": "1"},
			wantContains: []string{warnA},
		},
		{
			name:         "B fires: fluentd_disable=true, cloudwatch enabled",
			params:       map[string]string{"fluentd_disable": "true"},
			wantContains: []string{warnB},
			wantAbsent:   []string{warnA},
		},
		{
			name:         "B fires for non-canonical bool form",
			params:       map[string]string{"fluentd_disable": "1"},
			wantContains: []string{warnB},
		},
		{
			name:       "both set in one call fires neither",
			params:     map[string]string{"fluentd_disable": "true", "cloudwatch_disable": "true"},
			wantAbsent: []string{warnA, warnB},
		},
		{
			name:       "A suppressed when fluentd already disabled",
			params:     map[string]string{"cloudwatch_disable": "true"},
			current:    map[string]string{"fluentd_disable": "true"},
			wantAbsent: []string{warnA, warnB},
		},
		{
			name:       "B suppressed when cloudwatch already disabled",
			params:     map[string]string{"fluentd_disable": "true"},
			current:    map[string]string{"cloudwatch_disable": "true"},
			wantAbsent: []string{warnA, warnB},
		},
		{
			name:       "no warning on unrelated param set",
			params:     map[string]string{"cost_tracking_enable": "true"},
			wantAbsent: []string{warnA, warnB},
		},
		{
			name:         "C fires: app_cloudwatch_disable=true while cloudwatch already disabled",
			params:       map[string]string{"app_cloudwatch_disable": "true"},
			current:      map[string]string{"cloudwatch_disable": "true"},
			wantContains: []string{warnC},
			wantAbsent:   []string{warnA, warnB},
		},
		{
			name:       "C absent when cloudwatch enabled",
			params:     map[string]string{"app_cloudwatch_disable": "true"},
			wantAbsent: []string{warnA, warnB, warnC},
		},
		{
			name:         "C fires the other way: cloudwatch_disable=true while app_cloudwatch_disable already set",
			params:       map[string]string{"cloudwatch_disable": "true"},
			current:      map[string]string{"app_cloudwatch_disable": "true"},
			wantContains: []string{warnC},
			wantAbsent:   []string{warnA, warnB},
		},
		{
			name:         "C fires when both are set in one call",
			params:       map[string]string{"app_cloudwatch_disable": "true", "cloudwatch_disable": "true"},
			wantContains: []string{warnC},
			wantAbsent:   []string{warnA, warnB},
		},
		{
			name:       "B suppressed when app_cloudwatch_disable already set",
			params:     map[string]string{"fluentd_disable": "true"},
			current:    map[string]string{"app_cloudwatch_disable": "true"},
			wantAbsent: []string{warnA, warnB, warnC},
		},
		{
			name:       "no warning on unrelated param set with both already disabled",
			params:     map[string]string{"cost_tracking_enable": "true"},
			current:    map[string]string{"cloudwatch_disable": "true", "app_cloudwatch_disable": "true"},
			wantAbsent: []string{warnA, warnB, warnC},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			current := tc.current
			if current == nil {
				current = map[string]string{}
			}
			out := captureStderr(t, func() {
				if err := validateAndMutateParams(tc.params, "aws", current, false); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			for _, s := range tc.wantContains {
				if !strings.Contains(out, s) {
					t.Errorf("expected stderr to contain %q, got %q", s, out)
				}
			}
			for _, s := range tc.wantAbsent {
				if strings.Contains(out, s) {
					t.Errorf("expected stderr NOT to contain %q, got %q", s, out)
				}
			}
		})
	}
}

func TestValidateAndMutateParams_CloudwatchDisableBoolValidation(t *testing.T) {
	for _, param := range []string{"cloudwatch_disable", "app_cloudwatch_disable"} {
		captureStderr(t, func() {
			for _, v := range []string{"true", "false", "1", "0", "t", "F"} {
				if err := validateAndMutateParams(map[string]string{param: v}, "aws", map[string]string{}, false); err != nil {
					t.Errorf("%s=%q: unexpected error: %v", param, v, err)
				}
			}
		})
		if err := validateAndMutateParams(map[string]string{param: "maybe"}, "aws", map[string]string{}, false); err == nil {
			t.Errorf("%s=maybe: expected error, got nil", param)
		}
	}
}

func b64json(t *testing.T, s string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestValidateAndMutateParams_NodePoolRemoval(t *testing.T) {
	pools := func(names ...string) string {
		entries := make([]string, len(names))
		for i, n := range names {
			entries[i] = `{"name":"` + n + `"}`
		}
		return "[" + strings.Join(entries, ",") + "]"
	}

	for _, tc := range []struct {
		name    string
		params  map[string]string
		current map[string]string
		force   bool
		err     string
	}{
		{
			name:    "removal blocked",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("batch", "gpu")},
			err:     "convox.io/nodepool=batch",
		},
		{
			name:    "rename blocked",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("gpu-large")},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			err:     "convox.io/nodepool=gpu",
		},
		{
			name:    "clear blocked",
			params:  map[string]string{"additional_karpenter_nodepools_config": ""},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			err:     "node pools from the rack: gpu",
		},
		{
			name:    "removal allowed with force",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("batch", "gpu")},
			force:   true,
		},
		{
			name:    "addition only",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("batch", "gpu")},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
		},
		{
			name:    "current value base64 encoded",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			current: map[string]string{"additional_karpenter_nodepools_config": b64json(t, pools("batch", "gpu"))},
			err:     "convox.io/nodepool=batch",
		},
		{
			name:    "no current value",
			params:  map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
			current: map[string]string{},
		},
		{
			name:    "unrelated param on a rack with pools",
			params:  map[string]string{"node_disk": "50"},
			current: map[string]string{"additional_karpenter_nodepools_config": pools("gpu")},
		},
		{
			name:    "node group label removed",
			params:  map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"}]`},
			current: map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"},{"type":"t3.large","label":"analytics"}]`},
			err:     "node groups from the rack: analytics",
		},
		{
			name:    "label moved to build groups is a removal",
			params:  map[string]string{"additional_node_groups_config": `[]`, "additional_build_groups_config": `[{"type":"t3.large","label":"analytics"}]`},
			current: map[string]string{"additional_node_groups_config": `[{"type":"t3.large","label":"analytics"}]`},
			err:     "convox.io/label=analytics",
		},
		{
			name:    "duplicate label with one entry removed",
			params:  map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"}]`},
			current: map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"},{"type":"t3.large","label":"web"}]`},
		},
		{
			name:    "unlabelled group removed",
			params:  map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"}]`},
			current: map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"},{"type":"t3.large"}]`},
		},
		{
			name:    "duplicate label fully removed is listed once",
			params:  map[string]string{"additional_node_groups_config": `[]`},
			current: map[string]string{"additional_node_groups_config": `[{"type":"t3.medium","label":"web"},{"type":"t3.large","label":"web"}]`},
			err:     "node groups from the rack: web\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAndMutateParams(tc.params, "aws", tc.current, tc.force)

			if tc.err == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q", tc.err)
			}
			if !strings.Contains(err.Error(), tc.err) {
				t.Errorf("error %q should contain %q", err.Error(), tc.err)
			}
			if !strings.Contains(err.Error(), "--force") {
				t.Errorf("error %q should mention --force", err.Error())
			}
			if !strings.HasPrefix(err.Error(), "destructive change,") {
				t.Errorf("error %q should lead with the destructive warning", err.Error())
			}
		})
	}
}

func TestValidateAndMutateParams_NodePoolRemovalFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "pools.json")
	if err := os.WriteFile(f, []byte(`[{"name":"gpu"}]`), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := validateAndMutateParams(
		map[string]string{"additional_karpenter_nodepools_config": f},
		"aws",
		map[string]string{"additional_karpenter_nodepools_config": `[{"name":"batch"},{"name":"gpu"}]`},
		false,
	)
	if err == nil {
		t.Fatal("expected error for pool removed via a .json file value")
	}
	if !strings.Contains(err.Error(), "convox.io/nodepool=batch") {
		t.Errorf("error %q should name the removed pool", err.Error())
	}
}

func TestValidateAndMutateParams_NodeGroupRemovalFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "groups.json")
	if err := os.WriteFile(f, []byte(`[{"id":0,"type":"t3.medium","label":"web"}]`), 0600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := validateAndMutateParams(
		map[string]string{"additional_node_groups_config": f},
		"aws",
		map[string]string{"additional_node_groups_config": `[{"id":0,"type":"t3.medium","label":"web"},{"id":1,"type":"t3.large","label":"analytics"}]`},
		false,
	)
	if err == nil {
		t.Fatal("expected error for group removed via a .json file value")
	}
	if !strings.Contains(err.Error(), "convox.io/label=analytics") {
		t.Errorf("error %q should name the removed group", err.Error())
	}
}

func TestValidateAndMutateParams_BuildGroupRemovalNamesBuilds(t *testing.T) {
	err := validateAndMutateParams(
		map[string]string{"additional_build_groups_config": `[]`},
		"aws",
		map[string]string{"additional_build_groups_config": `[{"id":0,"type":"t3.large","label":"bigbuild"}]`},
		false,
	)
	if err == nil {
		t.Fatal("expected error for removed build group")
	}
	if !strings.Contains(err.Error(), "build groups from the rack: bigbuild") {
		t.Errorf("error %q should name build groups, not node groups", err.Error())
	}
	if !strings.Contains(err.Error(), "Builds pinned by BuildLabels to convox.io/label=bigbuild") {
		t.Errorf("error %q should attribute the breakage to builds, not services", err.Error())
	}
}

func TestValidateAndMutateParams_DeployFastFail(t *testing.T) {
	cases := []struct {
		param   string
		value   string
		wantErr string
	}{
		{"deploy_progress_deadline", "0", ""},
		{"deploy_progress_deadline", "30", ""},
		{"deploy_progress_deadline", "21600", ""},
		{"deploy_progress_deadline", "29", "between 30 and 21600"},
		{"deploy_progress_deadline", "21601", "between 30 and 21600"},
		{"deploy_progress_deadline", "-1", "between 30 and 21600"},
		{"deploy_progress_deadline", "soon", "must be an integer"},
		{"deploy_crash_restart_limit", "0", ""},
		{"deploy_crash_restart_limit", "3", ""},
		{"deploy_crash_restart_limit", "-1", "positive number of restarts"},
		{"deploy_crash_restart_limit", "three", "must be an integer"},
	}

	for _, c := range cases {
		t.Run(c.param+"="+c.value, func(t *testing.T) {
			err := validateAndMutateParams(map[string]string{c.param: c.value}, "aws", map[string]string{}, false)
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("expected %s=%s to be accepted, got: %v", c.param, c.value, err)
			case c.wantErr != "" && err == nil:
				t.Fatalf("expected %s=%s to be rejected", c.param, c.value)
			case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
				t.Errorf("error %q should contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestValidateAndMutateParams_Whitelist(t *testing.T) {
	const (
		warning      = "WARNING: whitelist does not include 0.0.0.0/0. Clients outside these ranges, including Convox Console, can no longer reach this rack."
		warningStart = "WARNING: whitelist"
	)

	for _, tc := range []struct {
		name     string
		params   map[string]string
		current  map[string]string
		provider string
		force    bool
		err      string
		notErr   string
		want     string
		absent   bool
		warn     bool
	}{
		{
			name:   "open value accepted unchanged",
			params: map[string]string{"whitelist": "0.0.0.0/0"},
			want:   "0.0.0.0/0",
		},
		{
			name:   "restricted value refused",
			params: map[string]string{"whitelist": "10.0.0.0/8"},
			err:    "does not include 0.0.0.0/0",
		},
		{
			name:   "restricted value refusal names force",
			params: map[string]string{"whitelist": "10.0.0.0/8"},
			err:    "--force",
		},
		{
			name:   "restricted value refusal explains the consequence",
			params: map[string]string{"whitelist": "10.0.0.0/8"},
			err:    "balancer rules in your cloud provider",
		},
		{
			name:   "restricted value accepted with force",
			params: map[string]string{"whitelist": "10.0.0.0/8"},
			force:  true,
			want:   "10.0.0.0/8",
			warn:   true,
		},
		{
			name:   "restricted range alongside the default route",
			params: map[string]string{"whitelist": "10.0.0.0/8,0.0.0.0/0"},
			want:   "10.0.0.0/8,0.0.0.0/0",
		},
		{
			name:   "surrounding whitespace trimmed",
			params: map[string]string{"whitelist": "0.0.0.0/0, 10.0.0.0/8"},
			want:   "0.0.0.0/0,10.0.0.0/8",
		},
		{
			name:   "padded mask counts as the default route",
			params: map[string]string{"whitelist": "0.0.0.0/00"},
			want:   "0.0.0.0/0",
		},
		{
			name:   "host address with a zero prefix is the default route",
			params: map[string]string{"whitelist": "1.2.3.4/0"},
			want:   "0.0.0.0/0",
		},
		{
			name:   "ipv4 mapped default route",
			params: map[string]string{"whitelist": "::ffff:0.0.0.0/96"},
			want:   "0.0.0.0/0",
		},
		{
			name:   "host bits canonicalized",
			params: map[string]string{"whitelist": "10.0.0.1/8"},
			force:  true,
			want:   "10.0.0.0/8",
			warn:   true,
		},
		{
			name:   "prefix out of range",
			params: map[string]string{"whitelist": "10.0.0.0/33"},
			err:    "not a valid cidr range",
		},
		{
			name:   "prefix out of range is not forceable",
			params: map[string]string{"whitelist": "10.0.0.0/33"},
			force:  true,
			err:    "not a valid cidr range",
		},
		{
			name:   "not a cidr at all",
			params: map[string]string{"whitelist": "not-a-cidr"},
			err:    "not a valid cidr range",
		},
		{
			name:   "bare address without a mask",
			params: map[string]string{"whitelist": "1.2.3.4"},
			err:    "not a valid cidr range",
		},
		{
			name:   "trailing comma dropped",
			params: map[string]string{"whitelist": "0.0.0.0/0,"},
			want:   "0.0.0.0/0",
		},
		{
			name:   "only separators",
			params: map[string]string{"whitelist": ","},
			err:    "requires at least one cidr range",
		},
		{
			name:   "ipv6 range refused",
			params: map[string]string{"whitelist": "::/0"},
			err:    "not an ipv4 cidr range",
		},
		{
			name:   "ipv6 range is not forceable",
			params: map[string]string{"whitelist": "::/0"},
			force:  true,
			err:    "not an ipv4 cidr range",
		},
		{
			name:   "ipv6 range refused alongside the default route",
			params: map[string]string{"whitelist": "0.0.0.0/0,::/0"},
			err:    "not an ipv4 cidr range",
		},
		{
			name:   "ipv4 mapped range accepted",
			params: map[string]string{"whitelist": "::ffff:10.0.0.0/104"},
			force:  true,
			want:   "10.0.0.0/8",
			warn:   true,
		},
		{
			name:     "refused on gcp",
			params:   map[string]string{"whitelist": "10.0.0.0/8"},
			provider: "gcp",
			err:      "does not include 0.0.0.0/0",
		},
		{
			name:     "refused on metal",
			params:   map[string]string{"whitelist": "10.0.0.0/8"},
			provider: "metal",
			err:      "does not include 0.0.0.0/0",
		},
		{
			name:     "refused when the provider has no known param map",
			params:   map[string]string{"whitelist": "10.0.0.0/8"},
			provider: "unknown",
			err:      "does not include 0.0.0.0/0",
		},
		{
			name:    "unrelated param on a restricted rack",
			params:  map[string]string{"node_disk": "50"},
			current: map[string]string{"whitelist": "10.0.0.0/8"},
		},
		{
			name:   "unrelated param does not invent the key",
			params: map[string]string{"node_disk": "50"},
			absent: true,
		},
		{
			name:   "empty value keeps the existing message",
			params: map[string]string{"whitelist": ""},
			err:    "requires an explicit value",
			notErr: "cidr",
		},
		{
			name:   "whitespace value keeps the existing message",
			params: map[string]string{"whitelist": "   "},
			err:    "requires an explicit value",
			notErr: "cidr",
		},
		{
			name:    "v2 rack on a known provider skips validation",
			params:  map[string]string{"whitelist": "10.0.0.0/8"},
			current: map[string]string{"HighAvailability": "true"},
		},
		{
			name:     "local rack reports the unknown key first",
			params:   map[string]string{"whitelist": "10.0.0.0/8"},
			provider: "local",
			err:      "unknown parameter",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := tc.provider
			if provider == "" {
				provider = "aws"
			}

			current := tc.current
			if current == nil {
				current = map[string]string{}
			}

			var err error

			out := captureStderr(t, func() {
				err = validateAndMutateParams(tc.params, provider, current, tc.force)
			})

			switch {
			case tc.err == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.err != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tc.err)
			case tc.err != "" && !strings.Contains(err.Error(), tc.err):
				t.Fatalf("error %q should contain %q", err.Error(), tc.err)
			}

			if tc.notErr != "" && err != nil && strings.Contains(err.Error(), tc.notErr) {
				t.Errorf("error %q should not contain %q", err.Error(), tc.notErr)
			}

			if tc.want != "" && tc.params["whitelist"] != tc.want {
				t.Errorf("whitelist = %q, want %q", tc.params["whitelist"], tc.want)
			}

			if tc.absent {
				if _, ok := tc.params["whitelist"]; ok {
					t.Errorf("whitelist key was added to a call that did not set it")
				}
			}

			if tc.warn && !strings.Contains(out, warning) {
				t.Errorf("expected stderr to contain %q, got %q", warning, out)
			}

			if !tc.warn && strings.Contains(out, warningStart) {
				t.Errorf("unexpected whitelist warning on stderr: %q", out)
			}
		})
	}
}
