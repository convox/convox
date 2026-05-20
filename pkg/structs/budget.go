package structs

import (
	"fmt"
	"math"
	"time"
)

const (
	BudgetAtCapActionAlertOnly       = "alert-only"
	BudgetAtCapActionBlockNewDeploys = "block-new-deploys"
	BudgetAtCapActionAutoShutdown    = "auto-shutdown"

	BudgetDefaultAlertThresholdPercent = 80.0
	BudgetDefaultPricingAdjustment     = 1.0
	BudgetDefaultAtCapAction           = BudgetAtCapActionAlertOnly

	BudgetPricingAdjustmentMin = 0.1
	BudgetPricingAdjustmentMax = 1.5

	BudgetStateAnnotation  = "convox.com/budget-state"
	BudgetConfigAnnotation = "convox.com/budget-config"

	BudgetShutdownStateAnnotation           = "convox.com/budget-shutdown-state"
	BudgetFlapSuppressedUntilAnnotation     = "convox.com/budget-flap-suppressed-until"
	BudgetRecoveryBannerDismissedAnnotation = "convox.com/budget-recovery-banner-dismissed"
	BudgetFlapSuppressFiredAtAnnotation     = "convox.com/budget-flap-suppress-fired-at"

	// Separate from state annotation so dedup survives corrupt state JSON.
	BudgetShutdownStateCorruptFiredAtAnnotation = "convox.com/budget-shutdown-state-corrupt-fired-at"

	// Dedup for :noop when no state annotation exists to persist the tracker.
	BudgetShutdownNoopFiredAtAnnotation = "convox.com/budget-shutdown-noop-fired-at"

	KedaPausedReplicasAnnotation = "autoscaling.keda.sh/paused-replicas"

	BudgetShutdownStateSchemaVersion = 1

	BudgetDefaultNotifyBeforeMinutes = 30
	BudgetDefaultShutdownGracePeriod = "5m"
	BudgetDefaultShutdownOrder       = "largest-cost"
	BudgetDefaultRecoveryMode        = "auto-on-reset"

	BudgetFlapCooldown = 24 * time.Hour

	// Extend, do not rename -- webhook receivers treat these as stable strings.
	BudgetShutdownReasonK8sApiFailure       = "k8s-api-failure"
	BudgetShutdownReasonStateCorrupt        = "state-corrupt"
	BudgetShutdownReasonAdmissionRejected   = "admission-rejected"
	BudgetShutdownReasonAnnotationRejected  = "annotation-rejected"
	BudgetShutdownReasonCooldownWriteFailed = "cooldown-write-failed"
	BudgetShutdownReasonSchemaIncompatible  = "schema-incompatible"
)

type AppBudget struct {
	MonthlyCapUsd         float64 `json:"monthly-cap-usd"`
	AlertThresholdPercent float64 `json:"alert-threshold-percent,omitempty"`
	AtCapAction           string  `json:"at-cap-action,omitempty"`
	PricingAdjustment     float64 `json:"pricing-adjustment,omitempty"`
	LastCapMutationBy     string  `json:"last-cap-mutation-by,omitempty"`
}

type AppBudgetState struct {
	MonthStart            time.Time `json:"month-start"`
	CurrentMonthSpendUsd  float64   `json:"current-month-spend-usd"`
	CurrentMonthSpendAsOf time.Time `json:"current-month-spend-as-of"`
	AlertFiredAtThreshold time.Time `json:"alert-fired-at-threshold,omitempty"`
	AlertFiredAtCap       time.Time `json:"alert-fired-at-cap,omitempty"`
	CircuitBreakerTripped bool      `json:"circuit-breaker-tripped,omitempty"`
	CircuitBreakerAckBy   string    `json:"circuit-breaker-ack-by,omitempty"`
	CircuitBreakerAckAt   time.Time `json:"circuit-breaker-ack-at,omitempty"`
	WarningCount          int       `json:"warning-count,omitempty"`

	// Keyed by service label; "_build" and "_unattributed" are reserved buckets.
	PerServiceSpendUsd map[string]float64 `json:"per-service-spend-usd,omitempty"`

	PerServiceInstanceType map[string]string `json:"per-service-instance-type,omitempty"`

	// Inner key: "<instanceType>:<capacityType>".
	PerServiceSpendByVariant map[string]map[string]float64 `json:"per-service-spend-by-variant,omitempty"`

	// Snapshot (not cumulative) -- pod count per variant from the last tick.
	PerServiceVariantPodsLastTick map[string]map[string]int `json:"per-service-variant-pods-last-tick,omitempty"`
}

type AppCost struct {
	App                 string                   `json:"app"`
	MonthStart          time.Time                `json:"month-start"`
	AsOf                time.Time                `json:"as-of"`
	SpendUsd            float64                  `json:"spend-usd"`
	Breakdown           []ServiceCostLine        `json:"breakdown"`
	VariantBreakdown    []ServiceVariantCostLine `json:"variant-breakdown,omitempty"`
	PricingSource       string                   `json:"pricing-source"`
	PricingTableVersion string                   `json:"pricing-table-version"`
	PricingAdjustment   float64                  `json:"pricing-adjustment"`
	WarningCount        int                      `json:"warning-count,omitempty"`
	TrackingEnabled     bool                     `json:"tracking-enabled,omitempty"`
}

type ServiceCostLine struct {
	Service      string  `json:"service"`
	GpuHours     float64 `json:"gpu-hours"`
	CpuHours     float64 `json:"cpu-hours"`
	MemGbHours   float64 `json:"mem-gb-hours"`
	InstanceType string  `json:"instance-type,omitempty"`
	SpendUsd     float64 `json:"spend-usd"`
	Attribution  string  `json:"attribution,omitempty"`
}

type ServiceVariantCostLine struct {
	Service      string  `json:"service"`
	InstanceType string  `json:"instance-type"`
	CapacityType string  `json:"capacity-type"`
	SpendUsd     float64 `json:"spend-usd"`
	Replicas     int     `json:"replicas,omitempty"`
}

const CapacityTypeUnknown = "unknown"

type AppBudgetOptions struct {
	MonthlyCapUsd         *string `param:"monthly-cap-usd"`
	AlertThresholdPercent *int    `param:"alert-threshold-percent"`
	AtCapAction           *string `param:"at-cap-action"`
	PricingAdjustment     *string `param:"pricing-adjustment"`
}

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

func (b *AppBudget) Validate() error {
	if math.IsNaN(b.MonthlyCapUsd) || math.IsInf(b.MonthlyCapUsd, 0) {
		return fmt.Errorf("monthly-cap-usd must be a finite number")
	}
	if b.MonthlyCapUsd <= 0 {
		return fmt.Errorf("monthly-cap-usd must be > 0")
	}
	if math.IsNaN(b.AlertThresholdPercent) || math.IsInf(b.AlertThresholdPercent, 0) {
		return fmt.Errorf("alert-threshold-percent must be a finite number")
	}
	if b.AlertThresholdPercent < 1 || b.AlertThresholdPercent > 100 {
		return fmt.Errorf("alert-threshold-percent must be between 1 and 100")
	}
	switch b.AtCapAction {
	case BudgetAtCapActionAlertOnly, BudgetAtCapActionBlockNewDeploys, BudgetAtCapActionAutoShutdown:
	default:
		return fmt.Errorf("at-cap-action must be %q, %q, or %q",
			BudgetAtCapActionAlertOnly, BudgetAtCapActionBlockNewDeploys, BudgetAtCapActionAutoShutdown)
	}
	if math.IsNaN(b.PricingAdjustment) || math.IsInf(b.PricingAdjustment, 0) {
		return fmt.Errorf("pricing-adjustment must be a finite number")
	}
	if b.PricingAdjustment < BudgetPricingAdjustmentMin || b.PricingAdjustment > BudgetPricingAdjustmentMax {
		return fmt.Errorf("pricing-adjustment must be between %.1f and %.1f", BudgetPricingAdjustmentMin, BudgetPricingAdjustmentMax)
	}
	return nil
}

type AppBudgetSimulationResult struct {
	App                          string                           `json:"app"`
	AtCapAction                  string                           `json:"at-cap-action"`
	WebhookUrl                   string                           `json:"webhook-url"`
	NotifyBeforeMinutes          int                              `json:"notify-before-minutes"`
	ShutdownGracePeriod          string                           `json:"shutdown-grace-period"`
	ShutdownOrder                string                           `json:"shutdown-order"`
	RecoveryMode                 string                           `json:"recovery-mode"`
	Eligibility                  []AppBudgetSimulationEligibility `json:"eligibility"`
	WouldShutDownServices        []string                         `json:"would-shut-down-services"`
	WouldShutDownCount           int                              `json:"would-shut-down-count"`
	EstimatedCostSavedUsdPerHour float64                          `json:"estimated-cost-saved-usd-per-hour"`
	SimulatedAt                  time.Time                        `json:"simulated-at"`
}

type AppBudgetSimulationEligibility struct {
	Service        string  `json:"service"`
	Eligible       bool    `json:"eligible"`
	Reason         string  `json:"reason,omitempty"`
	Replicas       int     `json:"replicas"`
	CostUsdPerHour float64 `json:"cost-usd-per-hour"`
}

// camelCase JSON tags: annotation-internal, not wire surface.
type AppBudgetShutdownState struct {
	SchemaVersion int `json:"schemaVersion"`

	ShutdownAt *time.Time `json:"shutdownAt"`
	ArmedAt    *time.Time `json:"armedAt"`
	RestoredAt *time.Time `json:"restoredAt"`
	ExpiredAt  *time.Time `json:"expiredAt"`

	NotifyBeforeMinutes int `json:"notifyBeforeMinutes,omitempty"`

	RecoveryMode   string `json:"recoveryMode"`
	ShutdownOrder  string `json:"shutdownOrder"`
	ShutdownTickId string `json:"shutdownTickId"`
	ManifestSha256 string `json:"manifestSha256"`

	EligibleServiceCount int                             `json:"eligibleServiceCount"`
	Services             []AppBudgetShutdownStateService `json:"services"`

	FlapSuppressedUntil *time.Time `json:"flapSuppressedUntil"`

	// GET-only; must be nil'd before persisting the annotation.
	RecoveryBannerDismissedAt *time.Time `json:"recoveryBannerDismissedAt,omitempty"`

	ArmedNotificationFiredAt          *time.Time `json:"armedNotificationFiredAt"`
	FiredNotificationFiredAt          *time.Time `json:"firedNotificationFiredAt"`
	NoopNotificationFiredAt           *time.Time `json:"noopNotificationFiredAt"`
	CancelledNotificationFiredAt      *time.Time `json:"cancelledNotificationFiredAt"`
	ExpiredNotificationFiredAt        *time.Time `json:"expiredNotificationFiredAt"`
	FlapSuppressedNotificationFiredAt *time.Time `json:"flapSuppressedNotificationFiredAt"`
	FailedNotificationFiredAt         *time.Time `json:"failedNotificationFiredAt"`
	RestoredNotificationFiredAt       *time.Time `json:"restoredNotificationFiredAt"`

	DiscoveryReason string `json:"discoveryReason,omitempty"`

	FailureReason string `json:"failureReason,omitempty"`
}

type AppBudgetShutdownStateService struct {
	Name                       string                              `json:"name"`
	OriginalScale              AppBudgetShutdownStateOriginalScale `json:"originalScale"`
	OriginalGracePeriodSeconds int64                               `json:"originalGracePeriodSeconds"`
	KedaScaledObject           *AppBudgetShutdownStateKeda         `json:"kedaScaledObject"`
	ShutdownSequenceIndex      int                                 `json:"shutdownSequenceIndex"`
	ShutdownAt                 *time.Time                          `json:"shutdownAt"`
}

type AppBudgetShutdownStateOriginalScale struct {
	Count    int `json:"count"`
	Min      int `json:"min"`
	Max      int `json:"max"`
	Replicas int `json:"replicas"`
}

type AppBudgetShutdownStateKeda struct {
	Name                        string `json:"name"`
	PausedReplicasAnnotationSet bool   `json:"pausedReplicasAnnotationSet"`
}

func (s *AppBudgetShutdownState) ValidateRequiredFields() error {
	if s.SchemaVersion == 0 {
		return fmt.Errorf("schemaVersion is required and must be > 0")
	}
	if s.SchemaVersion > BudgetShutdownStateSchemaVersion {
		return fmt.Errorf("schemaVersion %d not supported (max supported: %d)",
			s.SchemaVersion, BudgetShutdownStateSchemaVersion)
	}
	if s.ArmedAt == nil || s.ArmedAt.IsZero() {
		return fmt.Errorf("armedAt is required")
	}
	if s.RecoveryMode == "" {
		return fmt.Errorf("recoveryMode is required")
	}
	if s.ShutdownOrder == "" {
		return fmt.Errorf("shutdownOrder is required")
	}
	if s.ShutdownTickId == "" {
		return fmt.Errorf("shutdownTickId is required")
	}
	if s.EligibleServiceCount <= 0 {
		return fmt.Errorf("eligibleServiceCount must be > 0")
	}
	if len(s.Services) == 0 {
		return fmt.Errorf("services must be non-empty")
	}
	return nil
}

type AppBudgetResetOptions struct {
	ForceClearCooldown bool `param:"force_clear_cooldown"`
	ResetPeriod        bool `param:"reset_period"`
}

type AppBudgetDismissRecoveryResult struct {
	App    string `json:"app"`
	Status string `json:"status"`
}

const (
	BudgetDismissRecoveryStatusDismissed        = "dismissed"
	BudgetDismissRecoveryStatusAlreadyDismissed = "already-dismissed"
	BudgetDismissRecoveryStatusNoBanner         = "no-banner"
)
