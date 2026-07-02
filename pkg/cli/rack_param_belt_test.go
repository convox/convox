package cli

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/rack"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// TestWhitespaceNormalizationOnClearableParams verifies that whitespace-only
// inputs to clearable params are normalized to empty string before reaching
// writeVars / TF coalesce. Without this, `convox rack params set X=" "`
// would write " " to vars.json, defeating preserveEmpty (which only acts on
// TrimSpace-empty entries) AND the TF coalesce guard (which treats " " as
// non-empty).
func TestWhitespaceNormalizationOnClearableParams(t *testing.T) {
	cases := []struct {
		name  string
		param string
		input string
		want  string
	}{
		{"empty", "prometheus_url", "", ""},
		{"single-space", "prometheus_url", " ", ""},
		{"tabs-and-spaces", "prometheus_url", " \t\n ", ""},
		{"chart-version-empty", "prometheus_gpu_metrics_chart_version", "", ""},
		{"chart-version-whitespace", "prometheus_gpu_metrics_chart_version", "   ", ""},
		{"non-clearable-empty-rejected", "cost_tracking_enable", "", "REJECT"},
		{"non-clearable-whitespace-rejected", "cost_tracking_enable", " ", "REJECT"},
		{"clearable-non-empty-passthrough", "prometheus_url", "https://prom.kube-system.svc.cluster.local", "https://prom.kube-system.svc.cluster.local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{tc.param: tc.input}
			err := validateAndMutateParams(params, "aws", nil, false)
			if tc.want == "REJECT" {
				if err == nil {
					t.Errorf("input %q for %q should be rejected, got %q", tc.input, tc.param, params[tc.param])
				}
				return
			}
			if err != nil {
				t.Fatalf("input %q for %q: unexpected error %v", tc.input, tc.param, err)
			}
			if got := params[tc.param]; got != tc.want {
				t.Errorf("input %q for %q: got %q, want %q", tc.input, tc.param, got, tc.want)
			}
		})
	}
}

// paramGroupsExcluded enumerates settable params intentionally absent from
// any paramGroup. These are managed-internally, install-immutable, or
// internal feature gates surfaced only via the default unfiltered view.
//
// MAINTENANCE: this allowlist is the architectural decision boundary. Adding
// a new entry IS an architectural change — review with spec authors before
// merging. Its size is asserted in TestParamGroupsCoverage to fail loudly
// when a new entry is added without explicit signoff.
var paramGroupsExcluded = map[string]bool{
	"image": true, "name": true, "rack_name": true, "release": true,
	"settings": true, "sync_tf_now": true,
	"private": true, "region": true, "availability_zones": true,
	"api_feature_gates": true,
}

// TestParamGroupsCoverage asserts every entry of every providerKnownParams
// map is in some paramGroups entry (and that every paramGroups key has a
// description), except for the documented allowlist. Catches future PRs
// that add a settable param without grouping it.
func TestParamGroupsCoverage(t *testing.T) {
	if got, want := len(paramGroupsExcluded), 10; got != want {
		t.Fatalf("paramGroupsExcluded has %d entries; expected exactly %d. "+
			"If you added an entry, that is an architectural change — review "+
			"with spec authors and update this assertion.", got, want)
	}
	grouped := make(map[string]bool)
	for groupName, members := range paramGroups {
		if _, ok := groupDescriptions[groupName]; !ok {
			t.Errorf("paramGroups[%q] has no entry in groupDescriptions", groupName)
		}
		for k := range members {
			grouped[k] = true
		}
	}
	var missing []string
	for provider, known := range providerKnownParams {
		for k := range known {
			if paramGroupsExcluded[k] || grouped[k] {
				continue
			}
			missing = append(missing, provider+":"+k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("settable params missing from paramGroups (and not in paramGroupsExcluded): %v", missing)
	}
}

// TestClearableMatchesPreserveEmpty asserts the rack-side clearableParams
// (pkg/cli/rack.go) and the writeVars preserveEmpty (pkg/rack/terraform.go)
// stay in sync. Both maps are reflected from their source-of-truth
// declarations — drift in either direction trips the test.
//
// jsonConfigCarveouts are clearable but NOT in preserveEmpty because the
// CLI base64-encodes them before writeVars runs (so empty stripping is a
// no-op for those keys regardless of preserveEmpty membership).
func TestClearableMatchesPreserveEmpty(t *testing.T) {
	preserveEmpty := rack.PreserveEmptyParams()
	jsonConfigCarveouts := map[string]bool{
		"additional_node_groups_config":         true,
		"additional_build_groups_config":        true,
		"additional_karpenter_nodepools_config": true,
		"karpenter_config":                      true,
	}
	for k := range clearableParams {
		if jsonConfigCarveouts[k] {
			continue
		}
		if !preserveEmpty[k] {
			t.Errorf("clearable param %q (rack.go) missing from preserveEmpty (terraform.go)", k)
		}
	}
	for k := range preserveEmpty {
		if !clearableParams[k] {
			t.Errorf("preserveEmpty entry %q (terraform.go) missing from clearableParams (rack.go)", k)
		}
	}
}

// boolParamsUntypedAllowlist enumerates boolParams entries that
// TestBoolParamsHaveTFType (AWS-scoped) skips. Two reasons can land an
// entry here:
//   - "azure_files_enable": Azure-only variable, not declared in
//     terraform/system/aws/variables.tf at all. Type-correctness in
//     terraform/system/azure/variables.tf is out of scope for this test.
//   - "build_disable_convox_resolver": declared in aws/ but intentionally
//     untyped because adding type=bool today would trigger the TF
//     state-fingertrap on downgrade for users with non-canonical bool
//     strings persisted from earlier versions.
//
// Size is asserted to prevent drive-by additions; remove an entry only
// when fixing the underlying TF variable upstream AND extending the test
// scope to cover that provider.
var boolParamsUntypedAllowlist = map[string]bool{
	"azure_files_enable":            true,
	"build_disable_convox_resolver": true,
}

// TestBoolParamsHaveTFType asserts every boolParams entry has type=bool in
// terraform/system/aws/variables.tf, except the documented allowlist. AWS
// scope is intentional — boolParams is a global rack-side validation set
// that must pass through the AWS rack module without TF type errors.
// Other providers' variables.tf are not parsed here.
func TestBoolParamsHaveTFType(t *testing.T) {
	if got, want := len(boolParamsUntypedAllowlist), 2; got != want {
		t.Fatalf("boolParamsUntypedAllowlist has %d entries; expected exactly %d. "+
			"Adding an entry is an architectural decision — review the spec "+
			"reasoning. Removing one means upgrading the TF variable to type=bool.",
			got, want)
	}

	parser := hclparse.NewParser()
	const path = "../../terraform/system/aws/variables.tf"
	f, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		t.Fatalf("hclparse %s: %v", path, diags)
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatalf("hclparse: failed to coerce body for %s", path)
	}
	fileBytes := parser.Files()[path].Bytes

	types := make(map[string]string)
	for _, block := range body.Blocks {
		if block.Type != "variable" || len(block.Labels) != 1 {
			continue
		}
		name := block.Labels[0]
		for _, attr := range block.Body.Attributes {
			if attr.Name != "type" {
				continue
			}
			src := string(attr.Expr.Range().SliceBytes(fileBytes))
			types[name] = strings.TrimSpace(src)
		}
	}

	for k := range boolParams {
		if boolParamsUntypedAllowlist[k] {
			// Forward-direction guard: if the allowlisted variable IS now
			// type=bool, fail loud so the allowlist gets pruned (rather
			// than silently accepting an outdated allowlist entry).
			if types[k] == "bool" {
				t.Errorf("boolParams entry %q is in boolParamsUntypedAllowlist but variables.tf now declares type=bool; remove the allowlist entry", k)
			}
			continue
		}
		decl, ok := types[k]
		if !ok {
			t.Errorf("boolParams entry %q has no `type` declaration in variables.tf", k)
			continue
		}
		if decl != "bool" {
			t.Errorf("boolParams entry %q declared as type=%q in variables.tf; expected `bool`", k, decl)
		}
	}
}

// TestBoolCanonicalization asserts ParseBool-acceptable variants are
// canonicalized to "true"/"false" AND non-acceptable inputs are rejected.
// String-typed bool-likes (karpenter_auth_mode, karpenter_enabled) MUST NOT
// be canonicalized — they pass through verbatim.
func TestBoolCanonicalization(t *testing.T) {
	canonCases := map[string]string{
		"1":     "true",
		"0":     "false",
		"t":     "true",
		"T":     "true",
		"f":     "false",
		"F":     "false",
		"true":  "true",
		"True":  "true",
		"TRUE":  "true",
		"false": "false",
		"False": "false",
		"FALSE": "false",
	}
	for in, want := range canonCases {
		params := map[string]string{"cost_tracking_enable": in}
		if err := validateAndMutateParams(params, "aws", nil, false); err != nil {
			t.Fatalf("input %q: unexpected error %v", in, err)
		}
		if got := params["cost_tracking_enable"]; got != want {
			t.Errorf("input %q: got %q, want %q", in, got, want)
		}
	}

	rejectCases := []string{"yes", "no", "on", "off", "y", "n", "x", "yo", "fals", "trueish", "maybe", " "}
	for _, invalid := range rejectCases {
		params := map[string]string{"cost_tracking_enable": invalid}
		if err := validateAndMutateParams(params, "aws", nil, false); err == nil {
			t.Errorf("input %q should be rejected but was accepted (canonicalized to %q)", invalid, params["cost_tracking_enable"])
		}
	}

	carveoutCases := map[string]string{
		"karpenter_auth_mode": "true",
		"karpenter_enabled":   "false",
	}
	for k, v := range carveoutCases {
		params := map[string]string{k: v}
		if err := validateAndMutateParams(params, "aws", nil, false); err != nil {
			t.Fatalf("carve-out %s=%q: unexpected error %v", k, v, err)
		}
		if got := params[k]; got != v {
			t.Errorf("carve-out %s: got %q, want %q (must NOT canonicalize)", k, got, v)
		}
	}
}

// TestCoalesceLiteralsMatchTFDefaults asserts that the literal default in
// each `coalesce(var.X, "literal")` site under terraform/cluster/aws/
// matches the TF variable's declared default in BOTH
// terraform/cluster/aws/variables.tf AND terraform/system/aws/variables.tf.
//
// Three-way invariant: cluster default → system default → coalesce literal
// must all agree. The system module passes var.X through to the cluster
// module, so a system→cluster drift means fresh installs see the system
// default while cleared params fall through to the coalesce literal — a
// silent split between cohorts.
func TestCoalesceLiteralsMatchTFDefaults(t *testing.T) {
	coalesceSites := []struct {
		variable string
		file     string
	}{
		{"gpu_observability_chart_version", "../../terraform/cluster/aws/dcgm.tf"},
		{"dcgm_scrape_interval", "../../terraform/cluster/aws/dcgm.tf"},
	}

	clusterDefaults := parseTFVariableDefaults(t, "../../terraform/cluster/aws/variables.tf")
	systemDefaults := parseTFVariableDefaults(t, "../../terraform/system/aws/variables.tf")

	for _, site := range coalesceSites {
		clusterDefault, ok := clusterDefaults[site.variable]
		if !ok {
			t.Errorf("variable %q has no `default = \"...\"` declaration in cluster/aws/variables.tf", site.variable)
			continue
		}
		systemDefault, ok := systemDefaults[site.variable]
		if !ok {
			t.Errorf("variable %q has no `default = \"...\"` declaration in system/aws/variables.tf", site.variable)
			continue
		}
		if clusterDefault != systemDefault {
			t.Errorf("variable %q default drift: cluster/aws/variables.tf = %q vs system/aws/variables.tf = %q",
				site.variable, clusterDefault, systemDefault)
		}
		got, ok := parseCoalesceLiteral(t, site.file, site.variable)
		if !ok {
			// Hard fail: a missing site is either a regex-tolerance gap
			// (terraform fmt produced a layout the regex doesn't match) or
			// a removed coalesce guard (the protection this test exists to
			// verify is gone). Either way drift detection is dormant —
			// halt the run rather than silently passing the cluster-vs-system
			// default check below.
			t.Fatalf("variable %q has no coalesce(var.%s, \"...\") site in %s — regex-tolerance gap or removed guard",
				site.variable, site.variable, site.file)
		}
		if got != clusterDefault {
			t.Errorf("coalesce literal for var.%s in %s is %q; cluster/aws/variables.tf declares default = %q (drift)",
				site.variable, site.file, got, clusterDefault)
		}
	}
}

// parseTFVariableDefaults returns variableName → defaultLiteral for every
// variable block in path that declares a string default.
func parseTFVariableDefaults(t *testing.T, path string) map[string]string {
	t.Helper()
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		t.Fatalf("hclparse %s: %v", path, diags)
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		t.Fatalf("hclparse: failed to coerce body for %s", path)
	}
	fileBytes := parser.Files()[path].Bytes

	out := make(map[string]string)
	for _, block := range body.Blocks {
		if block.Type != "variable" || len(block.Labels) != 1 {
			continue
		}
		name := block.Labels[0]
		for _, attr := range block.Body.Attributes {
			if attr.Name != "default" {
				continue
			}
			src := strings.TrimSpace(string(attr.Expr.Range().SliceBytes(fileBytes)))
			if len(src) >= 2 && src[0] == '"' && src[len(src)-1] == '"' {
				out[name] = src[1 : len(src)-1]
			}
		}
	}
	return out
}

// parseCoalesceLiteral scans path for a `coalesce(var.<variable>, "<literal>")`
// expression and returns the literal. Whitespace-tolerant across the
// formatter variations terraform fmt produces:
//   - single-line:                     `coalesce(var.X, "Y")`
//   - multi-line with trailing comma:  `coalesce(\n  var.X,\n  "Y",\n)`
//   - multi-line without trailing comma
//
func TestRouterTypeEnumValidation(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantErr bool
		wantVal string
	}{
		{"nginx", "nginx", false, "nginx"},
		{"contour", "contour", false, "contour"},
		{"uppercase", "CONTOUR", false, "contour"},
		{"mixed-case", "Nginx", false, "nginx"},
		{"invalid-istio", "istio", true, ""},
		{"invalid-both", "both", true, ""},
		{"empty-rejected", "", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]string{"router_type": tc.value}
			err := validateAndMutateParams(params, "aws", nil, false)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantVal != "" && params["router_type"] != tc.wantVal {
				t.Errorf("got %q, want %q", params["router_type"], tc.wantVal)
			}
		})
	}
}

func TestRouterTypeRejectedOnGCP(t *testing.T) {
	params := map[string]string{"router_type": "contour"}
	err := validateAndMutateParams(params, "gcp", nil, false)
	if err == nil {
		t.Fatal("expected error for router_type on GCP, got nil")
	}
}

func TestIsVersionLessThan(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"3.25.0", "3.25.1", true},
		{"3.25.1", "3.25.0", false},
		{"3.25.0", "3.25.0", false},
		{"3.25.0-rc1", "3.25.0", false},
		{"3.24.6", "3.25.0", true},
		{"invalid", "3.25.0", false},
		{"3.25", "3.25.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			if got := isVersionLessThan(tc.a, tc.b); got != tc.want {
				t.Errorf("isVersionLessThan(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// `(?s)` makes `\s` span newlines; the trailing `,?` accepts the optional
// comma terraform fmt emits between the last argument and the close paren
// in multi-line layouts. Reports false if no such site exists.
func parseCoalesceLiteral(t *testing.T, path, variable string) (string, bool) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	pattern := `(?s)coalesce\(\s*var\.` + regexp.QuoteMeta(variable) + `\s*,\s*"([^"]*)"\s*,?\s*\)`
	re := regexp.MustCompile(pattern)
	m := re.FindSubmatch(src)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

func TestValidateAndMutateParams_PodSecurity(t *testing.T) {
	accept := []map[string]string{
		{"pod_security_standard": "baseline"},
		{"pod_security_standard": "restricted"},
		{"pod_security_standard": ""},
		{"pod_security_mode": "warn"},
		{"pod_security_mode": "audit"},
		{"pod_security_mode": "enforce"},
		{"pod_security_standard": "baseline", "pod_security_mode": "warn"},
		{"pod_security_standard": "restricted", "pod_security_mode": "enforce"},
	}
	for _, p := range accept {
		if err := validateAndMutateParams(p, "aws", map[string]string{}, false); err != nil {
			t.Errorf("expected accept for %v, got %v", p, err)
		}
	}

	if err := validateAndMutateParams(
		map[string]string{"pod_security_mode": "enforce"}, "aws",
		map[string]string{"pod_security_standard": "baseline"}, false,
	); err != nil {
		t.Errorf("expected accept for enforce with effective standard, got %v", err)
	}

	reject := []map[string]string{
		{"pod_security_standard": "bogus"},
		{"pod_security_standard": "Baseline"},
		{"pod_security_standard": "BASELINE"},
		{"pod_security_standard": " baseline"},
		{"pod_security_mode": "bogus"},
		{"pod_security_mode": "Enforce"},
		{"pod_security_mode": ""},
	}
	for _, p := range reject {
		if err := validateAndMutateParams(p, "aws", map[string]string{}, false); err == nil {
			t.Errorf("expected reject for %v, got nil", p)
		}
	}
}
