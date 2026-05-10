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
		"build_node_enabled", "buildkit_host_path_cache_enable", "convox_domain_tls_cert_disable",
		"cost_tracking_enable", "deploy_extra_nlb", "disable_convox_resolver",
		"disable_image_manifest_cache", "ebs_volume_encryption_enabled",
		"ecr_scan_on_push_enable", "efs_csi_driver_enable", "fluentd_disable",
		"gpu_tag_enable", "imds_tags_enable", "internal_router",
		"karpenter_consolidation_enabled", "keda_enable", "pod_identity_agent_enable",
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
