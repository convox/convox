package structs

import (
	"fmt"
	"math"
	"time"
)

const (
	BudgetAtCapActionAlertOnly       = "alert-only"
	BudgetAtCapActionBlockNewDeploys = "block-new-deploys"

	BudgetDefaultAlertThresholdPercent = 80.0
	BudgetDefaultPricingAdjustment     = 1.0
	BudgetDefaultAtCapAction           = BudgetAtCapActionAlertOnly

	BudgetPricingAdjustmentMin = 0.1
	BudgetPricingAdjustmentMax = 1.5

	BudgetStateAnnotation  = "convox.com/budget-state"
	BudgetConfigAnnotation = "convox.com/budget-config"
)

// AppBudget is user-configured spend limits for an app.
type AppBudget struct {
	MonthlyCapUsd         float64 `json:"monthly_cap_usd"`
	AlertThresholdPercent float64 `json:"alert_threshold_percent,omitempty"`
	AtCapAction           string  `json:"at_cap_action,omitempty"`
	PricingAdjustment     float64 `json:"pricing_adjustment,omitempty"`
}

// AppBudgetState is computed state persisted as a namespace annotation.
type AppBudgetState struct {
	MonthStart            time.Time `json:"month_start"`
	CurrentMonthSpendUsd  float64   `json:"current_month_spend_usd"`
	CurrentMonthSpendAsOf time.Time `json:"current_month_spend_as_of"`
	AlertFiredAtThreshold time.Time `json:"alert_fired_at_threshold,omitempty"`
	AlertFiredAtCap       time.Time `json:"alert_fired_at_cap,omitempty"`
	CircuitBreakerTripped bool      `json:"circuit_breaker_tripped,omitempty"`
	CircuitBreakerAckBy   string    `json:"circuit_breaker_ack_by,omitempty"`
	CircuitBreakerAckAt   time.Time `json:"circuit_breaker_ack_at,omitempty"`
	WarningCount          int       `json:"warning_count,omitempty"`
}

// AppCost is the response shape for GET /apps/{app}/cost.
type AppCost struct {
	App                 string            `json:"app"`
	MonthStart          time.Time         `json:"month_start"`
	AsOf                time.Time         `json:"as_of"`
	SpendUsd            float64           `json:"spend_usd"`
	Breakdown           []ServiceCostLine `json:"breakdown"`
	PricingSource       string            `json:"pricing_source"`
	PricingTableVersion string            `json:"pricing_table_version"`
	PricingAdjustment   float64           `json:"pricing_adjustment"`
	WarningCount        int               `json:"warning_count,omitempty"`
}

type ServiceCostLine struct {
	Service      string  `json:"service"`
	GpuHours     float64 `json:"gpu_hours"`
	CpuHours     float64 `json:"cpu_hours"`
	MemGbHours   float64 `json:"mem_gb_hours"`
	InstanceType string  `json:"instance_type,omitempty"`
	SpendUsd     float64 `json:"spend_usd"`
	Attribution  string  `json:"attribution,omitempty"`
}

// AppBudgetOptions carries partial updates for AppBudgetSet.
//
// Float-valued fields use *string so they survive stdsdk form marshalling
// (which does not natively support *float64). The server parses these
// strings and rejects non-numeric or out-of-range values.
type AppBudgetOptions struct {
	MonthlyCapUsd         *string `param:"monthly_cap_usd"`
	AlertThresholdPercent *int    `param:"alert_threshold_percent"`
	AtCapAction           *string `param:"at_cap_action"`
	PricingAdjustment     *string `param:"pricing_adjustment"`
}

// ApplyDefaults fills zero-valued fields with documented defaults.
func (b *AppBudget) ApplyDefaults() {
	if b.AlertThresholdPercent == 0 {
		b.AlertThresholdPercent = BudgetDefaultAlertThresholdPercent
	}
	if b.AtCapAction == "" {
		b.AtCapAction = BudgetDefaultAtCapAction
	}
	if b.PricingAdjustment == 0 {
		b.PricingAdjustment = BudgetDefaultPricingAdjustment
	}
}

// Validate returns an error for out-of-range or invalid fields.
func (b *AppBudget) Validate() error {
	if math.IsNaN(b.MonthlyCapUsd) || math.IsInf(b.MonthlyCapUsd, 0) {
		return fmt.Errorf("monthly_cap_usd must be a finite number")
	}
	if b.MonthlyCapUsd <= 0 {
		return fmt.Errorf("monthly_cap_usd must be > 0")
	}
	if math.IsNaN(b.AlertThresholdPercent) || math.IsInf(b.AlertThresholdPercent, 0) {
		return fmt.Errorf("alert_threshold_percent must be a finite number")
	}
	if b.AlertThresholdPercent < 1 || b.AlertThresholdPercent > 100 {
		return fmt.Errorf("alert_threshold_percent must be between 1 and 100")
	}
	switch b.AtCapAction {
	case BudgetAtCapActionAlertOnly, BudgetAtCapActionBlockNewDeploys:
	default:
		return fmt.Errorf("at_cap_action must be %q or %q", BudgetAtCapActionAlertOnly, BudgetAtCapActionBlockNewDeploys)
	}
	if math.IsNaN(b.PricingAdjustment) || math.IsInf(b.PricingAdjustment, 0) {
		return fmt.Errorf("pricing_adjustment must be a finite number")
	}
	if b.PricingAdjustment < BudgetPricingAdjustmentMin || b.PricingAdjustment > BudgetPricingAdjustmentMax {
		return fmt.Errorf("pricing_adjustment must be between %.1f and %.1f", BudgetPricingAdjustmentMin, BudgetPricingAdjustmentMax)
	}
	return nil
}
