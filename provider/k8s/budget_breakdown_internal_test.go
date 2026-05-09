package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/structs"
)

func TestBuildBreakdown_NilState(t *testing.T) {
	got := buildBreakdown(nil)
	if got == nil {
		t.Fatal("must return non-nil slice (wire shape)")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestBuildBreakdown_NoPerService(t *testing.T) {
	state := &structs.AppBudgetState{}
	got := buildBreakdown(state)
	if got == nil {
		t.Fatal("must return non-nil slice (wire shape)")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestBuildBreakdown_DescendingSpendThenAlphabetical(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendUsd: map[string]float64{
			"low":   10,
			"high":  50,
			"mid":   30,
			"tied1": 25,
			"tied2": 25,
		},
		PerServiceInstanceType: map[string]string{
			"high": "m5.xlarge",
			"mid":  "m5.large",
		},
	}
	got := buildBreakdown(state)

	// Descending by spend: high (50), mid (30), tied1+tied2 (25 each), low (10).
	// Tied entries break alphabetically: tied1 before tied2.
	expectedOrder := []string{"high", "mid", "tied1", "tied2", "low"}
	if len(got) != len(expectedOrder) {
		t.Fatalf("expected %d entries, got %d: %v", len(expectedOrder), len(got), got)
	}
	for i, want := range expectedOrder {
		if got[i].Service != want {
			t.Errorf("position %d: want %q, got %q (full slice: %v)", i, want, got[i].Service, got)
		}
	}

	// Spot-check instance type pass-through.
	if got[0].InstanceType != "m5.xlarge" {
		t.Errorf("high InstanceType: want m5.xlarge, got %q", got[0].InstanceType)
	}
}

func TestBuildBreakdown_PreservesReservedBuckets(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendUsd: map[string]float64{
			perServiceBucketBuild:        15,
			perServiceBucketUnattributed: 5,
			"web":                        20,
		},
	}
	got := buildBreakdown(state)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries (web/_build/_unattributed), got %d", len(got))
	}
	// web (20) > _build (15) > _unattributed (5).
	if got[0].Service != "web" || got[1].Service != perServiceBucketBuild || got[2].Service != perServiceBucketUnattributed {
		t.Errorf("unexpected ordering: %v", got)
	}
}

func TestBuildVariantBreakdown_NilState(t *testing.T) {
	got := buildVariantBreakdown(nil)
	if got == nil {
		t.Fatal("must return non-nil slice (wire shape)")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestBuildVariantBreakdown_EmptyMap(t *testing.T) {
	got := buildVariantBreakdown(&structs.AppBudgetState{})
	if got == nil {
		t.Fatal("must return non-nil slice (wire shape)")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestBuildVariantBreakdown_SortDescendingThenAlphabetic(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendByVariant: map[string]map[string]float64{
			"web": {
				"t3.large:on-demand": 1.40,
				"t3.large:spot":      0.94,
			},
			"trainer": {
				"p3.2xlarge:on-demand": 10.00,
			},
			"worker": {
				"t3.large:unknown": 0.42,
			},
		},
	}
	got := buildVariantBreakdown(state)
	if len(got) != 4 {
		t.Fatalf("expected 4 rows (2 web + 1 trainer + 1 worker), got %d: %+v", len(got), got)
	}
	// Descending by spend, then alphabetic. Expected:
	//  trainer / p3.2xlarge / on-demand   $10.00
	//  web     / t3.large   / on-demand   $1.40
	//  web     / t3.large   / spot        $0.94
	//  worker  / t3.large   / unknown     $0.42
	expected := []struct{ svc, it, cap string }{
		{"trainer", "p3.2xlarge", "on-demand"},
		{"web", "t3.large", "on-demand"},
		{"web", "t3.large", "spot"},
		{"worker", "t3.large", "unknown"},
	}
	for i, want := range expected {
		if got[i].Service != want.svc || got[i].InstanceType != want.it || got[i].CapacityType != want.cap {
			t.Errorf("position %d: want (%s, %s, %s), got (%s, %s, %s)",
				i, want.svc, want.it, want.cap, got[i].Service, got[i].InstanceType, got[i].CapacityType)
		}
	}
}

func TestBuildVariantBreakdown_SkipsMalformedKeys(t *testing.T) {
	// Defensive: production keys are always well-formed but a corrupt
	// annotation (manual edit, partial deserialize) shouldn't panic.
	state := &structs.AppBudgetState{
		PerServiceSpendByVariant: map[string]map[string]float64{
			"web": {
				"":                      0.10, // empty key
				"no-colon-separator":    0.20, // no colon
				"t3.large:on-demand":    1.40,
			},
		},
	}
	got := buildVariantBreakdown(state)
	if len(got) != 1 {
		t.Errorf("expected 1 well-formed row, got %d: %+v", len(got), got)
	}
	if got[0].InstanceType != "t3.large" || got[0].CapacityType != "on-demand" {
		t.Errorf("unexpected row: %+v", got[0])
	}
}

// TestBuildVariantBreakdown_PopulatesReplicas asserts that pod-count
// data from PerServiceVariantPodsLastTick is joined onto the projected
// rows. Heterogeneous services must show distinct counts per variant
// so the UI can render "3 spot / 2 on-demand" rather than collapsing.
func TestBuildVariantBreakdown_PopulatesReplicas(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendByVariant: map[string]map[string]float64{
			"web": {
				"t3.large:on-demand": 1.40,
				"t3.large:spot":      0.94,
			},
		},
		PerServiceVariantPodsLastTick: map[string]map[string]int{
			"web": {
				"t3.large:on-demand": 3,
				"t3.large:spot":      2,
			},
		},
	}
	got := buildVariantBreakdown(state)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	// Sort order: descending by spend → on-demand first.
	if got[0].CapacityType != "on-demand" || got[0].Replicas != 3 {
		t.Errorf("on-demand row: want capacity=on-demand replicas=3, got %+v", got[0])
	}
	if got[1].CapacityType != "spot" || got[1].Replicas != 2 {
		t.Errorf("spot row: want capacity=spot replicas=2, got %+v", got[1])
	}
}

// TestBuildVariantBreakdown_MissingPodCountsZero asserts that a state
// with spend but no pod-count map (legacy / mid-rollout scenario) emits
// rows with Replicas=0 — the field is forward-compat zero rather than
// dropping the row entirely.
func TestBuildVariantBreakdown_MissingPodCountsZero(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendByVariant: map[string]map[string]float64{
			"web": {"t3.large:on-demand": 1.40},
		},
		// No PerServiceVariantPodsLastTick — simulating a pre-Replicas
		// rack annotation that was deserialized cleanly.
	}
	got := buildVariantBreakdown(state)
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].Replicas != 0 {
		t.Errorf("missing pod-count map: want Replicas=0, got %d", got[0].Replicas)
	}
}

// TestBuildVariantBreakdown_PartialPodCountCoverage asserts that a row
// for which we have spend but no matching pod-count entry still emits
// with Replicas=0. The other row in the same service that DOES have
// a count emits with the populated count. Defensive behaviour for a
// transient state where one variant was just observed and another has
// older spend data still on disk.
func TestBuildVariantBreakdown_PartialPodCountCoverage(t *testing.T) {
	state := &structs.AppBudgetState{
		PerServiceSpendByVariant: map[string]map[string]float64{
			"web": {
				"t3.large:on-demand": 1.40,
				"t3.large:spot":      0.94,
			},
		},
		PerServiceVariantPodsLastTick: map[string]map[string]int{
			"web": {
				"t3.large:on-demand": 3,
				// spot variant entry is missing
			},
		},
	}
	got := buildVariantBreakdown(state)
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(got))
	}
	// Sort order: on-demand $1.40 wins over spot $0.94.
	if got[0].CapacityType != "on-demand" || got[0].Replicas != 3 {
		t.Errorf("on-demand row: want replicas=3, got %+v", got[0])
	}
	if got[1].CapacityType != "spot" || got[1].Replicas != 0 {
		t.Errorf("spot row missing pod-count: want replicas=0, got %+v", got[1])
	}
}

func TestDominantInstanceTypeFromVariants_HighestSpendWins(t *testing.T) {
	// Single-instance heterogeneous capacity: same instance type wins.
	got := dominantInstanceTypeFromVariants(map[string]float64{
		"t3.large:on-demand": 0.70,
		"t3.large:spot":      0.30,
	})
	if got != "t3.large" {
		t.Errorf("homogeneous instance, mixed capacity: want t3.large, got %q", got)
	}

	// Heterogeneous instances: spend dominance wins regardless of pod count.
	got = dominantInstanceTypeFromVariants(map[string]float64{
		"t3.large:spot":          0.075, // 3 cheap pods
		"g4dn.xlarge:on-demand":  0.526, // 1 expensive pod — dominant by spend
	})
	if got != "g4dn.xlarge" {
		t.Errorf("expensive single pod must dominate: want g4dn.xlarge, got %q", got)
	}

	// Sum-across-capacity dominance: t3.large total exceeds c5.large total.
	got = dominantInstanceTypeFromVariants(map[string]float64{
		"t3.large:on-demand": 0.40,
		"t3.large:spot":      0.30,
		"c5.large:on-demand": 0.50,
	})
	if got != "t3.large" {
		t.Errorf("t3.large totals 0.70 vs c5.large 0.50: want t3.large, got %q", got)
	}
}

func TestDominantInstanceTypeFromVariants_EmptyAndNil(t *testing.T) {
	if got := dominantInstanceTypeFromVariants(nil); got != "" {
		t.Errorf("nil input: want empty, got %q", got)
	}
	if got := dominantInstanceTypeFromVariants(map[string]float64{}); got != "" {
		t.Errorf("empty input: want empty, got %q", got)
	}
}

func TestPerServiceMaxEntries_TruncatesNewEntriesPreservesExisting(t *testing.T) {
	// Lower the cap to 2 so we can drive truncation with three services
	// without constructing 1000+ fixtures.
	prev := perServiceMaxEntries
	perServiceMaxEntries = 2
	t.Cleanup(func() { perServiceMaxEntries = prev })

	// Simulate the merge step from accumulateBudgetApp: state already has
	// 2 entries (cap reached); a new tick brings a third service. The
	// existing entries must continue to accumulate; the new entry must
	// be dropped.
	state := &structs.AppBudgetState{
		PerServiceSpendUsd:     map[string]float64{"web": 10, "api": 5},
		PerServiceInstanceType: map[string]string{"web": "m5.large", "api": "m5.large"},
	}
	tickPerSvc := map[string]float64{
		"web":    1, // existing — accumulates
		"api":    2, // existing — accumulates
		"worker": 3, // new — dropped (cap reached)
	}
	tickPerSvcInst := map[string]string{
		"web":    "m5.large",
		"api":    "m5.large",
		"worker": "m5.large",
	}
	truncated := 0
	for svc, dollars := range tickPerSvc {
		if _, exists := state.PerServiceSpendUsd[svc]; !exists && len(state.PerServiceSpendUsd) >= perServiceMaxEntries {
			truncated++
			continue
		}
		state.PerServiceSpendUsd[svc] += dollars
		if _, hadIT := state.PerServiceInstanceType[svc]; !hadIT {
			if it := tickPerSvcInst[svc]; it != "" {
				state.PerServiceInstanceType[svc] = it
			}
		}
	}

	if truncated != 1 {
		t.Errorf("expected 1 truncated entry, got %d", truncated)
	}
	if state.PerServiceSpendUsd["web"] != 11 {
		t.Errorf("web should have accumulated to 11, got %v", state.PerServiceSpendUsd["web"])
	}
	if state.PerServiceSpendUsd["api"] != 7 {
		t.Errorf("api should have accumulated to 7, got %v", state.PerServiceSpendUsd["api"])
	}
	if _, present := state.PerServiceSpendUsd["worker"]; present {
		t.Errorf("worker must NOT be added when cap is reached")
	}
}
