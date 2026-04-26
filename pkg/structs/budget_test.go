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
	require.Contains(t, string(data), `"monthly-cap-usd":500`)
	require.Contains(t, string(data), `"at-cap-action":"alert-only"`)

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
	assert.Contains(t, string(data2), `"monthly-cap-usd":100`)
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
	payload := []byte(`{"name":"foo","release":"r1","status":"running","budget":{"monthly-cap-usd":100,"at-cap-action":"alert-only"}}`)

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

// TestAppBudget_KebabJSON_RoundTrip is the explicit kebab-form contract
// (R3 Tests R2 T-7). Asserts the JSON wire format uses "monthly-cap-usd"
// and "at-cap-action" — NOT the legacy snake form.
func TestAppBudget_KebabJSON_RoundTrip(t *testing.T) {
	b := structs.AppBudget{
		MonthlyCapUsd:         500,
		AlertThresholdPercent: 80,
		AtCapAction:           "alert-only",
		PricingAdjustment:     1.0,
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)
	require.Contains(t, string(data), `"monthly-cap-usd":500`)
	require.Contains(t, string(data), `"at-cap-action":"alert-only"`)
	// Also assert the kebab forms appear and the snake legacy forms do NOT.
	require.NotContains(t, string(data), "monthly_cap_usd")
	require.NotContains(t, string(data), "at_cap_action")
	var got structs.AppBudget
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, b, got)
}

// TestAppBudgetValidate_KebabErrorMessages — Validate() error strings
// must echo the kebab form so users see the same identifier across CLI
// flags, JSON wire, and error text.
func TestAppBudgetValidate_KebabErrorMessages(t *testing.T) {
	// Negative monthly-cap-usd path
	b1 := structs.AppBudget{MonthlyCapUsd: -1, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 1.0}
	err := b1.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "monthly-cap-usd")
	assert.NotContains(t, err.Error(), "monthly_cap_usd")

	// Out-of-range alert-threshold-percent
	b2 := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 200, AtCapAction: "alert-only", PricingAdjustment: 1.0}
	err = b2.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alert-threshold-percent")
	assert.NotContains(t, err.Error(), "alert_threshold_percent")

	// Bad at-cap-action
	b3 := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "nope", PricingAdjustment: 1.0}
	err = b3.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at-cap-action")
	assert.NotContains(t, err.Error(), "at_cap_action")

	// Out-of-range pricing-adjustment
	b4 := structs.AppBudget{MonthlyCapUsd: 100, AlertThresholdPercent: 80, AtCapAction: "alert-only", PricingAdjustment: 5}
	err = b4.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pricing-adjustment")
	assert.NotContains(t, err.Error(), "pricing_adjustment")
}

// TestAppBudget_SnakeCaseJSON_RejectsWithError is the negative regression
// guard required by R3 Tests R2 T-7. A JSON payload using the legacy
// snake-case keys must NOT bind to AppBudget fields — encoding/json
// silent-drops unknown keys, so we observe binding failure as zero values
// on the round-trip target.
//
// This test exists to fail loudly if a future encoder tweak (e.g. a custom
// UnmarshalJSON or a permissive decoder option) accidentally restored
// snake-case acceptance. The kebab form is the contract; the legacy form
// must not silently work as a fallback.
func TestAppBudget_SnakeCaseJSON_RejectsWithError(t *testing.T) {
	snake := []byte(`{"monthly_cap_usd":500,"at_cap_action":"alert-only","alert_threshold_percent":80,"pricing_adjustment":1.0}`)
	var got structs.AppBudget
	require.NoError(t, json.Unmarshal(snake, &got),
		"json.Unmarshal must not error on unknown keys; it silent-drops them")

	// All four fields must be ZERO — proves the snake-case keys did NOT bind.
	assert.Equal(t, 0.0, got.MonthlyCapUsd, "snake monthly_cap_usd must not bind")
	assert.Equal(t, "", got.AtCapAction, "snake at_cap_action must not bind")
	assert.Equal(t, 0.0, got.AlertThresholdPercent, "snake alert_threshold_percent must not bind")
	assert.Equal(t, 0.0, got.PricingAdjustment, "snake pricing_adjustment must not bind")
}


// TestServiceCostLine_KebabJSON_RoundTrip — F-9 fix (catalog F-9).
// Verifies the JSON tag rename from snake to kebab on the InstanceType
// field. Wire format must use `instance-type` (matching the rest of the
// pkg/structs/budget.go convention) — the snake form `instance_type`
// must NOT bind on Unmarshal of the new wire format.
func TestServiceCostLine_KebabJSON_RoundTrip(t *testing.T) {
	line := structs.ServiceCostLine{
		Service:      "ml-batch",
		GpuHours:     1.5,
		CpuHours:     0.25,
		MemGbHours:   2.0,
		InstanceType: "g5.xlarge",
		SpendUsd:     12.34,
		Attribution:  "test",
	}

	data, err := json.Marshal(line)
	require.NoError(t, err)
	wire := string(data)

	// Wire format MUST use kebab.
	require.Contains(t, wire, `"instance-type":"g5.xlarge"`,
		"InstanceType must marshal as kebab-case `instance-type` per convention")
	require.NotContains(t, wire, `"instance_type"`,
		"snake-form `instance_type` must NOT appear in wire output (catalog F-9)")

	// Round-trip from kebab wire format.
	var got structs.ServiceCostLine
	require.NoError(t, json.Unmarshal([]byte(wire), &got))
	assert.Equal(t, "g5.xlarge", got.InstanceType, "kebab-tagged field must bind on Unmarshal")

	// Snake-form input MUST NOT bind (defensive — catches a future
	// regression that adds a duplicate snake tag).
	snake := `{"service":"ml-batch","instance_type":"g5.xlarge"}`
	var got2 structs.ServiceCostLine
	require.NoError(t, json.Unmarshal([]byte(snake), &got2))
	assert.Equal(t, "", got2.InstanceType,
		"snake `instance_type` must NOT bind to InstanceType field — kebab tag is the only valid wire form")
}
