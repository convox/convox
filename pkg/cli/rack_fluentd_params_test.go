package cli

import (
	"fmt"
	"strings"
	"testing"
)

var fluentdParamDefaults = map[string]string{
	"router_log_redact_params": `""`,
	"syslog_access_logs":       "false",
	"syslog_tls_verify":        "true",
}

func TestFluentdParamsWired(t *testing.T) {
	for _, param := range []string{"router_log_redact_params", "syslog_access_logs"} {
		if !awsKnownParams[param] {
			t.Errorf("awsKnownParams is missing %s", param)
		}
		assertFluentdVariable(t, "../../terraform/system/aws/variables.tf", param)
		assertFluentdVariable(t, "../../terraform/fluentd/aws/variables.tf", param)
		assertFluentdParamPassed(t, "../../terraform/system/aws/main.tf", "fluentd", param)
	}

	for _, param := range []string{"router_log_redact_params", "syslog_tls_verify"} {
		assertFluentdVariable(t, "../../terraform/fluentd/elasticsearch/variables.tf", param)

		for _, provider := range []string{"azure", "do", "gcp", "metal"} {
			if !providerKnownParams[provider][param] {
				t.Errorf("%sKnownParams is missing %s", provider, param)
			}
			for _, tier := range []string{"system", "rack", "api"} {
				assertFluentdVariable(t, fmt.Sprintf("../../terraform/%s/%s/variables.tf", tier, provider), param)
			}
			for path, module := range map[string]string{
				fmt.Sprintf("../../terraform/system/%s/main.tf", provider): "rack",
				fmt.Sprintf("../../terraform/rack/%s/main.tf", provider):   "api",
				fmt.Sprintf("../../terraform/api/%s/main.tf", provider):    "fluentd",
			} {
				assertFluentdParamPassed(t, path, module, param)
			}
		}
	}
}

func assertFluentdVariable(t *testing.T, path, name string) {
	t.Helper()

	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "variable" || len(block.Labels) != 1 || block.Labels[0] != name {
			continue
		}
		def, ok := block.Body.Attributes["default"]
		if !ok {
			t.Errorf("%s: %s declares no default", path, name)
			return
		}
		if src := strings.TrimSpace(string(def.Expr.Range().SliceBytes(mustReadFile(t, path)))); src != fluentdParamDefaults[name] {
			t.Errorf("%s: %s default is %s; want %s", path, name, src, fluentdParamDefaults[name])
		}
		return
	}
	t.Errorf("%s: no variable %q", path, name)
}

func assertFluentdParamPassed(t *testing.T, path, module, name string) {
	t.Helper()

	for _, block := range parseHCLBody(t, path).Blocks {
		if block.Type != "module" || len(block.Labels) != 1 || block.Labels[0] != module {
			continue
		}
		attr, ok := block.Body.Attributes[name]
		if !ok {
			t.Errorf("%s: module %q does not pass %s", path, module, name)
			return
		}
		if src := strings.TrimSpace(string(attr.Expr.Range().SliceBytes(mustReadFile(t, path)))); src != "var."+name {
			t.Errorf("%s: %s is %q; want %q", path, name, src, "var."+name)
		}
		return
	}
	t.Errorf("%s: no module %q", path, module)
}

func TestValidateAndMutateParams_RouterLogRedactParams(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"all", false},
		{"token", false},
		{"reset_password_token,token,api.key,x-sig", false},
		{"", false},
		{"ALL", true},
		{"All", true},
		{"token,all", true},
		{"all,token", true},
		{"a,,b", true},
		{"token,", true},
		{"token, code", true},
		{"token code", true},
		{"user[token]", true},
		{"token=1", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			for _, provider := range []string{"aws", "azure", "do", "gcp", "metal"} {
				err := validateAndMutateParams(map[string]string{"router_log_redact_params": tt.value}, provider, map[string]string{}, false)
				if tt.wantErr && err == nil {
					t.Errorf("%s: %q accepted", provider, tt.value)
				}
				if !tt.wantErr && err != nil {
					t.Errorf("%s: %q rejected: %v", provider, tt.value, err)
				}
			}
		})
	}

	if err := validateAndMutateParams(map[string]string{"router_log_redact_params": "token"}, "local", map[string]string{}, false); err == nil {
		t.Error("router_log_redact_params should be unknown on local")
	}
}

func TestValidateAndMutateParams_FluentdMemory(t *testing.T) {
	const invalid = "param 'fluentd_memory' must be a whole number of Mi or Gi, such as 200Mi or 1Gi"

	tests := []struct {
		value   string
		wantErr bool
	}{
		{"200Mi", false},
		{"512Mi", false},
		{"1Gi", false},
		{"1Mi", false},
		{"0Mi", true},
		{"0200Mi", true},
		{"05Gi", true},
		{"2.5Gi", true},
		{"512", true},
		{"500m", true},
		{"2560000Ki", true},
		{"1Ti", true},
		{"512mi", true},
		{"512 Mi", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			for _, provider := range []string{"aws", "azure", "do", "gcp", "metal"} {
				for _, force := range []bool{false, true} {
					err := validateAndMutateParams(map[string]string{"fluentd_memory": tt.value}, provider, map[string]string{}, force)
					if !tt.wantErr {
						if err != nil {
							t.Errorf("%s force=%v: %q rejected: %v", provider, force, tt.value, err)
						}
						continue
					}
					if err == nil || err.Error() != invalid {
						t.Errorf("%s force=%v: %q: got %v, want %q", provider, force, tt.value, err, invalid)
					}
				}
			}
		})
	}

	if err := validateAndMutateParams(map[string]string{"fluentd_memory": "512Mi"}, "local", map[string]string{}, false); err == nil {
		t.Error("fluentd_memory should be unknown on local")
	}
}

func TestValidateAndMutateParams_SyslogParamsProviderCoverage(t *testing.T) {
	for _, provider := range []string{"azure", "do", "gcp", "metal"} {
		if err := validateAndMutateParams(map[string]string{"syslog_tls_verify": "false"}, provider, map[string]string{}, false); err != nil {
			t.Errorf("%s: syslog_tls_verify=false rejected: %v", provider, err)
		}
		if err := validateAndMutateParams(map[string]string{"syslog_tls_verify": "garbage"}, provider, map[string]string{}, false); err == nil {
			t.Errorf("%s: syslog_tls_verify=garbage accepted", provider)
		}
		if err := validateAndMutateParams(map[string]string{"syslog_access_logs": "true"}, provider, map[string]string{}, false); err == nil {
			t.Errorf("%s: syslog_access_logs should be unknown", provider)
		}
	}
}

func TestValidateAndMutateParams_Syslog(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
		got     string
	}{
		{"", false, ""},
		{"  ", false, ""},
		{"tcp://logs", false, ""},
		{"udp://10.0.0.5:514", false, ""},
		{"tcp+tls://logs.example.com:6514", false, ""},
		{"tcp+tls://a:6514, udp://b:514", false, ""},
		{"tcp://a:514,\tudp://b_1:514 ", false, ""},
		{"udp+tls://logs.example.com:6514", true, "udp+tls://logs.example.com:6514"},
		{"tpc://logs", true, "tpc://logs"},
		{"TCP://logs", true, "TCP://logs"},
		{"tcp://logs/", true, "tcp://logs/"},
		{"tcp://logs:abc", true, "tcp://logs:abc"},
		{"tcp://", true, "tcp://"},
		{"tcp://user@logs:514", true, "tcp://user@logs:514"},
		{"logs:514", true, "logs:514"},
		{"null", true, "null"},
		{"tcp://a,", true, ""},
		{"tcp://a, ", true, " "},
		{"tcp://a,,udp://b", true, ""},
		{"tcp://a, tcp://b c", true, " tcp://b c"},
		{"udp+tcp://logs", true, "udp+tcp://logs"},
		{"x tcp://logs", true, "x tcp://logs"},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			for _, provider := range []string{"aws", "azure", "do", "gcp", "metal"} {
				for _, force := range []bool{false, true} {
					err := validateAndMutateParams(map[string]string{"syslog": tt.value}, provider, map[string]string{}, force)
					if !tt.wantErr {
						if err != nil {
							t.Errorf("%s force=%v: %q rejected: %v", provider, force, tt.value, err)
						}
						continue
					}
					if err == nil {
						t.Errorf("%s force=%v: %q accepted", provider, force, tt.value)
						continue
					}
					if want := fmt.Sprintf("(got %q)", tt.got); !strings.Contains(err.Error(), want) {
						t.Errorf("%s force=%v: %q: error %q does not contain %s", provider, force, tt.value, err, want)
					}
				}
			}
		})
	}
}
