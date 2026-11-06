package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

func render(t *testing.T, baseDir, provider string) string {
	t.Helper()
	var b bytes.Buffer
	if err := renderTelemetry(&b, baseDir, provider); err != nil {
		t.Fatalf("renderTelemetry(%s): %v", provider, err)
	}
	return b.String()
}

func TestVerifyAllMatchesCommitted(t *testing.T) {
	if err := VerifyAll(repoRoot(t)); err != nil {
		t.Fatalf("committed telemetry.tf out of sync with generator: %v", err)
	}
}

func TestComputeMapsExcludesCredentials(t *testing.T) {
	root := repoRoot(t)
	cases := map[string][]string{
		"do":  {"access_id", "secret_key", "token"},
		"aws": {"private_eks_host", "private_eks_user"},
	}
	for provider, omitted := range cases {
		varMap, defaultMap, err := computeMaps(root, provider)
		if err != nil {
			t.Fatalf("computeMaps(%s): %v", provider, err)
		}
		for _, name := range omitted {
			if _, ok := varMap[name]; ok {
				t.Errorf("%s: %q must be excluded from value map", provider, name)
			}
			if _, ok := defaultMap[name]; ok {
				t.Errorf("%s: %q must be excluded from default map", provider, name)
			}
		}
	}
}

func TestComputeMapsSameKeySet(t *testing.T) {
	root := repoRoot(t)
	for _, provider := range providers {
		varMap, defaultMap, err := computeMaps(root, provider)
		if err != nil {
			t.Fatalf("computeMaps(%s): %v", provider, err)
		}
		if len(varMap) != len(defaultMap) {
			t.Fatalf("%s: value map has %d keys, default map has %d", provider, len(varMap), len(defaultMap))
		}
		for name := range varMap {
			if _, ok := defaultMap[name]; !ok {
				t.Errorf("%s: %q in value map but not default map", provider, name)
			}
		}
	}
}

func TestDefaultRenderingCoalescesZeroValues(t *testing.T) {
	out := render(t, repoRoot(t), "aws")
	for _, want := range []string{
		`name                                      = ""`,
		`build_node_min_count                      = "0"`,
		`build_node_enabled                        = "false"`,
		`telemetry                                 = "false"`,
	} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("aws telemetry.tf missing expected default line: %q", want)
		}
	}
}

func TestRenderIsFormatIdempotent(t *testing.T) {
	for _, provider := range providers {
		out := render(t, repoRoot(t), provider)
		if formatted := hclwrite.Format([]byte(out)); !bytes.Equal(formatted, []byte(out)) {
			t.Errorf("%s: rendered output is not hclwrite.Format-clean", provider)
		}
	}
}

func TestDefaultRenderingDoesNotHTMLEscape(t *testing.T) {
	base := t.TempDir()

	tmplDir := filepath.Join(base, "cmd", "telemetry-gen")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmpl := `{{ define "telemetry" }}
locals {
  telemetry_default_map = {
{{- range $k, $v := .DefaultMap }}
    {{ $k }} = "{{ $v }}"
{{- end }}
  }
}
{{ end }}`
	if err := os.WriteFile(filepath.Join(tmplDir, "telemetry.tf.tmpl"), []byte(tmpl), 0600); err != nil {
		t.Fatal(err)
	}

	modDir := filepath.Join(base, "terraform", "system", "esc")
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	variables := `variable "special" {
  type    = string
  default = "a&b<c>d"
}
`
	if err := os.WriteFile(filepath.Join(modDir, "variables.tf"), []byte(variables), 0600); err != nil {
		t.Fatal(err)
	}

	out := render(t, base, "esc")
	if !bytes.Contains([]byte(out), []byte(`special = "a&b<c>d"`)) {
		t.Errorf("expected raw string default, got:\n%s", out)
	}
	if bytes.Contains([]byte(out), []byte("&amp;")) || bytes.Contains([]byte(out), []byte("&lt;")) {
		t.Errorf("output is HTML-escaped (use text/template not html/template):\n%s", out)
	}
}

func TestRedactSetMatchesRedactedParamKeys(t *testing.T) {
	module, diags := tfconfig.LoadModule(filepath.Join(repoRoot(t), "terraform", "rack", "k8s"))
	if diags.Err() != nil {
		t.Fatal(diags.Err())
	}
	v, ok := module.Variables["redacted_param_keys"]
	if !ok {
		t.Fatal("redacted_param_keys variable not found in terraform/rack/k8s")
	}
	list, ok := v.Default.([]interface{})
	if !ok {
		t.Fatalf("redacted_param_keys default is %T, want list", v.Default)
	}

	declared := map[string]bool{}
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("redacted_param_keys element is %T, want string", item)
		}
		declared[s] = true
	}
	if len(declared) != len(telemetryRedact) {
		t.Fatalf("redacted_param_keys default (%d keys) and telemetryRedact (%d keys) diverge", len(declared), len(telemetryRedact))
	}
	for k := range telemetryRedact {
		if !declared[k] {
			t.Errorf("REDACT key %q missing from redacted_param_keys default", k)
		}
	}
}

func TestRedactKeysSurviveInMaps(t *testing.T) {
	root := repoRoot(t)
	for _, provider := range providers {
		module, diags := tfconfig.LoadModule(filepath.Join(root, "terraform", "system", provider))
		if diags.Err() != nil {
			t.Fatalf("LoadModule(%s): %v", provider, diags.Err())
		}
		varMap, defaultMap, err := computeMaps(root, provider)
		if err != nil {
			t.Fatalf("computeMaps(%s): %v", provider, err)
		}
		for name := range module.Variables {
			if !telemetryRedact[name] {
				continue
			}
			if _, ok := varMap[name]; !ok {
				t.Errorf("%s: REDACT key %q must not be excluded from value map", provider, name)
			}
			if _, ok := defaultMap[name]; !ok {
				t.Errorf("%s: REDACT key %q must not be excluded from default map", provider, name)
			}
		}
	}
}

func TestUnbucketedParamNames(t *testing.T) {
	omit := map[string]bool{"secret_key": true}
	redact := map[string]bool{"docker_hub_password": true}
	benign := map[string]bool{"imds_http_tokens": true}

	flagged := unbucketedParamNames(
		[]string{
			"region",              // no pattern
			"secret_key",          // omit
			"docker_hub_password", // redact
			"imds_http_tokens",    // benign
			"new_api_token",       // token pattern, untriaged
			"private_endpoint",    // private_ prefix, untriaged
			"signing_key",         // _key suffix, untriaged
		},
		omit, redact, benign,
	)

	want := map[string]bool{"new_api_token": true, "private_endpoint": true, "signing_key": true}
	if len(flagged) != len(want) {
		t.Fatalf("expected %d unbucketed names, got %v", len(want), flagged)
	}
	for _, n := range flagged {
		if !want[n] {
			t.Errorf("unexpected unbucketed name: %q", n)
		}
	}
}
