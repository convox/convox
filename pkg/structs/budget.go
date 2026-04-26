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

	// Set G (auto-shutdown) annotation keys. Kebab-suffixed per
	// convention. Lifecycle and GC documented in Set G v2 spec §7.
	BudgetShutdownStateAnnotation           = "convox.com/budget-shutdown-state"
	BudgetFlapSuppressedUntilAnnotation     = "convox.com/budget-flap-suppressed-until"
	BudgetRecoveryBannerDismissedAnnotation = "convox.com/budget-recovery-banner-dismissed"
	BudgetFlapSuppressFiredAtAnnotation     = "convox.com/budget-flap-suppress-fired-at"

	// BudgetShutdownStateCorruptFiredAtAnnotation is the dedup annotation
	// for :failed reason="state-corrupt" — written to a SEPARATE
	// annotation key (NOT inside the corrupt state JSON) so the next
	// accumulator tick can still skip the re-fire even though the main
	// state annotation is unparseable. Per spec §3 R5 + F10 fix.
	BudgetShutdownStateCorruptFiredAtAnnotation = "convox.com/budget-shutdown-state-corrupt-fired-at"

	// BudgetShutdownNoopFiredAtAnnotation is the dedup annotation for
	// :noop fired from the shutdownState==nil branch (where the dedup
	// tracker on the state struct cannot be persisted because no state
	// annotation exists). Cleared when shutdownState transitions to
	// non-nil (i.e. :armed fires) or when the cap-fired flag clears.
	// Per spec §7.2 + §9.2 + F9 fix.
	BudgetShutdownNoopFiredAtAnnotation = "convox.com/budget-shutdown-noop-fired-at"

	// KedaPausedReplicasAnnotation is the KEDA-blessed pause primitive
	// (KEDA 2.0+). Set on the ScaledObject at shutdown; cleared via
	// MergePatch null on restore. Survives ScaledObject re-render
	// because the KedaScaledObject builder generates spec.* only.
	KedaPausedReplicasAnnotation = "autoscaling.keda.sh/paused-replicas"

	// BudgetShutdownStateSchemaVersion is the current writer's schema.
	// Future readers (3.25.0+) MUST tolerate v1 transparently per the
	// forward-migration contract in Set G v2 spec §7.3.
	BudgetShutdownStateSchemaVersion = 1

	// Auto-shutdown lifecycle defaults (per Set G v2 spec §2.1).
	BudgetDefaultNotifyBeforeMinutes = 30
	BudgetDefaultShutdownGracePeriod = "5m"
	BudgetDefaultShutdownOrder       = "largest-cost"
	BudgetDefaultRecoveryMode        = "auto-on-reset"

	// Auto-shutdown flap-prevention cooldown (24h per Set G v2 spec §15).
	BudgetFlapCooldown = 24 * time.Hour

	// Auto-shutdown :failed reason classifications per Set G v2 spec §8.7.
	// These are written to AppBudgetShutdownState.FailureReason so the
	// FAILED banner rendered by `convox budget show` can display the
	// canonical reason. Extend, do not rename — webhook receivers parsing
	// the failure_reason field treat values as opaque strings.
	BudgetShutdownReasonK8sApiFailure       = "k8s-api-failure"
	BudgetShutdownReasonStateCorrupt        = "state-corrupt"
	BudgetShutdownReasonAdmissionRejected   = "admission-rejected"
	BudgetShutdownReasonAnnotationRejected  = "annotation-rejected"
	BudgetShutdownReasonCooldownWriteFailed = "cooldown-write-failed"
	BudgetShutdownReasonSchemaIncompatible  = "schema-incompatible"
)

// AppBudget is user-configured spend limits for an app.
type AppBudget struct {
	MonthlyCapUsd         float64 `json:"monthly-cap-usd"`
	AlertThresholdPercent float64 `json:"alert-threshold-percent,omitempty"`
	AtCapAction           string  `json:"at-cap-action,omitempty"`
	PricingAdjustment     float64 `json:"pricing-adjustment,omitempty"`
	// LastCapMutationBy records the JWT-derived user who most recently
	// changed MonthlyCapUsd via AppBudgetSet (or the cap-raise alias).
	// The accumulator reads this on cap-raise detection to populate the
	// `:cancelled reason="cap-raised"` event's `actor` field per spec §8.4.
	// Empty for older racks (pre-3.24.6) or first-write installs — the
	// accumulator falls back to "system" so legacy state stays valid.
	LastCapMutationBy string `json:"last-cap-mutation-by,omitempty"`
}

// AppBudgetState is computed state persisted as a namespace annotation.
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
}

// AppCost is the response shape for GET /apps/{app}/cost.
type AppCost struct {
	App                 string            `json:"app"`
	MonthStart          time.Time         `json:"month-start"`
	AsOf                time.Time         `json:"as-of"`
	SpendUsd            float64           `json:"spend-usd"`
	Breakdown           []ServiceCostLine `json:"breakdown"`
	PricingSource       string            `json:"pricing-source"`
	PricingTableVersion string            `json:"pricing-table-version"`
	PricingAdjustment   float64           `json:"pricing-adjustment"`
	WarningCount        int               `json:"warning-count,omitempty"`
}

// ServiceCostLine is one row in an AppCost.Breakdown — the resource consumption
// and dollar attribution for a single service over the billing window. GpuHours
// / CpuHours / MemGbHours are integrated quantities (hours × allocation), not
// instantaneous values. InstanceType is preserved when available for billing
// audits.
type ServiceCostLine struct {
	Service      string  `json:"service"`
	GpuHours     float64 `json:"gpu-hours"`
	CpuHours     float64 `json:"cpu-hours"`
	MemGbHours   float64 `json:"mem-gb-hours"`
	InstanceType string  `json:"instance-type,omitempty"`
	SpendUsd     float64 `json:"spend-usd"`
	Attribution  string  `json:"attribution,omitempty"`
}

// AppBudgetOptions carries partial updates for AppBudgetSet.
//
// Float-valued fields use *string so they survive stdsdk form marshalling
// (which does not natively support *float64). The server parses these
// strings and rejects non-numeric or out-of-range values.
type AppBudgetOptions struct {
	MonthlyCapUsd         *string `param:"monthly-cap-usd"`
	AlertThresholdPercent *int    `param:"alert-threshold-percent"`
	AtCapAction           *string `param:"at-cap-action"`
	PricingAdjustment     *string `param:"pricing-adjustment"`
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

// AppBudgetSimulationResult is the response body for the
// POST /apps/{app}/budget/simulate-shutdown dry-run endpoint. The
// simulation does not modify cluster state; it returns the eligibility
// list, ordering, and estimated savings the customer would see if a
// real auto-shutdown fired now. Per Set G v2 spec §17.
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

// AppBudgetSimulationEligibility is one row per service in the
// simulation eligibility table. Eligible=false rows include the Reason
// (e.g. "in neverAutoShutdown" or "no service in manifest").
type AppBudgetSimulationEligibility struct {
	Service        string  `json:"service"`
	Eligible       bool    `json:"eligible"`
	Reason         string  `json:"reason,omitempty"`
	Replicas       int     `json:"replicas"`
	CostUsdPerHour float64 `json:"cost-usd-per-hour"`
}

// AppBudgetShutdownState is the JSON shape for the
// `convox.com/budget-shutdown-state` namespace annotation. Per Set G v2
// spec §7.2 this is provider/k8s/-private serialization; JSON tags are
// camelCase per the convention R3 F-NEW-R3-4 carve-out (annotation
// values are not pkg/structs/ surface).
//
// SchemaVersion is currently 1. Required fields (validated post-
// Unmarshal): SchemaVersion, ArmedAt, RecoveryMode, ShutdownOrder,
// ShutdownTickId, EligibleServiceCount > 0, Services non-empty. ShutdownAt,
// RestoredAt, ExpiredAt, FlapSuppressedUntil, ManifestSha256 are nullable.
type AppBudgetShutdownState struct {
	SchemaVersion int `json:"schemaVersion"`

	ShutdownAt *time.Time `json:"shutdownAt"`
	ArmedAt    *time.Time `json:"armedAt"`
	RestoredAt *time.Time `json:"restoredAt"`
	ExpiredAt  *time.Time `json:"expiredAt"`

	// NotifyBeforeMinutes is persisted at arm time so the renderer
	// (CLI banner + STATUS countdown) reads the customer-configured
	// value rather than the 30-minute default. Cross-version compat:
	// older racks lack this field; readers fall back to the default
	// when zero. Per Set G v2 spec §10.10 + 3.24.6 fixup F-18.
	NotifyBeforeMinutes int `json:"notifyBeforeMinutes,omitempty"`

	RecoveryMode   string `json:"recoveryMode"`
	ShutdownOrder  string `json:"shutdownOrder"`
	ShutdownTickId string `json:"shutdownTickId"`
	ManifestSha256 string `json:"manifestSha256"`

	EligibleServiceCount int                             `json:"eligibleServiceCount"`
	Services             []AppBudgetShutdownStateService `json:"services"`

	FlapSuppressedUntil *time.Time `json:"flapSuppressedUntil"`

	// Per-event dedup-firing trackers (8 of 9 events covered;
	// :simulated is CLI-driven and excluded). All fire-once-per-shutdown
	// semantics rely on these. See Set G v2 spec §7.2 + §9.2.
	ArmedNotificationFiredAt          *time.Time `json:"armedNotificationFiredAt"`
	FiredNotificationFiredAt          *time.Time `json:"firedNotificationFiredAt"`
	NoopNotificationFiredAt           *time.Time `json:"noopNotificationFiredAt"`
	CancelledNotificationFiredAt      *time.Time `json:"cancelledNotificationFiredAt"`
	ExpiredNotificationFiredAt        *time.Time `json:"expiredNotificationFiredAt"`
	FlapSuppressedNotificationFiredAt *time.Time `json:"flapSuppressedNotificationFiredAt"`
	FailedNotificationFiredAt         *time.Time `json:"failedNotificationFiredAt"`
	RestoredNotificationFiredAt       *time.Time `json:"restoredNotificationFiredAt"`

	// DiscoveryReason is set on annotations created via the external-
	// edit-detection path (§13.3). When non-empty, the annotation was
	// constructed from observed cluster state, not from the normal
	// :armed/:fired write path. Customer-displayable.
	DiscoveryReason string `json:"discoveryReason,omitempty"`

	// FailureReason is set when a :failed event is fired, capturing the
	// canonical reason enum (e.g. "k8s-api-failure", "state-corrupt",
	// "admission-webhook-rejected", "annotation-rejected") so the FAILED
	// banner rendered by `convox budget show` can display it (per Set G
	// v2 spec §16.3 — `Auto-shutdown FAILED. Reason: <failureReason>.`).
	// Persisted to the state annotation BEFORE :failed fires so the
	// post-failure banner reads the reason from the persisted state.
	FailureReason string `json:"failureReason,omitempty"`
}

// AppBudgetShutdownStateService is one entry in
// AppBudgetShutdownState.Services. Saved at shutdown time; restore
// reads it back to PATCH replicas + grace period back. KedaScaledObject
// is null when the service had no ScaledObject at shutdown.
type AppBudgetShutdownStateService struct {
	Name                       string                              `json:"name"`
	OriginalScale              AppBudgetShutdownStateOriginalScale `json:"originalScale"`
	OriginalGracePeriodSeconds int64                               `json:"originalGracePeriodSeconds"`
	KedaScaledObject           *AppBudgetShutdownStateKeda         `json:"kedaScaledObject"`
	ShutdownSequenceIndex      int                                 `json:"shutdownSequenceIndex"`
	ShutdownAt                 *time.Time                          `json:"shutdownAt"`
}

// AppBudgetShutdownStateOriginalScale captures the per-service replicas
// at shutdown time. count is Deployment.Spec.Replicas; min/max are the
// pre-PATCH ScaledObject values (preserved as observational telemetry —
// PIVOT 1 paused-replicas does NOT modify min/max); replicas is the
// observed Status.Replicas at shutdown time.
type AppBudgetShutdownStateOriginalScale struct {
	Count    int `json:"count"`
	Min      int `json:"min"`
	Max      int `json:"max"`
	Replicas int `json:"replicas"`
}

// AppBudgetShutdownStateKeda carries the KEDA-related per-service
// state. Per Set G v2 spec §7.2 (state-persistence R2 N1 cleanup): only
// name + pausedReplicasAnnotationSet — the v0 savedMin/Max fields were
// dropped (PIVOT 1 paused-replicas never modifies min/max).
type AppBudgetShutdownStateKeda struct {
	Name                        string `json:"name"`
	PausedReplicasAnnotationSet bool   `json:"pausedReplicasAnnotationSet"`
}

// ValidateRequiredFields rejects post-Unmarshal annotation states that
// pass json.Unmarshal but have zero-valued required fields. Per
// Set G v2 spec §3 R5 class 4 (R4 state-persistence A2 absorbed). Go's
// json.Unmarshal does NOT fail on missing fields by default, so this
// step is required for safe load.
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

// AppBudgetResetOptions carries optional knobs for AppBudgetReset.
// ForceClearCooldown (CanAdmin-gated server-side) clears the 24h
// flap-prevention cooldown. Standard reset preserves the cooldown to
// protect against accidental flap re-arm.
type AppBudgetResetOptions struct {
	// MF-12 fix (R6 γ-4 A2): tag is `force_clear_cooldown` (snake) to
	// match the actual wire form set by the manually-coded SDK params at
	// sdk/methods.go:233 and read server-side at pkg/api/controllers.go.
	// The kebab form was a stale tag — manually-constructed SDK params
	// bypass struct-tag marshaling, so the kebab tag never reached the
	// wire. Renaming to snake locks tag, SDK, and server-read in agreement.
	ForceClearCooldown bool `param:"force_clear_cooldown"`
}

// AppBudgetDismissRecoveryResult is the response body for the
// dismiss-recovery endpoint per Set G v2 spec advisory #3 — three
// distinct outcomes the customer must be able to distinguish in the
// CLI:
//
//   - "dismissed"      : a recovery banner was active; it is now dismissed
//   - "already-dismissed" : a recovery banner exists but was previously dismissed
//   - "no-banner"      : no recovery banner is active for this app
type AppBudgetDismissRecoveryResult struct {
	App    string `json:"app"`
	Status string `json:"status"`
}

// AppBudgetDismissRecovery status enum.
const (
	BudgetDismissRecoveryStatusDismissed        = "dismissed"
	BudgetDismissRecoveryStatusAlreadyDismissed = "already-dismissed"
	BudgetDismissRecoveryStatusNoBanner         = "no-banner"
)
