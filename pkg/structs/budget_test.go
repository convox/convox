package structs_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppBudgetJSONRoundtrip(t *testing.T) {
	b := structs.AppBudget{
		MonthlyCapUsd:         500.0,
		AlertThresholdPercent: 80,
		AtCapAction:           "alert-only",
		PricingAdjustment:     1.0,
	}

	data, err := json.Marshal(b)
	require.NoError(t, err)
	require.Contains(t, string(data), `"monthly_cap_usd":500`)
	require.Contains(t, string(data), `"at_cap_action":"alert-only"`)

	var got structs.AppBudget
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, b, got)
}

func TestAppBudgetStateJSONRoundtrip(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	s := structs.AppBudgetState{
		MonthStart:            now,
		CurrentMonthSpendUsd:  250.5,
		CurrentMonthSpendAsOf: now,
		AlertFiredAtThreshold: now,
		AlertFiredAtCap:       time.Time{},
		CircuitBreakerTripped: false,
		CircuitBreakerAckBy:   "",
		CircuitBreakerAckAt:   time.Time{},
		WarningCount:          2,
	}

	data, err := json.Marshal(s)
	require.NoError(t, err)

	var got structs.AppBudgetState
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, s, got)
}

func TestAppCostJSONRoundtrip(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	c := structs.AppCost{
		App:                 "myapp",
		MonthStart:          now,
		AsOf:                now,
		SpendUsd:            123.45,
		PricingSource:       "on-demand-static-table",
		PricingTableVersion: "2026-04-22",
		PricingAdjustment:   1.0,
		WarningCount:        1,
		Breakdown: []structs.ServiceCostLine{
			{
				Service:      "web",
				GpuHours:     0,
				CpuHours:     12.5,
				MemGbHours:   48,
				InstanceType: "m5.large",
				SpendUsd:     1.2,
				Attribution:  "dominant-resource",
			},
		},
	}

	data, err := json.Marshal(c)
	require.NoError(t, err)

	var got structs.AppCost
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, c, got)
}

func TestAppBudgetValidate_AtCapAction(t *testing.T) {
	cases := []struct {
		action string
		ok     bool
	}{
		{"alert-only", true},
		{"block-new-deploys", true},
		{"", false},
		{"auto-shutdown-aggressive", false},
		{"foo", false},
	}
	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			b := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: c.action, PricingAdjustment: 1.0}
			err := b.Validate()
			if c.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAppBudgetValidate_AlertThreshold(t *testing.T) {
	cases := []struct {
		pct float64
		ok  bool
	}{
		{1, true},
		{50, true},
		{100, true},
		{0, false},
		{-1, false},
		{101, false},
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			b := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: c.pct, AtCapAction: "alert-only", PricingAdjustment: 1.0}
			err := b.Validate()
			if c.ok {
				assert.NoError(t, err, "pct=%v should be valid", c.pct)
			} else {
				assert.Error(t, err, "pct=%v should be invalid", c.pct)
			}
		})
	}
}

func TestAppBudgetValidate_PricingAdjustment(t *testing.T) {
	cases := []struct {
		pa float64
		ok bool
	}{
		{0.1, true},
		{1.0, true},
		{1.5, true},
		{0.09, false},
		{1.51, false},
		{0, false},
		{-1, false},
		{70, false},
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			b := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: c.pa}
			err := b.Validate()
			if c.ok {
				assert.NoError(t, err, "pa=%v should be valid", c.pa)
			} else {
				assert.Error(t, err, "pa=%v should be invalid", c.pa)
			}
		})
	}
}

func TestAppBudgetValidate_MonthlyCap(t *testing.T) {
	for _, cap := range []float64{0, -1, -100} {
		b := structs.AppBudget{MonthlyCapUsd: cap, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1.0}
		assert.Error(t, b.Validate(), "cap=%v should be invalid", cap)
	}
}

// TestAppBudgetValidate_RejectsNaNAndInf — NaN and ±Inf bypass the < 0,
// < 0.1, and > 1.5 comparisons (all return false) so they must be rejected
// explicitly. json.Marshal also refuses these but we prefer a clean
// Validate error over a cryptic downstream encoder failure.
func TestAppBudgetValidate_RejectsNaNAndInf(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)
	ninf := math.Inf(-1)

	for _, cap := range []float64{nan, inf, ninf} {
		b := structs.AppBudget{MonthlyCapUsd: cap, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1.0}
		assert.Error(t, b.Validate(), "MonthlyCapUsd=%v should be rejected", cap)
	}
	for _, pct := range []float64{nan, inf, ninf} {
		b := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: pct, AtCapAction: "alert-only", PricingAdjustment: 1.0}
		assert.Error(t, b.Validate(), "AlertThresholdPercent=%v should be rejected", pct)
	}
	for _, pa := range []float64{nan, inf, ninf} {
		b := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: pa}
		assert.Error(t, b.Validate(), "PricingAdjustment=%v should be rejected", pa)
	}
}

func TestAppBudgetApplyDefaults(t *testing.T) {
	b := structs.AppBudget{MonthlyCapUsd: 500}
	b.ApplyDefaults()

	assert.Equal(t, float64(500), b.MonthlyCapUsd)
	assert.Equal(t, float64(80), b.AlertThresholdPercent)
	assert.Equal(t, "alert-only", b.AtCapAction)
	assert.Equal(t, 1.0, b.PricingAdjustment)
}

func TestAppBudgetApplyDefaults_PreservesSet(t *testing.T) {
	b := structs.AppBudget{
		MonthlyCapUsd:         500,
		AlertThresholdPercent: 90,
		AtCapAction:           "block-new-deploys",
		PricingAdjustment:     0.7,
	}
	b.ApplyDefaults()

	assert.Equal(t, float64(90), b.AlertThresholdPercent)
	assert.Equal(t, "block-new-deploys", b.AtCapAction)
	assert.Equal(t, 0.7, b.PricingAdjustment)
}

func TestAppBudgetPointerOnApp(t *testing.T) {
	// Backward-compat: App with nil Budget must serialise without "budget" field.
	a := structs.App{Name: "myapp", Release: "r1"}
	data, err := json.Marshal(a)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "budget")

	// App with Budget set must include it.
	a.Budget = &structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1.0}
	data2, err := json.Marshal(a)
	require.NoError(t, err)
	assert.Contains(t, string(data2), `"budget"`)
	assert.Contains(t, string(data2), `"monthly_cap_usd":100`)
}

// TestAppWithBudget_OldSDKUnmarshal proves the vendored-console3 compat
// claim: a pre-budget App struct (no Budget field) can unmarshal a JSON
// payload that contains "budget":{...} without error, silently dropping
// the unknown field. This is the reverse of TestAppBudgetPointerOnApp.
func TestAppWithBudget_OldSDKUnmarshal(t *testing.T) {
	type oldApp struct {
		Name    string `json:"name"`
		Release string `json:"release"`
		Status  string `json:"status"`
	}
	payload := []byte(`{"name":"foo","release":"r1","status":"running","budget":{"monthly_cap_usd":100,"at_cap_action":"alert-only"}}`)

	var old oldApp
	require.NoError(t, json.Unmarshal(payload, &old))
	assert.Equal(t, "foo", old.Name)
	assert.Equal(t, "r1", old.Release)
	assert.Equal(t, "running", old.Status)

	// Roundtrip back: old struct serialises without the budget field.
	out, err := json.Marshal(old)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "budget")
}

func TestServiceCostLineJSONRoundtrip(t *testing.T) {
	line := structs.ServiceCostLine{
		Service:      "inference",
		GpuHours:     2.0,
		CpuHours:     0.1,
		MemGbHours:   8.5,
		InstanceType: "g5.xlarge",
		SpendUsd:     2.012,
		Attribution:  "gpu-allocated",
	}

	data, err := json.Marshal(line)
	require.NoError(t, err)

	var got structs.ServiceCostLine
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, line, got)
}
