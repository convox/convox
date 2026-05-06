package k8s

import (
	"regexp"
	"strings"
	"testing"
)

// TestPromQLConstantsNonEmpty: every entry in AllPromQLConstants() has a
// non-empty trimmed value, AND no Go const declared inside prometheus_queries.go
// is silently absent from the map.
func TestPromQLConstantsNonEmpty(t *testing.T) {
	consts := AllPromQLConstants()
	if len(consts) == 0 {
		t.Fatal("expected non-empty PromQL constants map")
	}
	for name, q := range consts {
		if strings.TrimSpace(q) == "" {
			t.Errorf("constant %s is empty", name)
		}
	}
}

// TestPromQLConstantsHaveLabelFilters: app-scoped per-pod consts must filter by
// the convox K8s namespace OR app/service. Two regexes — one per label
// convention — keyed off the const name.
//
//   - DCGM-source per-pod consts use bare `namespace=`/`app=`/`service=`.
//   - HTTP RED consts use bare `namespace=`/`service=`.
//   - Cluster-aggregate consts have NO filter and are excluded from this test.
//   - KSM-source consts (K8sReplicaCountByService etc.) use bare KSM `namespace=`
//     labels (the `label_*` cross-join only appears in the actual Vue panel
//     query, not in these primitive consts).
func TestPromQLConstantsHaveLabelFilters(t *testing.T) {
	consts := AllPromQLConstants()
	dcgmRe := regexp.MustCompile(`(namespace|app|service)=~?"\$(namespace|app|service)"`)
	httpRedRe := regexp.MustCompile(`(namespace|service)=~"\$(namespace|service)"`)

	for _, name := range dcgmPerPodSourceConsts() {
		q, ok := consts[name]
		if !ok {
			t.Errorf("dcgm per-pod const %s missing from AllPromQLConstants()", name)
			continue
		}
		if !dcgmRe.MatchString(q) {
			t.Errorf("dcgm per-pod const %s lacks bare-label app/namespace filter: %s", name, q)
		}
		// Defensive: dcgm consts must NOT use the `label_` prefix. DCGM exporter
		// emits bare K8s labels — `label_app` is a kube-state-metrics
		// convention. Guard against regression to the pre-implementation plan draft.
		if strings.Contains(q, "label_app") || strings.Contains(q, "label_service") {
			t.Errorf("dcgm per-pod const %s uses label_* prefix (KSM convention); DCGM emits bare labels: %s", name, q)
		}
	}

	for _, name := range httpRedSourceConsts() {
		q, ok := consts[name]
		if !ok {
			t.Errorf("http red const %s missing from AllPromQLConstants()", name)
			continue
		}
		if !httpRedRe.MatchString(q) {
			t.Errorf("http red const %s lacks bare-label namespace/service filter: %s", name, q)
		}
	}
}

// TestPromQLConstantsAreSyntacticallyClean: cheap sanity check — no const
// contains stray newline, leading/trailing whitespace, or unbalanced braces.
// Catches typos before they reach Prometheus.
func TestPromQLConstantsAreSyntacticallyClean(t *testing.T) {
	for name, q := range AllPromQLConstants() {
		if q != strings.TrimSpace(q) {
			t.Errorf("const %s has leading/trailing whitespace: %q", name, q)
		}
		if strings.ContainsAny(q, "\r\n") {
			t.Errorf("const %s contains newline: %q", name, q)
		}
		if strings.Count(q, "{") != strings.Count(q, "}") {
			t.Errorf("const %s has unbalanced braces: %s", name, q)
		}
		if strings.Count(q, "(") != strings.Count(q, ")") {
			t.Errorf("const %s has unbalanced parens: %s", name, q)
		}
	}
}
