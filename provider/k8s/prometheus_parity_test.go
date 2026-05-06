package k8s

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

// promQLParityYAMLPath is the canonical hand-mirror manifest. Tests resolve it
// relative to the package source dir; `go test` runs with cwd at the package
// root, so two levels up reaches `examples/`.
const promQLParityYAMLPath = "../../examples/gpu-llm/grafana/promql-source-of-truth.yaml"
const promQLDashboardsGlob = "../../examples/gpu-llm/grafana/*.json"

// validUpstreamSourcePrefixes lists the `_source` annotation prefixes that
// exempt a panel target from Go-const lookup. These cover JSONs re-hosted from
// upstream projects (vLLM, TGI, Triton, etc.) and Karpenter-grafana-ref
// derivatives. Convox-authored extensions to upstream-derived JSONs use the
// `convox-authored:` prefix instead.
func validUpstreamSourcePrefixes() []string {
	return []string{
		"upstream:vllm",
		"upstream:tgi",
		"upstream:triton",
		"upstream:ollama",
		"upstream:karpenter-grafana-ref",
	}
}

// TestPromQLParityGoVsYaml: every entry in AllPromQLConstants() must have a
// matching key in promql-source-of-truth.yaml with the same trimmed value, AND
// the YAML must not list unknown keys. Drift fails CI.
func TestPromQLParityGoVsYaml(t *testing.T) {
	data, err := os.ReadFile(promQLParityYAMLPath)
	if err != nil {
		t.Fatalf("cannot read parity manifest at %s: %v", promQLParityYAMLPath, err)
	}
	var doc struct {
		Queries map[string]string `yaml:"queries"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("cannot parse parity manifest: %v", err)
	}
	consts := AllPromQLConstants()
	for name, goVal := range consts {
		yamlVal, ok := doc.Queries[name]
		if !ok {
			t.Errorf("constant %s missing from parity manifest %s", name, promQLParityYAMLPath)
			continue
		}
		if strings.TrimSpace(yamlVal) != strings.TrimSpace(goVal) {
			t.Errorf("constant %s diverges:\n  go:    %q\n  yaml:  %q", name, goVal, yamlVal)
		}
	}
	for name := range doc.Queries {
		if _, ok := consts[name]; !ok {
			t.Errorf("parity manifest has unknown constant %s (remove or add to Go consts)", name)
		}
	}
}

// TestPromQLConstantsAppearInJsonDashboards: forward parity. Every JSON
// dashboard panel target carrying `_source: "<CONST_NAME>"` must have its `expr`
// field equal to the const value verbatim. Files tagged
// `{"convox_source_check": "upstream"}` at the dashboard root are skipped for
// forward parity (their panels reference upstream metrics, not Convox consts).
//
// JSON-decode each file's panel targets so JSON-escaped quote characters
// (e.g. `\"$namespace\"` in the on-disk bytes) compare against the unescaped
// Go const value. Comparing via raw byte-level strings.Contains would have
// failed for any const containing literal `"` characters.
func TestPromQLConstantsAppearInJsonDashboards(t *testing.T) {
	files, err := filepath.Glob(promQLDashboardsGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no JSON dashboards present — skipping forward-parity test")
	}
	consts := AllPromQLConstants()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		if isUpstreamExempt(data) {
			continue
		}
		targets, err := extractPanelTargets(data)
		if err != nil {
			t.Errorf("parse panel targets in %s: %v", f, err)
			continue
		}
		for _, tgt := range targets {
			if tgt.Source == "" {
				continue
			}
			if strings.HasPrefix(tgt.Source, "upstream:") || strings.HasPrefix(tgt.Source, "convox-authored:") {
				continue
			}
			expectedQuery, ok := consts[tgt.Source]
			if !ok {
				t.Errorf("file %s panel target _source=%q is not a known Go const, upstream tag, or convox-authored tag", f, tgt.Source)
				continue
			}
			if tgt.Expr != expectedQuery {
				t.Errorf("file %s _source=%q expr drift:\n  json:  %q\n  go:    %q", f, tgt.Source, tgt.Expr, expectedQuery)
			}
		}
	}
}

// panelTarget is the (subset of) Grafana panel-target schema relevant for
// parity: the canonical PromQL `expr` plus the Convox-authored `_source`
// annotation pointing at the Go const name (or upstream tag).
type panelTarget struct {
	Expr   string `json:"expr"`
	Source string `json:"_source"`
}

// extractPanelTargets walks the dashboard JSON and yields every panel
// target (incl. nested rows). It returns ALL targets; callers decide which
// to skip based on tags.
func extractPanelTargets(data []byte) ([]panelTarget, error) {
	var dash struct {
		Panels []struct {
			Targets []panelTarget `json:"targets"`
			Panels  []struct {
				Targets []panelTarget `json:"targets"`
			} `json:"panels"`
		} `json:"panels"`
	}
	if err := json.Unmarshal(data, &dash); err != nil {
		return nil, err
	}
	var out []panelTarget
	for _, p := range dash.Panels {
		out = append(out, p.Targets...)
		for _, np := range p.Panels {
			out = append(out, np.Targets...)
		}
	}
	return out, nil
}

// TestEveryPanelTargetHasSource: reverse parity. Every panel target in every
// JSON dashboard MUST carry an `_source` annotation that is one of:
//   - a Go const name (looked up in AllPromQLConstants())
//   - an `upstream:*` exemption prefix (validUpstreamSourcePrefixes)
//   - a `convox-authored:*` extension tag
//
// Upstream-tagged dashboards are NOT skipped at the file level — convox-authored
// panels added to upstream JSONs (e.g. vLLM KV-cache extensions) still get
// checked. Empty / whitespace-only `_source` values fail.
//
// Skip behavior matches TestPromQLConstantsAppearInJsonDashboards.
func TestEveryPanelTargetHasSource(t *testing.T) {
	files, err := filepath.Glob(promQLDashboardsGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Skip("no JSON dashboards present — skipping reverse-parity test")
	}
	consts := AllPromQLConstants()
	prefixes := validUpstreamSourcePrefixes()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		var dash map[string]interface{}
		if err := json.Unmarshal(data, &dash); err != nil {
			t.Errorf("file %s: invalid JSON: %v", f, err)
			continue
		}
		panels := flattenPanels(dash)
		for i, p := range panels {
			targets, _ := p["targets"].([]interface{})
			for j, target := range targets {
				tgt, _ := target.(map[string]interface{})
				expr, hasExpr := tgt["expr"].(string)
				if !hasExpr || strings.TrimSpace(expr) == "" {
					continue
				}
				src, hasSrc := tgt["_source"].(string)
				if !hasSrc || strings.TrimSpace(src) == "" {
					t.Errorf("file %s panel %d target %d missing or empty _source annotation; expr=%q", f, i, j, expr)
					continue
				}
				if isValidUpstreamSource(src, prefixes) {
					continue
				}
				if strings.HasPrefix(src, "convox-authored:") {
					continue
				}
				if _, ok := consts[src]; !ok {
					t.Errorf("file %s panel %d target %d _source %q is neither a Go const name nor an upstream:*/convox-authored:* tag", f, i, j, src)
				}
			}
		}
	}
}

// flattenPanels walks a Grafana dashboard JSON's panels[] (and any nested
// row-collapsed `panels[].panels[]` for Grafana 9+ row collapses), returning a
// flat list of leaf panels in deterministic order.
func flattenPanels(dash map[string]interface{}) []map[string]interface{} {
	out := []map[string]interface{}{}
	raw, _ := dash["panels"].([]interface{})
	for _, p := range raw {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		// Row collapse — Grafana nests child panels under the row's own
		// "panels" array when rendered collapsed.
		if t, _ := pm["type"].(string); t == "row" {
			children, _ := pm["panels"].([]interface{})
			for _, c := range children {
				cm, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				out = append(out, cm)
			}
			continue
		}
		out = append(out, pm)
	}
	return out
}

// isUpstreamExempt: the `convox_source_check: "upstream"` tag at the dashboard
// root flags a re-hosted upstream JSON. Forward-parity test skips these.
func isUpstreamExempt(data []byte) bool {
	var dash map[string]interface{}
	if err := json.Unmarshal(data, &dash); err != nil {
		return false
	}
	v, ok := dash["convox_source_check"].(string)
	return ok && v == "upstream"
}

func isValidUpstreamSource(src string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(src, p) {
			return true
		}
	}
	return false
}
