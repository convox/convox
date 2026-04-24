package k8s

import (
	"time"

	"github.com/convox/convox/pkg/billing"
	v1 "k8s.io/api/core/v1"
)

// AccumulateBudgetAppForTest is a test-only hook exposing the per-app
// accumulator tick without the leader-election + polling scaffolding. A
// variadic `now` lets tests inject a deterministic clock.
func AccumulateBudgetAppForTest(p *Provider, app string, now ...time.Time) error {
	t := time.Now().UTC()
	if len(now) > 0 {
		t = now[0]
	}
	return p.accumulateBudgetApp(app, t)
}

// DominantResourceFractionForTest exposes the dominant-resource attribution
// formula for unit tests. Kept a thin wrapper — the production function
// signature must stay internal because it couples to v1.Pod/v1.Node
// pointers we don't want leaking into the package's public surface.
func DominantResourceFractionForTest(pod *v1.Pod, node *v1.Node, price billing.InstancePrice) float64 {
	return dominantResourceFraction(pod, node, price)
}

// NodeInstanceTypeForTest exposes the node-label priority helper for tests.
func NodeInstanceTypeForTest(n *v1.Node) string {
	return nodeInstanceType(n)
}

// SanitizeAckByForTest exposes the ack_by audit-string sanitizer for tests.
func SanitizeAckByForTest(s string) string {
	return sanitizeAckBy(s)
}
