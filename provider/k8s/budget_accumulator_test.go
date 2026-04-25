package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func TestAppBudgetSetAndGet(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd:         strPtr("500"),
			AlertThresholdPercent: intPtr(80),
			AtCapAction:           options.String("alert-only"),
			PricingAdjustment:     strPtr("1.0"),
		}, "test")
		require.NoError(t, err)

		cfg, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, float64(500), cfg.MonthlyCapUsd)
		assert.Equal(t, "alert-only", cfg.AtCapAction)
		assert.Nil(t, state, "state is not written until the accumulator ticks")
	})
}

func TestAppBudgetSetValidation(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd: strPtr("-1"),
		}, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "monthly_cap_usd")
	})
}

func TestAppBudgetSetRejectsNonNumericCap(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd: strPtr("not-a-number"),
		}, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "monthly_cap_usd")
	})
}

func TestAppBudgetClear(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd: strPtr("500"),
		}, "test"))

		require.NoError(t, p.AppBudgetClear("app1", "test"))

		cfg, _, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		assert.Nil(t, cfg, "config should be cleared")
	})
}

// TestAppBudgetReset re-arms the dedupe flags and emits app:budget:reset.
func TestAppBudgetReset(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd: strPtr("500"),
			AtCapAction:   options.String("block-new-deploys"),
		}, "test"))

		// Pre-seed a tripped state annotation.
		now := time.Now().UTC()
		state := structs.AppBudgetState{
			MonthStart:            time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
			CurrentMonthSpendUsd:  501,
			CurrentMonthSpendAsOf: now,
			AlertFiredAtThreshold: now,
			AlertFiredAtCap:       now,
			CircuitBreakerTripped: true,
		}
		writeState(t, kk, "rack1-app1", &state)

		require.NoError(t, p.AppBudgetReset("app1", "nick@convox.com"))

		_, got, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.False(t, got.CircuitBreakerTripped)
		assert.True(t, got.AlertFiredAtThreshold.IsZero(), "threshold dedupe must re-arm")
		assert.True(t, got.AlertFiredAtCap.IsZero(), "cap dedupe must re-arm")
		assert.Equal(t, "nick@convox.com", got.CircuitBreakerAckBy)
		assert.False(t, got.CircuitBreakerAckAt.IsZero())
		assert.Equal(t, float64(501), got.CurrentMonthSpendUsd, "spend must not be zeroed")
	})
}

func TestBudgetEnforcementReleasePromoteBlocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			CurrentMonthSpendUsd:  501,
			CircuitBreakerTripped: true,
		})

		err := p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "budget cap exceeded")
		assert.Contains(t, err.Error(), "convox budget reset")

		var httpErr *structs.HttpError
		assert.ErrorAs(t, err, &httpErr)
		assert.Equal(t, 409, httpErr.Code())
	})
}

func TestBudgetEnforcementServiceUpdateBlocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			CircuitBreakerTripped: true,
		})

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(3)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "budget cap exceeded")

		var httpErr *structs.HttpError
		assert.ErrorAs(t, err, &httpErr)
		assert.Equal(t, 409, httpErr.Code())
	})
}

// ServiceRestart must NOT return an ErrConflict from the budget gate even
// when the breaker is tripped. The restart path calls common.AppManifest →
// AppGet → Atom.Status, which is deliberately left unmocked here: if the
// budget gate were wrong and blocked the call, we would get a 409 before
// reaching the Status call. If it correctly lets the call through, we see
// the expected Status mock-miss panic, which we recover as proof that the
// breaker was bypassed on this path.
func TestBudgetEnforcementServiceRestartNotBlocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{CircuitBreakerTripped: true})

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Panic from the unmocked Atom.Status call. That the
					// panic reached this deep is proof the budget gate did
					// not block the call — the gate short-circuits with a
					// clean ErrConflict return well before Atom.Status is
					// invoked.
					err = nil
				}
			}()
			err = p.ServiceRestart("app1", "web")
		}()

		if err != nil {
			var httpErr *structs.HttpError
			if errors.As(err, &httpErr) && httpErr.Code() == 409 {
				t.Fatalf("ServiceRestart returned budget-gate 409: %v", err)
			}
		}
	})
}

func TestBudgetAccumulatorThresholdFires(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  85, // above 80% threshold
			CurrentMonthSpendAsOf: frozen,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.False(t, state.AlertFiredAtThreshold.IsZero(), "threshold alert should have fired")
		assert.True(t, state.AlertFiredAtCap.IsZero(), "cap alert should not fire yet")
		assert.False(t, state.CircuitBreakerTripped)
	})
}

func TestBudgetAccumulatorCapFiresAndTripsBreaker(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})

		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: frozen,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.False(t, state.AlertFiredAtThreshold.IsZero())
		assert.False(t, state.AlertFiredAtCap.IsZero())
		assert.True(t, state.CircuitBreakerTripped, "block-new-deploys should trip breaker")
	})
}

func TestBudgetAccumulatorAlertOnlyDoesNotTripBreaker(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: frozen,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.False(t, state.AlertFiredAtCap.IsZero(), "cap event fires even in alert-only")
		assert.False(t, state.CircuitBreakerTripped, "alert-only must NOT trip breaker")
	})
}

// Dedup: two consecutive ticks over-cap should fire EventSend exactly once.
func TestBudgetAccumulatorDedupesCapAlert(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		first := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  105,
			CurrentMonthSpendAsOf: first,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", first))
		_, state1, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		firstCap := state1.AlertFiredAtCap

		second := first.Add(5 * time.Minute)
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", second))
		_, state2, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		assert.Equal(t, firstCap.UnixNano(), state2.AlertFiredAtCap.UnixNano(), "AlertFiredAtCap must not re-fire on a second tick")
	})
}

// After reset, a subsequent over-cap tick MUST re-trip and re-fire.
func TestBudgetAccumulatorResetThenReTrip(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})

		// Frozen clock keeps MonthStart / tick-time within the same month
		// regardless of wall-clock hour.
		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  101,
			CurrentMonthSpendAsOf: frozen,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))
		require.NoError(t, p.AppBudgetReset("app1", "nick"))

		// Spend grows; tick again.
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  120,
			CurrentMonthSpendAsOf: frozen,
		})
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.True(t, state.CircuitBreakerTripped, "reset + further over-cap must re-trip")
		assert.False(t, state.AlertFiredAtCap.IsZero(), "reset + further over-cap must fire again")
	})
}

// Scale-to-zero app: accumulator runs with zero pods and does not panic; no state change.
func TestBudgetAccumulatorScaleToZero(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1"))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, 0.0, state.CurrentMonthSpendUsd)
		assert.True(t, state.AlertFiredAtCap.IsZero())
		assert.False(t, state.CircuitBreakerTripped)
	})
}

// Month rollover: a tick in month N+1 with MonthStart=N must reset spend
// and dedupe flags.
func TestBudgetAccumulatorMonthRollover(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		// Pre-seed prior-month state with alerts fired and spend at cap.
		prev := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			CurrentMonthSpendUsd:  150,
			CurrentMonthSpendAsOf: prev,
			AlertFiredAtThreshold: prev,
			AlertFiredAtCap:       prev,
		})

		// Tick one month later.
		now := time.Date(2026, 4, 1, 0, 10, 0, 0, time.UTC)
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", now))

		_, got, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), got.MonthStart, "month rolled")
		assert.Equal(t, 0.0, got.CurrentMonthSpendUsd, "spend must reset")
		assert.True(t, got.AlertFiredAtThreshold.IsZero(), "threshold dedupe must re-arm")
		assert.True(t, got.AlertFiredAtCap.IsZero(), "cap dedupe must re-arm")
	})
}

// Cap raise while tripped must not auto-clear CircuitBreakerTripped. Only
// an explicit AppBudgetReset does that.
func TestBudgetAccumulatorCapRaiseKeepsBreakerTripped(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		// Frozen wall clock — prevents a May-1 UTC month rollover from
		// wiping breaker state mid-test.
		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  110,
			CurrentMonthSpendAsOf: frozen,
			AlertFiredAtCap:       frozen,
			CircuitBreakerTripped: true,
		})

		// Operator raises the cap from 100 to 500 (without running reset).
		require.NoError(t, p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd: strPtr("500"),
			AtCapAction:   options.String("block-new-deploys"),
		}, "test"))

		// Tick: spend 110 is below new cap 500, but the breaker must stay tripped.
		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, got, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.CircuitBreakerTripped, "cap raise must not clear breaker")
	})
}

// Budget-less app: AppCost still returns a coherent shape (no panic, empty breakdown).
func TestAppCost_NoBudget(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cost, err := p.AppCost("app1")
		require.NoError(t, err)
		require.NotNil(t, cost)
		assert.Equal(t, "app1", cost.App)
		assert.Equal(t, 0.0, cost.SpendUsd)
		assert.Equal(t, 1.0, cost.PricingAdjustment, "pricing adjustment defaults to 1.0 when unset")
		assert.Equal(t, "on-demand-static-table", cost.PricingSource)
		assert.Empty(t, cost.Breakdown)
	})
}

// AppCost with budget set reports the stored PricingAdjustment.
func TestAppCost_WithBudget(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 0.7,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:           startOfApril(),
			CurrentMonthSpendUsd: 42,
			WarningCount:         3,
		})

		cost, err := p.AppCost("app1")
		require.NoError(t, err)
		assert.Equal(t, 42.0, cost.SpendUsd)
		assert.Equal(t, 0.7, cost.PricingAdjustment)
		assert.Equal(t, 3, cost.WarningCount)
	})
}

// Corrupt MonthlyCapUsd (e.g. hand-edited annotation with cap=0) must be
// skipped by the accumulator rather than firing a perpetual cap alert.
func TestBudgetAccumulatorGuardsAgainstZeroCap(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd:         0, // hand-edited bad value
			AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:           startOfApril(),
			CurrentMonthSpendUsd: 5,
		})

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1"))

		_, got, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, got.AlertFiredAtCap.IsZero(), "zero-cap config must not fire cap alerts")
	})
}

// Reset must not require config to exist — covers the edge case where the
// operator ran `convox budget clear` while the breaker was tripped.
func TestAppBudgetReset_WithoutConfig(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  99,
			CircuitBreakerTripped: true,
			AlertFiredAtCap:       time.Now().UTC(),
		})

		require.NoError(t, p.AppBudgetReset("app1", "nick"))

		_, got, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.False(t, got.CircuitBreakerTripped)
		assert.True(t, got.AlertFiredAtCap.IsZero())
		assert.Equal(t, "nick", got.CircuitBreakerAckBy)
	})
}

// AppBudgetClear must wipe both config and state. Leaving state behind a
// tripped breaker would keep deploys blocked with no config to reset
// against.
func TestAppBudgetClear_WipesState(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  150,
			CircuitBreakerTripped: true,
		})

		require.NoError(t, p.AppBudgetClear("app1", "test"))

		cfg, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		assert.Nil(t, cfg, "config must be cleared")
		assert.Nil(t, state, "state must be cleared too")
	})
}

// ProcessRun must be gated by the breaker for non-build services. This
// closes the gap where `convox run` could spin a fresh GPU pod past the
// cap while ReleasePromote / ServiceUpdate were blocked.
func TestBudgetEnforcementProcessRunBlocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			CircuitBreakerTripped: true,
		})

		_, err := p.ProcessRun("app1", "web", structs.ProcessRunOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "budget cap exceeded")
	})
}

// Regression guard: a caller cannot bypass the breaker by passing
// service="build" on the URL path. Only opts.IsBuild (a server-side flag
// set by BuildCreate) exempts the call; the URL-derived service string
// must not.
func TestBudgetEnforcementProcessRunBuildSpoofBlocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 500, AlertThresholdPercent: 80, AtCapAction: "block-new-deploys", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{CircuitBreakerTripped: true})

		// service="build" without IsBuild=true MUST be blocked.
		_, err := p.ProcessRun("app1", "build", structs.ProcessRunOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "budget cap exceeded",
			"URL service=build must not bypass the budget breaker")

		// opts.IsBuild=true (trusted, set only by BuildCreate) MUST pass
		// the breaker so operators can ship a fix when over cap.
		var buildErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Unmocked downstream — proof the breaker gate was
					// bypassed for IsBuild=true.
					buildErr = nil
				}
			}()
			_, buildErr = p.ProcessRun("app1", "build", structs.ProcessRunOptions{IsBuild: true})
		}()
		if buildErr != nil {
			var httpErr *structs.HttpError
			if errors.As(buildErr, &httpErr) && httpErr.Code() == 409 {
				t.Fatalf("opts.IsBuild=true must not be gated by breaker; got 409: %v", buildErr)
			}
		}
	})
}

// Server-side NaN / Inf rejection for MonthlyCapUsd arriving via the wire.
// Validate() guards NaN/Inf finiteness; applyBudgetOptions guards the
// stdsdk→server path before Validate ever runs.
func TestAppBudgetSetRejectsNaNAndInf(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		for _, v := range []string{"NaN", "Inf", "-Inf", "+Inf"} {
			err := p.AppBudgetSet("app1", structs.AppBudgetOptions{MonthlyCapUsd: strPtr(v)}, "test")
			require.Error(t, err, "value=%q should be rejected", v)
			assert.Contains(t, err.Error(), "monthly_cap_usd")
		}
		for _, v := range []string{"NaN", "Inf"} {
			err := p.AppBudgetSet("app1", structs.AppBudgetOptions{
				MonthlyCapUsd:     strPtr("100"),
				PricingAdjustment: strPtr(v),
			}, "test")
			require.Error(t, err, "pricing_adjustment=%q should be rejected", v)
			assert.Contains(t, err.Error(), "pricing_adjustment")
		}
	})
}

// applyBudgetOptions error path for PricingAdjustment is otherwise 0% covered
// — the CLI rejects it before reaching the SDK, but a direct SDK caller can
// reach here.
func TestAppBudgetSetRejectsNonNumericAdjustment(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppBudgetSet("app1", structs.AppBudgetOptions{
			MonthlyCapUsd:     strPtr("100"),
			PricingAdjustment: strPtr("not-a-number"),
		}, "test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pricing_adjustment")
	})
}

// AppBudgetClear emits an app:budget:clear event so downstream webhook
// receivers see clear on the same channel as threshold / cap / reset.
func TestAppBudgetClear_EmitsEvent(t *testing.T) {
	// EventSend is fire-and-forget; no webhook is configured in tests, so
	// this test asserts the call simply does not panic and AppBudgetClear
	// returns nil. The at=alert kind=clear log line is the richer signal
	// captured in go test -v output.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:           startOfApril(),
			CurrentMonthSpendUsd: 42,
		})

		require.NoError(t, p.AppBudgetClear("app1", "test"))
	})
}

// Pod-seeded integration: the accumulator walks pods on priced nodes and
// attributes a non-zero delta. Closes the Round-1 coverage gap where the
// pod-iteration loop ran on zero-pod state only.
func TestBudgetAccumulator_ChargesRunningPod(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 1000, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		// Last tick 1 hour ago; one running pod fully allocating an m5.large
		// ($0.096/hr).
		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendUsd:  0,
			CurrentMonthSpendAsOf: frozen.Add(-1 * time.Hour),
		})

		_, err := kk.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
			ObjectMeta: am.ObjectMeta{
				Name:   "node1",
				Labels: map[string]string{"node.kubernetes.io/instance-type": "m5.large"},
			},
			Status: ac.NodeStatus{
				Allocatable: ac.ResourceList{
					ac.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
					ac.ResourceMemory: *resource.NewQuantity(8<<30, resource.BinarySI),
				},
			},
		}, am.CreateOptions{})
		require.NoError(t, err)

		_, err = kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), &ac.Pod{
			ObjectMeta: am.ObjectMeta{Name: "p1"},
			Spec: ac.PodSpec{
				NodeName: "node1",
				Containers: []ac.Container{{
					Name: "web",
					Resources: ac.ResourceRequirements{
						Requests: ac.ResourceList{
							ac.ResourceCPU:    *resource.NewMilliQuantity(2000, resource.DecimalSI),
							ac.ResourceMemory: *resource.NewQuantity(8<<30, resource.BinarySI),
						},
					},
				}},
			},
			Status: ac.PodStatus{Phase: ac.PodRunning},
		}, am.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		// Expected delta: $0.096/hr × 1.0 fraction × 1.0 hr × 1.0 adjustment = $0.096.
		assert.InDelta(t, 0.096, state.CurrentMonthSpendUsd, 0.001,
			"m5.large full allocation for 1h should charge ~$0.096")
		assert.Equal(t, 0, state.WarningCount)
	})
}

// Pod on a node without an instance-type label → warnings++, no charge.
func TestBudgetAccumulator_UnlabeledNodeIncrementsWarnings(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 1000, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendAsOf: frozen.Add(-1 * time.Hour),
		})

		_, err := kk.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
			ObjectMeta: am.ObjectMeta{Name: "node1"}, // no instance-type label
		}, am.CreateOptions{})
		require.NoError(t, err)

		_, err = kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), &ac.Pod{
			ObjectMeta: am.ObjectMeta{Name: "p1"},
			Spec:       ac.PodSpec{NodeName: "node1", Containers: []ac.Container{{Name: "web"}}},
			Status:     ac.PodStatus{Phase: ac.PodRunning},
		}, am.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, 0.0, state.CurrentMonthSpendUsd)
		assert.Equal(t, 1, state.WarningCount)
	})
}

// Non-Running pod (Pending, Succeeded) is skipped without incrementing
// warnings.
func TestBudgetAccumulator_NonRunningPodSkipped(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		writeConfig(t, kk, "rack1-app1", &structs.AppBudget{
			MonthlyCapUsd: 1000, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1,
		})

		frozen := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		writeState(t, kk, "rack1-app1", &structs.AppBudgetState{
			MonthStart:            startOfApril(),
			CurrentMonthSpendAsOf: frozen.Add(-1 * time.Hour),
		})

		_, err := kk.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
			ObjectMeta: am.ObjectMeta{
				Name:   "node1",
				Labels: map[string]string{"node.kubernetes.io/instance-type": "m5.large"},
			},
		}, am.CreateOptions{})
		require.NoError(t, err)

		_, err = kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), &ac.Pod{
			ObjectMeta: am.ObjectMeta{Name: "p1"},
			Spec:       ac.PodSpec{NodeName: "node1", Containers: []ac.Container{{Name: "web"}}},
			Status:     ac.PodStatus{Phase: ac.PodPending},
		}, am.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, k8s.AccumulateBudgetAppForTest(p, "app1", frozen))

		_, state, err := p.AppBudgetGet("app1")
		require.NoError(t, err)
		require.NotNil(t, state)
		assert.Equal(t, 0.0, state.CurrentMonthSpendUsd)
		assert.Equal(t, 0, state.WarningCount, "non-Running pods do not increment warnings")
	})
}

func startOfApril() time.Time {
	return time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
}

func writeConfig(t *testing.T, c *fake.Clientset, ns string, cfg *structs.AppBudget) {
	t.Helper()
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	patchAnnotation(t, c, ns, structs.BudgetConfigAnnotation, string(data))
}

func writeState(t *testing.T, c *fake.Clientset, ns string, state *structs.AppBudgetState) {
	t.Helper()
	data, err := json.Marshal(state)
	require.NoError(t, err)
	patchAnnotation(t, c, ns, structs.BudgetStateAnnotation, string(data))
}

func patchAnnotation(t *testing.T, c *fake.Clientset, ns, key, value string) {
	t.Helper()
	n, err := c.CoreV1().Namespaces().Get(context.TODO(), ns, am.GetOptions{})
	require.NoError(t, err)
	if n.Annotations == nil {
		n.Annotations = map[string]string{}
	}
	n.Annotations[key] = value
	_, err = c.CoreV1().Namespaces().Update(context.TODO(), n, am.UpdateOptions{})
	require.NoError(t, err)
}

// Smoke reference to silence unused import in some build layouts.
var _ = ac.Namespace{}
