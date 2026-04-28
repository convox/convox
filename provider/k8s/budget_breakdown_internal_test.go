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
