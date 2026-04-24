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
