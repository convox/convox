package k8s_test

import (
	"bufio"
	"encoding/csv"
	"fmt"
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
// at the new name. This is the explicit catch for the
// DCGM_FI_DEV_CLOCK_THROTTLE_REASONS → DCGM_FI_DEV_CLOCKS_EVENT_REASONS
// rename — adding a deprecated name to the CSV would have been caught
// in CI instead of crashlooping every user's exporter pod.
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
// user's exporter pod on rack startup with the error:
//
//	"could not find DCGM field; err: unknown ExporterCounter field <name>"
//
// killing all GPU scrape output rack-wide. The PR-time check makes the
// regression visible in CI seconds after introduction instead of hours
// after rack deploy. Backstops the failure mode where
// DCGM_FI_DEV_CLOCK_THROTTLE_REASONS slipped into the CSV after being
// deprecated upstream — the runtime exporter rejects the unknown name
// on startup, killing every user's GPU scrape.
func TestDcgmExporterCSV_FieldsValid(t *testing.T) {
	csvPath := filepath.Join("..", "..", "terraform", "cluster", "aws", "files", "dcp-metrics-included.csv")
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("read CSV at %s: %v", csvPath, err)
	}

	// Parse with the same encoding/csv package the dcgm-exporter binary
	// uses. Catches CSV-syntax errors (bare embedded quotes in non-quoted
	// fields, mismatched columns, malformed escape sequences) at PR time
	// instead of in the running pod's startup. Without this end-to-end
	// parse, a syntactically broken CSV would still pass the field-name
	// regex below — narrow scope, wrong-shaped safety net.
	//
	// Strip comment lines first because encoding/csv has no native
	// comment support; the dcgm-exporter parser does the same prep.
	var prepped strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		prepped.WriteString(line)
		prepped.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan CSV: %v", err)
	}
	csvR := csv.NewReader(strings.NewReader(prepped.String()))
	csvR.FieldsPerRecord = -1 // help text often contains commas; don't enforce equal field count
	if _, err := csvR.ReadAll(); err != nil {
		t.Fatalf("CSV parse error — would crashloop the dcgm-exporter pod on startup with the same error: %v", err)
	}

	// Field-name validation pass — uses the regex parser intentionally
	// because we only want the first column (the field NAME) and the
	// help-text column may contain commas / parens that confuse a naive
	// per-record split. The encoding/csv parse above guarantees the file
	// is well-formed; this pass adds the whitelist semantics on top.
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

// TestDcgmExporterCSV_ArgsPathInvariant validates the chain of names
// and paths that together describe how the override CSV gets onto the
// exporter pod, and asserts they all agree. Drift in any link silently
// crashloops the DaemonSet at runtime with no surfaced TF/helm error.
//
// The chain (with field source in parentheses):
//
//  1. kubernetes_config_map.dcgm_metrics_convox.metadata.name
//     ($cmName — the canonical configMap object name).
//  2. kubernetes_config_map.dcgm_metrics_convox.data["<key>.csv"] = file(...)
//     ($cmDataKey — the LHS data key MUST equal the RHS basename
//     because the helm values reference the LHS, but the actual file
//     content comes from the RHS).
//  3. helm values extraConfigMapVolumes[name=$cmName] .configMap.name
//     (must equal $cmName; drift here produces "configmap not found").
//  4. extraConfigMapVolumes[name=$cmName] .items[].{key,path} (key must
//     equal $cmDataKey; path is relative filename inside the projected
//     volume; key/path may appear in either order in HCL).
//  5. helm values extraVolumeMounts[name=$cmName] .mountPath (the
//     absolute mount root in the container; anchored to the same
//     volume name to stay bound to THIS volume even if a future PR
//     adds sibling mounts).
//  6. helm values arguments[1] (the `-f <file>` arg passed to the
//     dcgm-exporter binary).
//
// Composite invariants:
//   - $itemKey == $cmDataKey (volume projection finds a matching
//     configMap data entry).
//   - $cmVolumeRefName == $cmName (volume projection points at the
//     correct configMap object).
//   - $argPath == join($mountPath, $itemPath) (exporter's -f file
//     path resolves to a file that actually exists on the pod).
//
// Anchoring the volume + mount lookups by `name = $cmName` (instead of
// taking the first match in the file) eliminates the pre-Angle-B
// hazard where a future PR could add another extraVolumeMount above the
// dcgm-csv mount and cause the test to silently validate the wrong
// (mountPath, mount-name) pair while real wiring drifts.
func TestDcgmExporterCSV_ArgsPathInvariant(t *testing.T) {
	tfPath := filepath.Join("..", "..", "terraform", "cluster", "aws", "dcgm.tf")
	data, err := os.ReadFile(tfPath)
	if err != nil {
		t.Fatalf("read %s: %v", tfPath, err)
	}
	src := string(data)

	// configMap data block. The `<key>.csv = file(...basename.csv)`
	// pattern MUST have LHS == RHS basename — partial renames that desynchronize
	// the two slip past a single-capture regex. Capturing both and asserting
	// equality catches that drift.
	cmDataKeyPair := regexp.MustCompile(`"([^"]+\.csv)"\s*=\s*file\("\$\{path\.module\}/files/([^"]+\.csv)"\)`)
	dataMatch := cmDataKeyPair.FindStringSubmatch(src)
	if dataMatch == nil {
		t.Fatalf("dcgm.tf: kubernetes_config_map data block: could not locate `<key>.csv = file(${path.module}/files/<file>.csv)` pattern. Has the file structure changed?")
	}
	if dataMatch[1] != dataMatch[2] {
		t.Errorf("dcgm.tf: configMap data key %q does not match referenced filename %q — partial rename, both must agree (helm values reference the LHS key; the file content comes from the RHS basename).", dataMatch[1], dataMatch[2])
	}
	cmDataKey := dataMatch[1]

	// kubernetes_config_map.dcgm_metrics_convox metadata.name —
	// the canonical configMap object name. All three downstream refs
	// (extraConfigMapVolumes[].configMap.name, extraConfigMapVolumes[].name,
	// extraVolumeMounts[].name) MUST equal this value.
	cmResRe := regexp.MustCompile(`(?s)resource\s+"kubernetes_config_map"\s+"dcgm_metrics_convox"\s*\{(.+?)\n\}`)
	cmResMatch := cmResRe.FindStringSubmatch(src)
	if cmResMatch == nil {
		t.Fatalf("dcgm.tf: could not extract kubernetes_config_map.dcgm_metrics_convox resource block")
	}
	cmName := mustExtractField(t, cmResMatch[1], "name", "kubernetes_config_map.dcgm_metrics_convox metadata.name")

	// extraConfigMapVolumes — anchor to the entry where
	// `name = $cmName`. extractListEntryByName walks the list section
	// `extraConfigMapVolumes = [...]` and returns the {...} entry
	// containing `name = $cmName`. Eliminates the first-match-wins hazard.
	cmVolEntry, ok := extractListEntryByName(src, "extraConfigMapVolumes", cmName)
	if !ok {
		t.Fatalf(`dcgm.tf: could not find an extraConfigMapVolumes entry with name = %q. Either the volume name drifted or the helm values structure was reshaped.`, cmName)
	}
	// extraConfigMapVolumes[].configMap.name MUST equal cmName.
	cmRefRe := regexp.MustCompile(`(?s)configMap\s*=\s*\{[^{}]*?name\s*=\s*"([^"]+)"`)
	cmRefMatch := cmRefRe.FindStringSubmatch(cmVolEntry)
	if cmRefMatch == nil {
		t.Fatalf(`dcgm.tf: extraConfigMapVolumes[name=%q]: missing configMap.name field`, cmName)
	}
	if cmRefMatch[1] != cmName {
		t.Errorf(`dcgm.tf: extraConfigMapVolumes[name=%q].configMap.name = %q — must match the kubernetes_config_map metadata.name (%q). Drift here produces "configmap not found" at pod startup, not a TF/helm error.`, cmName, cmRefMatch[1], cmName)
	}
	// items[] iteration — extract ALL items in the list, validate every
	// one. Pre-Angle-B-M2 the regex grabbed only items[0]; if a future PR
	// split the CSV across multiple items, items[1+] would be silently
	// unverified. extractAllListItems pulls every {...} entry between the
	// `items = [` and matching `]` so each item's key+path get checked.
	//
	// For each item: key MUST equal the configMap data key (so the volume
	// projection finds a real entry); path drives the in-mount filename.
	// HCL allows key/path in either order; sub-regexes pull each field
	// independently of declaration order.
	itemEntries, ok := extractAllListItems(cmVolEntry, "items")
	if !ok || len(itemEntries) == 0 {
		t.Fatalf(`dcgm.tf: extraConfigMapVolumes[name=%q]: missing items[] list`, cmName)
	}
	if len(itemEntries) != 1 {
		t.Logf("note: extraConfigMapVolumes[name=%q] has %d items — validating all of them", cmName, len(itemEntries))
	}
	// First item drives the (mountPath, itemPath) → arguments[1] composite
	// invariant below; additional items must still satisfy the
	// item.key == cmDataKey check (volume projection correctness).
	var itemKey, itemPath string
	for i, body := range itemEntries {
		k := mustExtractField(t, body, "key", fmt.Sprintf("extraConfigMapVolumes items[%d].key", i))
		p := mustExtractField(t, body, "path", fmt.Sprintf("extraConfigMapVolumes items[%d].path", i))
		if k != cmDataKey {
			t.Errorf(`extraConfigMapVolumes items[%d].key = %q does not match the configMap data block key %q. Volume projection won't find a matching configMap entry; the file silently won't appear at the mount path.`, i, k, cmDataKey)
		}
		if i == 0 {
			itemKey, itemPath = k, p
		}
	}

	// extraVolumeMounts — same anchoring strategy as the volume
	// lookup. Bound to the entry where `name = $cmName`.
	mountEntry, ok := extractListEntryByName(src, "extraVolumeMounts", cmName)
	if !ok {
		t.Fatalf(`dcgm.tf: could not find an extraVolumeMounts entry with name = %q.`, cmName)
	}
	mountPath := mustExtractField(t, mountEntry, "mountPath", "extraVolumeMounts mountPath")

	// arguments = ["-f", "<path>"] — the `-f` flag is what
	// points the exporter at the override CSV. Without it the exporter
	// falls back to the chart's bundled default-counters.csv and the
	// 9 Convox-required fields go silent.
	argRe := regexp.MustCompile(`arguments\s*=\s*\[\s*"-f"\s*,\s*"([^"]+)"\s*\]`)
	argMatch := argRe.FindStringSubmatch(src)
	if argMatch == nil {
		t.Fatalf(`dcgm.tf: missing arguments = ["-f", "<path>"] line — has the chart values shape changed?`)
	}
	argPath := argMatch[1]

	// Composite assertions.
	if itemKey != cmDataKey {
		t.Errorf(`configMap data-key drift: data block declares key %q; extraConfigMapVolumes items[0].key = %q. The volume projection won't find a matching configMap entry; the file silently won't appear at the mount path.`, cmDataKey, itemKey)
	}
	expectedArg := strings.TrimRight(strings.TrimSpace(mountPath), "/") + "/" + strings.TrimLeft(strings.TrimSpace(itemPath), "/")
	if argPath != expectedArg {
		t.Errorf(`dcgm-exporter -f arg drift: arguments[1]=%q but mountPath+itemPath compose to %q. The exporter will fail to open the file at startup with "open %s: no such file or directory" and crashloop. Update all three (mountPath, items[0].path, arguments[1]) in lockstep.`, argPath, expectedArg, argPath)
	}
}

// extractListEntryByName walks a `<listName> = [ {...}, {...}, ... ]`
// HCL list and returns the content (without outer braces) of the first
// {...} entry containing `name = "<wantName>"`. Used to anchor field
// extraction to a specific list entry rather than first-match-wins.
//
// Naive depth counting; does NOT skip braces inside HCL strings or
// comments. dcgm.tf has no string-embedded braces in these regions; if
// a future contributor introduces them, this helper may misbehave —
// the caller's failure message will point at this comment as a
// constructive failure mode.
func extractListEntryByName(src, listName, wantName string) (string, bool) {
	listRe := regexp.MustCompile(regexp.QuoteMeta(listName) + `\s*=\s*\[`)
	listLoc := listRe.FindStringIndex(src)
	if listLoc == nil {
		return "", false
	}
	// Find matching `]` to bound the list section.
	listStart := listLoc[1]
	listEnd := -1
	depth := 1
loop:
	for i := listStart; i < len(src); i++ {
		switch src[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				listEnd = i
				break loop
			}
		}
	}
	if listEnd == -1 {
		return "", false
	}
	listBody := src[listStart:listEnd]
	// Walk top-level {...} entries within the list.
	depth = 0
	entryStart := -1
	nameRe := regexp.MustCompile(`name\s*=\s*"` + regexp.QuoteMeta(wantName) + `"`)
	for i := 0; i < len(listBody); i++ {
		switch listBody[i] {
		case '{':
			if depth == 0 {
				entryStart = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 && entryStart >= 0 {
				entry := listBody[entryStart:i]
				if nameRe.MatchString(entry) {
					return entry, true
				}
				entryStart = -1
			}
		}
	}
	return "", false
}

// mustExtractField pulls a `<fieldName> = "<value>"` line out of an
// HCL block body. Whitespace tolerant; field-name boundary anchored
// on BOTH sides so partial-prefix collisions (e.g. extracting `name`
// must not match `namespace`) are rejected. Calls t.Fatalf if not
// found, with a descriptive context.
func mustExtractField(t *testing.T, blockBody, fieldName, contextDescription string) string {
	t.Helper()
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(fieldName) + `\b\s*=\s*"([^"]+)"`)
	m := re.FindStringSubmatch(blockBody)
	if m == nil {
		t.Fatalf("dcgm.tf: %s missing field %q", contextDescription, fieldName)
	}
	return m[1]
}

// extractAllListItems walks a `<listName> = [ {...}, {...}, ... ]` HCL
// list and returns the content (without outer braces) of every {...}
// entry. Used to validate properties of EVERY item rather than just
// the first. Caller is responsible for sub-extracting fields from each
// returned entry.
//
// Naive depth counting; same string/comment caveat as
// extractListEntryByName. For dcgm.tf's items list the structure is
// simple key=value pairs.
func extractAllListItems(src, listName string) ([]string, bool) {
	listRe := regexp.MustCompile(regexp.QuoteMeta(listName) + `\s*=\s*\[`)
	listLoc := listRe.FindStringIndex(src)
	if listLoc == nil {
		return nil, false
	}
	listStart := listLoc[1]
	listEnd := -1
	depth := 1
loop:
	for i := listStart; i < len(src); i++ {
		switch src[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				listEnd = i
				break loop
			}
		}
	}
	if listEnd == -1 {
		return nil, false
	}
	listBody := src[listStart:listEnd]
	var entries []string
	depth = 0
	entryStart := -1
	for i := 0; i < len(listBody); i++ {
		switch listBody[i] {
		case '{':
			if depth == 0 {
				entryStart = i + 1
			}
			depth++
		case '}':
			depth--
			if depth == 0 && entryStart >= 0 {
				entries = append(entries, listBody[entryStart:i])
				entryStart = -1
			}
		}
	}
	return entries, true
}

// TestDcgmExporterCSV_RolloutAnnotationPresent guards the CSV-hash
// annotation on the dcgm_exporter helm_release. Without this annotation
// the helm values are unchanged when the CSV file content changes (the
// CSV is mounted via a separate kubernetes_config_map; the helm release
// only references the configMap by name). Helm sees no diff, the
// DaemonSet is not rolled, kubelet does NOT restart pods on configMap
// data change, and the dcgm-exporter process keeps reading the OLD CSV
// from its already-mounted volume.
//
// This is exactly the upgrade-path failure the
// CLOCK_THROTTLE_REASONS → CLOCKS_EVENT_REASONS rename produced in the
// field: users on a rack with the old CSV would have stayed
// crashlooping after a rack upgrade unless they manually
// `kubectl rollout restart daemonset/dcgm-exporter`. A
// content-hash-derived annotation forces helm to detect a values diff
// and roll the DaemonSet on every CSV mutation, healing the upgrade
// automatically.
//
// What this test checks (post-Angle-B-hardening):
//
//   - The annotation lives INSIDE a `podAnnotations = { ... }` block
//     specifically. Chart-level annotations like `commonAnnotations`
//     apply to the configMap/Service/etc. but NOT to the DaemonSet
//     pods, so they would NOT trigger a rolling update on CSV change.
//     A maintainer who copy-pastes the annotation into a sibling
//     annotations key would silently break the heal mechanism.
//
//   - The annotation key is prefixed `convox.com/dcgm-csv-` (any
//     suffix accepted: -sha256, -sha1, -md5, -hash, -content, etc.).
//     Pinning a literal -sha256 suffix would false-positive on a
//     legitimate rename (e.g. swapping to filemd5 for some reason),
//     pressuring a future maintainer to "just delete the test".
//
//   - The value is a deterministic file-content hash function call
//     over the CSV's path: file(sha256|sha1|md5)("${path.module}/
//     files/dcp-metrics-included.csv"). The actual function choice
//     is irrelevant — what matters is that the file's bytes drive
//     the value, so any change to the file changes the annotation,
//     which forces helm to upgrade the chart.
//
// A maintainer who removes the annotation, moves it out of
// `podAnnotations`, or stops deriving the value from the CSV file's
// contents will fail this test in CI with a message that names the
// specific drift mode. Pre-commit catch.
func TestDcgmExporterCSV_RolloutAnnotationPresent(t *testing.T) {
	tfPath := filepath.Join("..", "..", "terraform", "cluster", "aws", "dcgm.tf")
	data, err := os.ReadFile(tfPath)
	if err != nil {
		t.Fatalf("read %s: %v", tfPath, err)
	}
	src := string(data)

	// The annotation MUST live inside `podAnnotations = { ... }`.
	// extractBracedBlock returns the content between the outer braces of
	// the named block; if a maintainer moved the annotation to
	// commonAnnotations or any other annotations surface, this lookup
	// finds a podAnnotations block that no longer contains the hash, and
	// the regex below fails — error message points at the drift class.
	podAnnBody, ok := extractBracedBlock(src, regexp.MustCompile(`podAnnotations\s*=\s*\{`))
	if !ok {
		t.Fatalf("dcgm.tf: could not locate podAnnotations = { ... } block. The CSV-hash annotation MUST live inside podAnnotations specifically; chart-level annotations (commonAnnotations, deployment.annotations, etc.) apply to ConfigMaps/Services/etc. but NOT to DaemonSet pods, and would NOT trigger a rolling update on CSV change.")
	}

	// Within that block, match an annotation key prefixed
	// `convox.com/dcgm-csv-` (suffix tolerant) bound to a
	// file<hash>("...dcp-metrics-included.csv") call. Any of
	// filesha256/filesha1/filemd5 produces a deterministic content-
	// driven value. Loose match on suffix + hash function avoids
	// brittle false positives on equivalent renames.
	annotationRe := regexp.MustCompile(
		`"convox\.com/dcgm-csv-[a-z0-9-]+"\s*=\s*` +
			`file(?:sha256|sha1|md5)\(\s*"\$\{path\.module\}/files/dcp-metrics-included\.csv"\s*\)`,
	)
	if !annotationRe.MatchString(podAnnBody) {
		t.Fatalf("dcgm.tf: podAnnotations block is missing a CSV-content-hash annotation.\n\n" +
			"Required pattern (any suffix, any deterministic hash function):\n\n" +
			"\t\"convox.com/dcgm-csv-<suffix>\" = file<hash>(\"${path.module}/files/dcp-metrics-included.csv\")\n\n" +
			"where <suffix> is any identifier (sha256/sha1/md5/hash/content/...) and <hash> is one of filesha256/filesha1/filemd5.\n\n" +
			"Without this, helm sees no values diff when the CSV configMap content changes — kubelet does NOT restart pods on configMap data update, the dcgm-exporter process keeps reading the OLD CSV from its already-mounted volume, and users stay on the broken CSV until a manual `kubectl rollout restart daemonset/dcgm-exporter`.")
	}
}

// extractBracedBlock finds the regex match for openPattern (which MUST
// end with a literal `{` — that's the open brace whose match the helper
// pairs to its close brace) and returns the content between the
// matched-pattern's `{` and its matching `}`.
//
// Naive depth counting; does NOT skip braces inside HCL strings or
// comments. dcgm.tf has no such constructs in the regions we extract;
// if a future contributor introduces them, the helper may misbehave —
// the test's caller will fail with an unrecognized-structure message
// pointing at this comment. That's a constructive failure mode.
func extractBracedBlock(src string, openPattern *regexp.Regexp) (string, bool) {
	loc := openPattern.FindStringIndex(src)
	if loc == nil {
		return "", false
	}
	start := loc[1]
	depth := 1
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start:i], true
			}
		}
	}
	return "", false
}

// TestDcgmExporterFieldsInGoSourceMatchWhitelist scans the rack's Go
// source for hardcoded DCGM_FI_* token references and asserts every
// one is present in validDCGMFields. Catches typos and stale names in:
//
//   - provider/k8s/prometheus_queries.go (PromQL query constants)
//   - provider/k8s/prometheus.go (the GpuMetrics parser table)
//   - pkg/manifest/service.go (KEDA autoscaler default Prometheus
//     metric — highest-blast-radius site, a typo there breaks every
//     user's GPU autoscaler)
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
		"prometheus_queries.go",
		"prometheus.go",
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
