package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/billing"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func metaObject(labels map[string]string) am.ObjectMeta {
	return am.ObjectMeta{Labels: labels}
}

func makePod(cpuMilli, memMi int64, gpuKey string, gpuCount int64) *v1.Pod {
	reqs := v1.ResourceList{
		v1.ResourceCPU:    *resource.NewMilliQuantity(cpuMilli, resource.DecimalSI),
		v1.ResourceMemory: *resource.NewQuantity(memMi<<20, resource.BinarySI),
	}
	if gpuKey != "" && gpuCount > 0 {
		reqs[v1.ResourceName(gpuKey)] = *resource.NewQuantity(gpuCount, resource.DecimalSI)
	}
	return &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: reqs}}},
		},
	}
}

func makeNode(cpuMilli, memMi int64, instanceType string) *v1.Node {
	labels := map[string]string{}
	if instanceType != "" {
		labels["node.kubernetes.io/instance-type"] = instanceType
	}
	return &v1.Node{
		ObjectMeta: metaObject(labels),
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(cpuMilli, resource.DecimalSI),
				v1.ResourceMemory: *resource.NewQuantity(memMi<<20, resource.BinarySI),
			},
		},
	}
}

func TestDominantResourceFraction_TableDriven(t *testing.T) {
	// m5.xlarge = 4 vCPU, 16 GiB — 4000m / 16384Mi
	m5xl := makeNode(4000, 16384, "m5.xlarge")
	m5xlPrice := billing.InstancePrice{OnDemandUsdPerHour: 0.192, VcpuCount: 4, MemGb: 16}

	// g5.2xlarge = 8 vCPU, 32 GiB, 1 A10G
	g52xl := makeNode(8000, 32768, "g5.2xlarge")
	g52xlPrice := billing.InstancePrice{OnDemandUsdPerHour: 1.212, GpuCount: 1, GpuType: "A10G", VcpuCount: 8, MemGb: 32}

	cases := []struct {
		name     string
		pod      *v1.Pod
		node     *v1.Node
		price    billing.InstancePrice
		expected float64
	}{
		{"m5.xl 4c/8Gi dominant=cpu full", makePod(4000, 8192, "", 0), m5xl, m5xlPrice, 1.0},
		{"m5.xl 2c/4Gi dominant=cpu half", makePod(2000, 4096, "", 0), m5xl, m5xlPrice, 0.5},
		{"m5.xl 500m/4Gi dominant=mem qtr", makePod(500, 4096, "", 0), m5xl, m5xlPrice, 0.25},
		{"m5.xl 2c/16Gi dominant=mem full", makePod(2000, 16384, "", 0), m5xl, m5xlPrice, 1.0},
		{"g5.2xl 500m/4Gi no-gpu", makePod(500, 4096, "", 0), g52xl, g52xlPrice, 0.125},
		{"g5.2xl 1gpu claims full instance", makePod(500, 4096, "nvidia.com/gpu", 1), g52xl, g52xlPrice, 1.0},
		{"g5.2xl reqGpu>GpuCount clamps to 1.0", makePod(500, 4096, "nvidia.com/gpu", 2), g52xl, g52xlPrice, 1.0},
		{"m5.xl 0c/0Gi empty pod", makePod(0, 0, "", 0), m5xl, m5xlPrice, 0.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := k8s.DominantResourceFractionForTest(c.pod, c.node, c.price)
			assert.InDelta(t, c.expected, got, 0.0001, "expected %v got %v", c.expected, got)
		})
	}
}

func TestDominantResourceFraction_InitContainerContributes(t *testing.T) {
	// Init containers hold the same request ceiling as regular containers.
	// A heavy init container with 4000m CPU should dominate even when the
	// regular container asks for very little.
	node := makeNode(4000, 16384, "m5.xlarge")
	price := billing.InstancePrice{OnDemandUsdPerHour: 0.192, VcpuCount: 4, MemGb: 16}

	pod := &v1.Pod{
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU: *resource.NewMilliQuantity(4000, resource.DecimalSI),
					},
				},
			}},
			Containers: []v1.Container{{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU: *resource.NewMilliQuantity(100, resource.DecimalSI),
					},
				},
			}},
		},
	}

	got := k8s.DominantResourceFractionForTest(pod, node, price)
	assert.InDelta(t, 1.0, got, 0.0001)
}

func TestDominantResourceFraction_NonCanonicalGpuKeyIgnored(t *testing.T) {
	// An extended-resource name that happens to end in "gpu" but is not a
	// canonical vendor key must NOT be summed into the GPU fraction.
	g52xl := makeNode(8000, 32768, "g5.2xlarge")
	price := billing.InstancePrice{OnDemandUsdPerHour: 1.212, GpuCount: 1, VcpuCount: 8, MemGb: 32}

	pod := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Resources: v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:                          *resource.NewMilliQuantity(500, resource.DecimalSI),
						v1.ResourceMemory:                       *resource.NewQuantity(4096<<20, resource.BinarySI),
						v1.ResourceName("example.com/test-gpu"): *resource.NewQuantity(4, resource.DecimalSI),
					},
				},
			}},
		},
	}

	got := k8s.DominantResourceFractionForTest(pod, g52xl, price)
	// Should fall back to CPU/mem (neither claims full instance) → 500/8000 vs 4096/32768 = 0.125 mem
	assert.InDelta(t, 0.125, got, 0.0001)
}

func TestNodeInstanceType_PriorityOrder(t *testing.T) {
	// Modern label wins over legacy beta label.
	n := &v1.Node{ObjectMeta: metaObject(map[string]string{
		"node.kubernetes.io/instance-type": "m5.large",
		"beta.kubernetes.io/instance-type": "old.type",
	})}
	assert.Equal(t, "m5.large", k8s.NodeInstanceTypeForTest(n))

	// Only beta label present → used.
	n2 := &v1.Node{ObjectMeta: metaObject(map[string]string{
		"beta.kubernetes.io/instance-type": "old.type",
	})}
	assert.Equal(t, "old.type", k8s.NodeInstanceTypeForTest(n2))

	// No labels → empty string.
	n3 := &v1.Node{}
	assert.Equal(t, "", k8s.NodeInstanceTypeForTest(n3))
}

// nodeWithLabelsAndAnnotations builds a *v1.Node fixture for capacity-type
// tests so the helper can be exercised across the dual-signal precedence
// surface (Karpenter label, ANG label, both, neither).
func nodeWithLabelsAndAnnotations(labels, annotations map[string]string) *v1.Node {
	return &v1.Node{
		ObjectMeta: am.ObjectMeta{
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

// TestNodeCapacityType_DualSignal exercises the dual-signal capacity reader.
// Both AWS sources are LABELS, not annotations: Karpenter writes
// `karpenter.sh/capacity-type` lowercase ("spot"/"on-demand"); EKS managed
// node groups write `eks.amazonaws.com/capacityType` uppercase
// ("SPOT"/"ON_DEMAND"). The helper normalises both into the same lowercase
// dash form ("spot"/"on-demand") so downstream pricing math sees one shape.
func TestNodeCapacityType_DualSignal(t *testing.T) {
	cases := []struct {
		name     string
		node     *v1.Node
		expected string
	}{
		{
			name:     "Karpenter_spot_label",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"karpenter.sh/capacity-type": "spot"}, nil),
			expected: "spot",
		},
		{
			name:     "Karpenter_on_demand_label",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"karpenter.sh/capacity-type": "on-demand"}, nil),
			expected: "on-demand",
		},
		{
			name:     "Karpenter_uppercase_label_normalized_via_ToLower",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"karpenter.sh/capacity-type": "SPOT"}, nil),
			expected: "spot",
		},
		{
			// Live-EKS observation (test rack). EKS managed node groups
			// publish capacity type as a node LABEL, value uppercase
			// "SPOT" or "ON_DEMAND" — never an annotation. Reading the
			// label here returns "spot".
			name:     "EKS_label_SPOT_normalized_to_spot",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"eks.amazonaws.com/capacityType": "SPOT"}, nil),
			expected: "spot",
		},
		{
			name:     "EKS_label_ON_DEMAND_normalized_to_on_demand",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"eks.amazonaws.com/capacityType": "ON_DEMAND"}, nil),
			expected: "on-demand",
		},
		{
			// Annotation-only nodes (the pre-fix code path) must NOT
			// detect capacity. A real EKS-ANG node never has the value
			// in annotations — only labels — so an annotation-only
			// fixture exercises the case where the label was missing
			// AND somebody set the annotation by mistake. Result: empty,
			// because annotations are not consulted.
			name:     "EKS_annotation_only_returns_empty_no_label",
			node:     nodeWithLabelsAndAnnotations(nil, map[string]string{"eks.amazonaws.com/capacityType": "SPOT"}),
			expected: "",
		},
		{
			name: "Karpenter_label_takes_priority_over_EKS_label",
			node: nodeWithLabelsAndAnnotations(
				map[string]string{
					"karpenter.sh/capacity-type":      "spot",
					"eks.amazonaws.com/capacityType":  "ON_DEMAND",
				}, nil),
			expected: "spot",
		},
		{
			name: "Unknown_karpenter_value_falls_through_to_EKS_label",
			node: nodeWithLabelsAndAnnotations(
				map[string]string{
					"karpenter.sh/capacity-type":     "weird-value",
					"eks.amazonaws.com/capacityType": "SPOT",
				}, nil),
			expected: "spot",
		},
		{
			name:     "No_labels_no_annotations_returns_empty",
			node:     &v1.Node{},
			expected: "",
		},
		{
			name:     "Empty_karpenter_value_returns_empty",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"karpenter.sh/capacity-type": ""}, nil),
			expected: "",
		},
		{
			name:     "Empty_EKS_value_returns_empty",
			node:     nodeWithLabelsAndAnnotations(map[string]string{"eks.amazonaws.com/capacityType": ""}, nil),
			expected: "",
		},
		{
			name:     "Nil_node_returns_empty",
			node:     nil,
			expected: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := k8s.NodeCapacityTypeForTest(c.node)
			assert.Equal(t, c.expected, got)
		})
	}
}

func TestSanitizeAckBy(t *testing.T) {
	// Printable passes through.
	assert.Equal(t, "nick@convox.com", k8s.SanitizeAckByForTest("nick@convox.com"))

	// Control characters stripped.
	assert.Equal(t, "nickspy", k8s.SanitizeAckByForTest("nick\n\t\x00spy"))

	// Long input truncated to 256.
	long := ""
	for i := 0; i < 1000; i++ {
		long += "a"
	}
	assert.Equal(t, 256, len(k8s.SanitizeAckByForTest(long)))

	// Empty input gets a deterministic fallback.
	assert.Equal(t, "unknown", k8s.SanitizeAckByForTest(""))

	// Control-only input also falls through to "unknown".
	assert.Equal(t, "unknown", k8s.SanitizeAckByForTest("\n\t\r"))
}

// TestSanitizeAckBy_DefenseInDepthStrips locks in the round-2 hardening
// of sanitizeAckBy. Each case pins a specific Unicode-class strip rule.
// A regression of any individual rule (e.g. a future refactor that
// inlines and accidentally drops the C1 range check) would break the
// matching case here without changing the integrated audit-event tests.
// Non-ASCII inputs use Go's \u escape syntax so source-embedded
// invisible characters cannot confuse the parser or hide intent.
func TestSanitizeAckBy_DefenseInDepthStrips(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// C0 + DEL.
		{"C0_NUL", "alice\x00bob", "alicebob"},
		{"DEL_0x7F", "alice\x7fbob", "alicebob"},

		// C1 controls (0x80-0x9f) — legacy terminal escape sequences.
		{"C1_low_0x80", "alice\u0080bob", "alicebob"},
		{"C1_CSI_0x9b", "alice\u009bbob", "alicebob"},
		{"C1_high_0x9f", "alice\u009fbob", "alicebob"},

		// BiDi overrides — display-spoofing (rendered text reverses).
		{"BiDi_LRE_U202A", "alice\u202abob", "alicebob"},
		{"BiDi_RLE_U202B", "alice\u202bbob", "alicebob"},
		{"BiDi_PDF_U202C", "alice\u202cbob", "alicebob"},
		{"BiDi_LRO_U202D", "alice\u202dbob", "alicebob"},
		{"BiDi_RLO_U202E", "alice\u202ebob", "alicebob"},
		{"BiDi_LRI_U2066", "alice\u2066bob", "alicebob"},
		{"BiDi_RLI_U2067", "alice\u2067bob", "alicebob"},
		{"BiDi_FSI_U2068", "alice\u2068bob", "alicebob"},
		{"BiDi_PDI_U2069", "alice\u2069bob", "alicebob"},

		// Line/paragraph separators — legacy JSON parser break-out and
		// renderer-line-break injection.
		{"LSEP_U2028", "alice\u2028bob", "alicebob"},
		{"PSEP_U2029", "alice\u2029bob", "alicebob"},

		// Zero-width characters — invisible-character spoofing of audit-log values.
		{"ZWSP_U200B", "alice\u200bbob", "alicebob"},
		{"ZWNJ_U200C", "alice\u200cbob", "alicebob"},
		{"ZWJ_U200D", "alice\u200dbob", "alicebob"},
		{"LRM_U200E", "alice\u200ebob", "alicebob"},
		{"RLM_U200F", "alice\u200fbob", "alicebob"},

		// Byte order mark — invisible-character spoofing.
		{"BOM_UFEFF", "alice\ufeffbob", "alicebob"},

		// Truthful values pass through unmodified.
		{"truthful_email", "alice@example.com", "alice@example.com"},

		// Whitespace-only collapses to "unknown" — round-2 added
		// strings.TrimSpace gating to catch pathological "   " inputs
		// that pre-round-2 would have stamped a misleading whitespace
		// actor on the event.
		{"whitespace_spaces", "   ", "unknown"},
		{"whitespace_tabs", "\t\t", "unknown"},
		{"whitespace_mixed", " \t \n ", "unknown"},
		{"whitespace_unicode_NBSP", "\u00a0\u00a0", "unknown"},
		{"whitespace_unicode_NNBSP", "\u202f", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, k8s.SanitizeAckByForTest(tc.in),
				"sanitizeAckBy(%q) must equal %q", tc.in, tc.want)
		})
	}
}
