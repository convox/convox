package k8s_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// dcgmExporterChartVersion is the helm-chart version pinned by
// terraform/cluster/aws/variables.tf::gpu_observability_chart_version
// (default value). The whitelist below is the set of DCGM exporter
// counter names recognized by THIS chart version's runtime binary
// (dcgm-exporter image bundled with chart 4.8.1 = nvidia/k8s/dcgm-exporter:4.5.2-4.8.1).
//
// When the chart version bumps, both this constant AND the whitelist
// below MUST update in lockstep. The chart-bump checklist at
// docs/configuration/rack-parameters/aws/gpu_observability_chart_version.md
// is now CI-enforced via TestDcgmExporterCSV_ChartVersionPin below.
const dcgmExporterChartVersion = "4.8.1"

// validDCGMFields is the set of DCGM exporter counter field names
// valid for chart 4.8.1's runtime binary. Derived from NVIDIA's
// upstream etc/dcgm-exporter/default-counters.csv (the chart's bundled
// allowed-fields list) plus the binary's internal counter enum (see
// go-dcgm v4.5 pkg/dcgm/const_fields.go for the canonical name → ID map).
//
// The list deliberately INCLUDES fields the rack's override CSV does
// not currently enable — having a permissive whitelist that matches
// "what the chart accepts" is the right design: the test gates against
// "names the runtime will reject", NOT against "names we currently
// emit". When a new feature wants to enable a previously-disabled
// metric, that's a one-line CSV add — no whitelist change needed
// (because the field was already in the chart's binary).
//
// What this whitelist does NOT cover: a hypothetical future chart
// version that adds a new field name. Such a name would be unknown
// here and the test would fail until the whitelist is updated. That
// is the desired behavior — chart bumps must run the validation
// step in the chart-bump checklist.
var validDCGMFields = map[string]struct{}{
	// Clocks
	"DCGM_FI_DEV_SM_CLOCK":  {},
	"DCGM_FI_DEV_MEM_CLOCK": {},

	// Temperature
	"DCGM_FI_DEV_MEMORY_TEMP": {},
	"DCGM_FI_DEV_GPU_TEMP":    {},

	// Power
	"DCGM_FI_DEV_POWER_USAGE":              {},
	"DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION": {},

	// PCIE
	"DCGM_FI_DEV_PCIE_REPLAY_COUNTER": {},

	// Utilization
	"DCGM_FI_DEV_GPU_UTIL":      {},
	"DCGM_FI_DEV_MEM_COPY_UTIL": {},
	"DCGM_FI_DEV_ENC_UTIL":      {},
	"DCGM_FI_DEV_DEC_UTIL":      {},

	// Errors and violations
	"DCGM_FI_DEV_XID_ERRORS":           {},
	"DCGM_FI_DEV_CLOCKS_EVENT_REASONS": {},

	// Memory usage
	"DCGM_FI_DEV_FB_FREE":     {},
	"DCGM_FI_DEV_FB_USED":     {},
	"DCGM_FI_DEV_FB_RESERVED": {},

	// ECC
	"DCGM_FI_DEV_ECC_SBE_VOL_TOTAL": {},
	"DCGM_FI_DEV_ECC_DBE_VOL_TOTAL": {},

	// NVLink
	"DCGM_FI_DEV_NVLINK_REPLAY_ERROR_COUNT_TOTAL": {},
	"DCGM_FI_DEV_NVLINK_BANDWIDTH_TOTAL":          {},

	// Remapped rows
	"DCGM_FI_DEV_UNCORRECTABLE_REMAPPED_ROWS": {},
	"DCGM_FI_DEV_CORRECTABLE_REMAPPED_ROWS":   {},
	"DCGM_FI_DEV_ROW_REMAP_FAILURE":           {},

	// vGPU
	"DCGM_FI_DEV_VGPU_LICENSE_STATUS": {},

	// Driver
	"DCGM_FI_DRIVER_VERSION": {},

	// Profiling
	"DCGM_FI_PROF_GR_ENGINE_ACTIVE":   {},
	"DCGM_FI_PROF_SM_ACTIVE":          {},
	"DCGM_FI_PROF_PIPE_TENSOR_ACTIVE": {},
	"DCGM_FI_PROF_DRAM_ACTIVE":        {},
	"DCGM_FI_PROF_PIPE_FP64_ACTIVE":   {},
	"DCGM_FI_PROF_PIPE_FP32_ACTIVE":   {},
	"DCGM_FI_PROF_PIPE_FP16_ACTIVE":   {},
	"DCGM_FI_PROF_PCIE_TX_BYTES":      {},
	"DCGM_FI_PROF_PCIE_RX_BYTES":      {},
}

// deprecatedDCGMFields catches known-deprecated names that have been
// renamed in dcgm-exporter 4.x. A field present in the rack CSV that
// matches a key here fails the test with a remediation hint pointing
// at the new name. This is the explicit catch for the rc15 regression
// (DCGM_FI_DEV_CLOCK_THROTTLE_REASONS → DCGM_FI_DEV_CLOCKS_EVENT_REASONS)
// — adding a deprecated name to the CSV would have been caught in CI
// instead of crashlooping every customer's exporter pod.
//
// Add new entries here when NVIDIA deprecates more fields in future
// DCGM major versions. Fields removed entirely (no replacement) should
// have an empty replacement string with a comment explaining the cause.
var deprecatedDCGMFields = map[string]string{
	"DCGM_FI_DEV_CLOCK_THROTTLE_REASONS": "DCGM_FI_DEV_CLOCKS_EVENT_REASONS",
}

// fieldNamePattern matches the first column of a non-comment CSV line.
// CSV format from terraform/cluster/aws/files/dcp-metrics-included.csv:
//
//	DCGM_FIELD_NAME, gauge|counter|label, help message text
//
// Whitespace tolerance: leading whitespace skipped via TrimSpace; the
// FIELD,_PromType comma may have surrounding spaces.
var fieldNamePattern = regexp.MustCompile(`^(DCGM_FI_[A-Z0-9_]+)\s*,`)

// TestDcgmExporterCSV_FieldsValid validates that every field name in
// the rack's DCGM override CSV (terraform/cluster/aws/files/dcp-metrics-included.csv)
// is recognized by the dcgm-exporter binary pinned via
// gpu_observability_chart_version.
//
// Without this test, a deprecated or typo'd field name crashloops every
// customer's exporter pod on rack startup with the error:
//
//	"could not find DCGM field; err: unknown ExporterCounter field <name>"
//
// killing all GPU scrape output rack-wide. The PR-time check makes the
// regression visible in CI seconds after introduction instead of hours
// after rack deploy. Backstops the lesson learned in the rc15 cycle
// where DCGM_FI_DEV_CLOCK_THROTTLE_REASONS slipped into the CSV after
// being deprecated upstream.
func TestDcgmExporterCSV_FieldsValid(t *testing.T) {
	csvPath := filepath.Join("..", "..", "terraform", "cluster", "aws", "files", "dcp-metrics-included.csv")
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read CSV at %s: %v", csvPath, err)
	}

	for lineno, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := fieldNamePattern.FindStringSubmatch(trimmed)
		if m == nil {
			t.Errorf("line %d: malformed CSV row (does not start with DCGM_FI_*,): %q", lineno+1, trimmed)
			continue
		}
		field := m[1]
		if newName, deprecated := deprecatedDCGMFields[field]; deprecated {
			if newName != "" {
				t.Errorf("line %d: deprecated DCGM field %q — replace with %q (this name was removed in DCGM 4.x; the runtime exporter binary rejects it on startup, crashlooping the daemonset)", lineno+1, field, newName)
			} else {
				t.Errorf("line %d: deprecated DCGM field %q has no replacement and must be removed from the CSV", lineno+1, field)
			}
			continue
		}
		if _, ok := validDCGMFields[field]; !ok {
			t.Errorf("line %d: unknown DCGM field %q — not in the whitelist for chart %s. If this is a new field added by a chart bump, run the chart-bump checklist (docs/configuration/rack-parameters/aws/gpu_observability_chart_version.md) and add the name to validDCGMFields in this test file. If this is a typo, the runtime exporter would reject it on startup with 'unknown ExporterCounter field'.", lineno+1, field, dcgmExporterChartVersion)
		}
	}
}

// TestDcgmExporterCSV_ChartVersionPin pins the helm chart version
// referenced by the whitelist in TestDcgmExporterCSV_FieldsValid above
// across ALL THREE locations the rack source hardcodes the version
// string:
//
//  1. terraform/cluster/aws/variables.tf — the rack-param variable's
//     default value (what users see when they `convox rack params`).
//  2. terraform/cluster/aws/dcgm.tf — the helm_release `coalesce(var, "X")`
//     fallback that fires when the user clears the rack param.
//  3. terraform/system/aws/telemetry.tf — the system-tier params.yaml
//     default that surfaces the value to other rack-internal code paths.
//
// All three MUST agree with each other and with dcgmExporterChartVersion
// in this file. A maintainer who bumps the variables.tf default but
// forgets the coalesce fallback ships a rack that installs the OLD
// chart (because var.gpu_observability_chart_version unset → falls
// through to the literal). This test catches that drift in CI.
//
// Forces the chart-bump checklist at
// docs/configuration/rack-parameters/aws/gpu_observability_chart_version.md
// from advisory-only ("re-audit on bump") to CI-enforced.
func TestDcgmExporterCSV_ChartVersionPin(t *testing.T) {
	cases := []struct {
		path string
		re   *regexp.Regexp
		desc string
	}{
		{
			filepath.Join("..", "..", "terraform", "cluster", "aws", "variables.tf"),
			regexp.MustCompile(`(?s)variable\s+"gpu_observability_chart_version".*?default\s*=\s*"([^"]+)"`),
			"variables.tf rack-param default",
		},
		{
			filepath.Join("..", "..", "terraform", "cluster", "aws", "dcgm.tf"),
			regexp.MustCompile(`coalesce\(var\.gpu_observability_chart_version,\s*"([^"]+)"\)`),
			"dcgm.tf helm_release coalesce fallback",
		},
		{
			filepath.Join("..", "..", "terraform", "system", "aws", "telemetry.tf"),
			regexp.MustCompile(`gpu_observability_chart_version\s*=\s*"([^"]+)"`),
			"telemetry.tf system-tier params default",
		},
	}
	for _, c := range cases {
		data, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read %s (%s): %v", c.desc, c.path, err)
		}
		m := c.re.FindStringSubmatch(string(data))
		if m == nil {
			t.Fatalf("%s: could not find chart-version literal in %s — has the file been renamed or restructured?", c.desc, c.path)
		}
		if m[1] != dcgmExporterChartVersion {
			t.Errorf("%s: chart version literal is %q but the test whitelist is pinned to %q. Before bumping dcgmExporterChartVersion in this test, validate the whitelist against the new chart's etc/dcgm-exporter/default-counters.csv (run the chart-bump checklist at docs/configuration/rack-parameters/aws/gpu_observability_chart_version.md). All three locations (variables.tf, dcgm.tf, telemetry.tf) MUST agree; a maintainer who bumps one but forgets the others ships a rack that installs an inconsistent version mix.", c.desc, m[1], dcgmExporterChartVersion)
		}
	}
}

// TestDcgmExporterFieldsInGoSourceMatchWhitelist scans the rack's Go
// source for hardcoded DCGM_FI_* token references and asserts every
// one is present in validDCGMFields. Catches typos and stale names in:
//
//   - provider/k8s/prometheus_queries.go (PromQL query constants)
//   - provider/k8s/prometheus.go (the GpuMetrics parser table)
//   - pkg/manifest/service.go (KEDA autoscaler default Prometheus
//     metric — highest-blast-radius site, a typo there breaks every
//     customer's GPU autoscaler)
//
// A typo or upstream rename in any of these files would silently
// produce empty dashboards (the PromQL would query a metric Prometheus
// never emits; the parser table would silently no-op on the unknown
// name). The CSV-side test alone does NOT catch this — it gates the
// CSV against the whitelist but does not gate the Go source against
// the same whitelist. This test closes the round-trip.
//
// Comment-stripping: the scanner trims `//`-prefixed line comments
// before token-matching so historical-rename narratives in code
// comments (e.g. "the field formerly named DCGM_FI_DEV_CLOCK_THROTTLE_REASONS")
// don't trigger false positives. Block comments `/* */` are not
// stripped — the rack source does not use them for DCGM references,
// and stripping them in regex without a real Go parser is fragile.
// If a future contributor adds a block-comment historical reference,
// the test will fail and the contributor can either rewrite as `//`
// or extend the stripping.
func TestDcgmExporterFieldsInGoSourceMatchWhitelist(t *testing.T) {
	files := []string{
		filepath.Join("prometheus_queries.go"),
		filepath.Join("prometheus.go"),
		filepath.Join("..", "..", "pkg", "manifest", "service.go"),
	}
	tokenRe := regexp.MustCompile(`DCGM_FI_[A-Z0-9_]+`)
	commentRe := regexp.MustCompile(`//.*$`)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		// Strip line comments per-line so historical-rename narratives
		// in code comments don't false-positive the live-reference scan.
		var stripped strings.Builder
		for _, line := range strings.Split(string(data), "\n") {
			stripped.WriteString(commentRe.ReplaceAllString(line, ""))
			stripped.WriteString("\n")
		}
		seen := map[string]struct{}{}
		for _, m := range tokenRe.FindAllString(stripped.String(), -1) {
			if _, alreadySeen := seen[m]; alreadySeen {
				continue
			}
			seen[m] = struct{}{}
			if newName, deprecated := deprecatedDCGMFields[m]; deprecated {
				if newName != "" {
					t.Errorf("%s: deprecated DCGM field %q referenced — replace with %q", f, m, newName)
				} else {
					t.Errorf("%s: deprecated DCGM field %q has no replacement and must be removed", f, m)
				}
				continue
			}
			if _, ok := validDCGMFields[m]; !ok {
				t.Errorf("%s: DCGM field %q referenced but not in the whitelist for chart %s — typo or stale rename? If a new chart-supported field, add it to validDCGMFields after running the chart-bump checklist.", f, m, dcgmExporterChartVersion)
			}
		}
	}
}
