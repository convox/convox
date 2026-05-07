package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/audit"
	"github.com/convox/convox/pkg/billing"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	budgetLeaseName            = "convox-budget-accumulator"
	budgetDefaultPollInterval  = 10 * time.Minute
	budgetMinPollInterval      = 1 * time.Minute
	budgetMaxPollInterval      = 1 * time.Hour
	budgetWriteConflictRetries = 3
	// budgetTickShutdownGrace bounds how long runBudgetAccumulator waits
	// for an in-flight tick to drain after ctx cancel before logging
	// at=shutdown_timeout and returning. Picked to be longer than the
	// usual k8s API call (sub-second under healthy conditions, up to
	// ~2-3s under load) but short enough that an api-pod SIGTERM during
	// a rack update is not blocked indefinitely on a stuck namespace
	// Get/Update.
	budgetTickShutdownGrace = 5 * time.Second
)

// AppBudgetGet returns the app's budget config and state from namespace annotations.
// Returns (nil, nil, nil) when no budget is configured.
func (p *Provider) AppBudgetGet(app string) (*structs.AppBudget, *structs.AppBudgetState, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil, nil, errors.WithStack(structs.ErrNotFound("app not found: %s", app))
		}
		return nil, nil, errors.WithStack(err)
	}

	cfg, _ := readBudgetConfigAnnotation(ns.Annotations)
	state, _ := readBudgetStateAnnotation(ns.Annotations)

	return cfg, state, nil
}

// AppBudgetSet upserts the budget config via a namespace annotation patch
// and emits an app:budget:set audit event so downstream receivers (and a
// grep-able stdout log) see the cap/threshold/action transition with the
// asserting actor identity.
func (p *Provider) AppBudgetSet(app string, opts structs.AppBudgetOptions, ackBy string) error {
	// Reject enforcement-bearing options when cost tracking is disabled —
	// without the accumulator running, caps and alerts persist as
	// unenforced config (silent no-op). The check is a pure read of opts
	// + an env var, so it runs before the per-app lock to avoid
	// serializing rejections behind in-progress writes.
	if err := p.requireCostTrackingForBudget(opts); err != nil {
		return err
	}

	// Per-app advisory lock — serializes against AppBudgetReset and the
	// accumulator's reconcileAutoShutdown so the breaker-clear path
	// cannot interleave with their reads-then-decides-then-writes on
	// CircuitBreakerTripped / AlertFiredAt*. Without this, a concurrent
	// reconcileAutoShutdown could observe pre-clear state, decide to
	// fire :armed, and write a stale decision after our clear lands.
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

	nsName := p.AppNamespace(app)
	ackBy = sanitizeAckBy(ackBy)

	var prev structs.AppBudget
	var final structs.AppBudget
	var breakerClearedFromCapRaise bool
	var breakerClearedPrevSpend, breakerClearedPrevCap, breakerClearedNewCap float64
	var breakerClearedAckAt time.Time
	// F-A06-2 fix: capture the armed shutdown-state at the moment of
	// cap-raise so we can emit :cancelled reason="cap-raised" + delete
	// the orphan annotation atomically with the breaker-clear update.
	// Without this, the annotation persists with ArmedAt set and the
	// `convox budget show` banner reads stale "ARMED" indefinitely.
	var capRaiseArmedShutdownState *structs.AppBudgetShutdownState
	var capRaiseShutdownStateBaseState *structs.AppBudgetState

	for i := 0; i < budgetWriteConflictRetries; i++ {
		// Reset every per-iteration capture so a prior iteration's gate
		// firing cannot leak values into a successful retry where the
		// gate doesn't fire (defensive against future reorders).
		breakerClearedFromCapRaise = false
		breakerClearedPrevSpend = 0
		breakerClearedPrevCap = 0
		breakerClearedNewCap = 0
		breakerClearedAckAt = time.Time{}
		capRaiseArmedShutdownState = nil
		capRaiseShutdownStateBaseState = nil

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return errors.WithStack(structs.ErrNotFound("app not found: %s", app))
			}
			return errors.WithStack(err)
		}

		cfg, _ := readBudgetConfigAnnotation(ns.Annotations)
		if cfg == nil {
			cfg = &structs.AppBudget{}
		}
		prev = *cfg
		if err := applyBudgetOptions(cfg, opts); err != nil {
			return errors.WithStack(err)
		}
		// MF-6 fix (R6 γ-1 carry-forward NIT): record the JWT-derived
		// caller on every cap-changing mutation so the accumulator can
		// surface the originating user when it later fires
		// :cancelled reason="cap-raised". Spec §8.4 line 777 mandates
		// JWT-derived actor for cap-raise; previously hardcoded "system"
		// because the detection runs in the accumulator goroutine where
		// no HTTP context is in scope.
		if opts.MonthlyCapUsd != nil {
			cfg.LastCapMutationBy = ackBy
		}
		cfg.ApplyDefaults()
		if err := cfg.Validate(); err != nil {
			return errors.WithStack(structs.ErrBadRequest("%s", err.Error()))
		}
		final = *cfg

		// When the user truly RAISES the monthly cap (new > prev)
		// AND the new cap is above current month-to-date spend AND the
		// breaker is currently tripped, clear the breaker atomically
		// with the config write. The 409 body's option (2) "raise the
		// cap to unblock deploys" then actually works without a
		// separate `convox budget reset`.
		//
		// The `final > prev` check is the dominant guard against:
		//   (a) no-op cap "set" (same value) clearing a stuck breaker
		//       and emitting a misleading audit event labeled
		//       "cap-raised" with prev_cap == new_cap.
		//   (b) cap LOWERED while still > spend silently clearing a
		//       breaker. Per Decision 3 §1, cap-lower is orthogonal to
		//       breaker-clear; only an explicit raise should unblock.
		//   (c) partial AppBudgetSet calls that don't touch
		//       MonthlyCapUsd at all. applyBudgetOptions only mutates
		//       cfg.MonthlyCapUsd when opts.MonthlyCapUsd != nil, and
		//       ApplyDefaults never touches MonthlyCapUsd, so a partial
		//       update leaves final == prev and `final > prev` is false.
		//       (No separate opts.MonthlyCapUsd != nil clause is needed
		//       — it would be subsumed by `final > prev` and add no
		//       coverage.)
		//
		// Edge case: cap raised to a value still <= current spend. The
		// cap-raise persists but the breaker stays tripped (user
		// hasn't actually solved the over-cap problem). At the next
		// accumulator tick willFireCap evaluates against the new cap; if
		// spend still >= new cap, the breaker stays tripped.
		state, _ := readBudgetStateAnnotation(ns.Annotations)
		if state != nil && state.CircuitBreakerTripped &&
			final.MonthlyCapUsd > prev.MonthlyCapUsd &&
			final.MonthlyCapUsd > state.CurrentMonthSpendUsd {
			breakerClearedPrevSpend = state.CurrentMonthSpendUsd
			breakerClearedPrevCap = prev.MonthlyCapUsd
			breakerClearedNewCap = final.MonthlyCapUsd
			breakerClearedAckAt = time.Now().UTC()
			state.CircuitBreakerTripped = false
			state.AlertFiredAtThreshold = time.Time{}
			state.AlertFiredAtCap = time.Time{}
			state.CircuitBreakerAckBy = ackBy
			state.CircuitBreakerAckAt = breakerClearedAckAt
			breakerClearedFromCapRaise = true

			// F-A06-2 fix: when the breaker clears via cap-raise AND the
			// app was in :armed lifecycle (ArmedAt set, ShutdownAt nil),
			// also delete the BudgetShutdownStateAnnotation atomically
			// with the breaker-clear write. Otherwise the orphan armed
			// annotation persists and `convox budget show` displays a
			// stale "ARMED — auto-shutdown scheduled at HH:MM" banner
			// forever (the accumulator's reconcileAutoShutdown gates the
			// :fired path on AlertFiredAtCap.IsZero(), so it can never
			// progress). Capture the state for the post-Update :cancelled
			// emit; clear the annotation here so the same Namespace
			// Update lands both the breaker-clear and the annotation
			// delete in one round-trip. The locked AppBudgetSet entry
			// point already serializes against reconcileAutoShutdown
			// (per appBudgetLock surface), so the next tick will read a
			// clean state.
			if shutdownState, _ := readBudgetShutdownStateAnnotation(ns.Annotations); shutdownState != nil &&
				shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
				(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
				capRaiseArmedShutdownState = shutdownState
				capRaiseShutdownStateBaseState = state
				delete(ns.Annotations, structs.BudgetShutdownStateAnnotation)
			}
		}

		data, err := json.Marshal(cfg)
		if err != nil {
			return errors.WithStack(err)
		}

		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetConfigAnnotation] = string(data)

		if breakerClearedFromCapRaise {
			// State annotation is updated atomically with the config in
			// the same Namespaces().Update() call below — k8s atomic per
			// object, so the conflict-retry loop covers both annotations.
			stateData, err := json.Marshal(state)
			if err != nil {
				return errors.WithStack(err)
			}
			ns.Annotations[structs.BudgetStateAnnotation] = string(stateData)
		}

		if _, err := p.Cluster.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}

		fmt.Printf("ns=budget_accumulator at=alert kind=set app=%s ack_by=%q prev_cap_usd=%.2f new_cap_usd=%.2f prev_action=%q new_action=%q prev_pct=%.0f new_pct=%.0f prev_adj=%.2f new_adj=%.2f\n",
			app, ackBy, prev.MonthlyCapUsd, final.MonthlyCapUsd, prev.AtCapAction, final.AtCapAction,
			prev.AlertThresholdPercent, final.AlertThresholdPercent, prev.PricingAdjustment, final.PricingAdjustment)
		_ = p.EventSend("app:budget:set", structs.EventSendOptions{Data: map[string]string{
			"app":             app,
			"ack_by":          ackBy,
			"prev_cap_usd":    strconv.FormatFloat(prev.MonthlyCapUsd, 'f', 2, 64),
			"new_cap_usd":     strconv.FormatFloat(final.MonthlyCapUsd, 'f', 2, 64),
			"prev_action":     prev.AtCapAction,
			"new_action":      final.AtCapAction,
			"prev_pct":        strconv.FormatFloat(prev.AlertThresholdPercent, 'f', 0, 64),
			"new_pct":         strconv.FormatFloat(final.AlertThresholdPercent, 'f', 0, 64),
			"prev_adjustment": strconv.FormatFloat(prev.PricingAdjustment, 'f', 2, 64),
			"new_adjustment":  strconv.FormatFloat(final.PricingAdjustment, 'f', 2, 64),
			"set_at":          time.Now().UTC().Format(time.RFC3339),
		}})

		if breakerClearedFromCapRaise {
			// Discrete :breaker-cleared event with reason="cap-raised" so
			// the audit trail shows that a cap-raise (not a manual reset)
			// unblocked deploys. Mirrors the :reset event shape so existing
			// webhook consumers can generalize. Idempotent — emitted only
			// when the breaker-clear gate actually fired.
			//
			// cleared_at uses the same timestamp persisted into
			// state.CircuitBreakerAckAt so the audit-event field and the
			// k8s annotation field are bit-exact rather than drifting by
			// the microseconds between two time.Now() calls.
			fmt.Printf("ns=budget_accumulator at=alert kind=breaker_cleared app=%s ack_by=%q reason=cap-raised prev_spend_usd=%.2f prev_cap_usd=%.2f new_cap_usd=%.2f\n",
				app, ackBy, breakerClearedPrevSpend, breakerClearedPrevCap, breakerClearedNewCap)
			_ = p.EventSend("app:budget:breaker-cleared", structs.EventSendOptions{Data: map[string]string{
				"app":            app,
				"ack_by":         ackBy,
				"reason":         "cap-raised",
				"prev_spend_usd": strconv.FormatFloat(breakerClearedPrevSpend, 'f', 2, 64),
				"prev_cap_usd":   strconv.FormatFloat(breakerClearedPrevCap, 'f', 2, 64),
				"new_cap_usd":    strconv.FormatFloat(breakerClearedNewCap, 'f', 2, 64),
				"cleared_at":     breakerClearedAckAt.Format(time.RFC3339),
			}})

			// F-A06-2 fix: cap-raise during :armed lifecycle deletes the
			// orphan shutdown-state annotation (above, in the same Update
			// round-trip) and surfaces the lifecycle cancellation as
			// :cancelled reason="cap-raised". Receivers see the audit
			// pair (:breaker-cleared then :cancelled) for one user
			// action; the :cancelled actor is the cap-raiser (ackBy)
			// matching spec §8.4 line 777 JWT-derived attribution. The
			// annotation-delete-before-emit ordering matches the F-3
			// pattern at budget_accumulator.go reset-during-armed (and
			// the F-20 persist-then-emit family more generally).
			if capRaiseArmedShutdownState != nil {
				p.fireCancelledEvent(app, &final, capRaiseShutdownStateBaseState, capRaiseArmedShutdownState, ackBy, "cap-raised", breakerClearedPrevCap, breakerClearedNewCap, "", breakerClearedAckAt)
			}
		}

		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to set budget after %d retries", budgetWriteConflictRetries))
}

// AppBudgetClear removes both the budget config and the accumulated state.
// State must be cleared too: leaving a tripped breaker behind a cleared
// config would keep deploys blocked with no config to reset via. Emits an
// app:budget:clear event with the FULL prior-state snapshot so an auditor
// can reconstruct what was destroyed (spend, cap, alert timestamps,
// breaker state, prior ack).
func (p *Provider) AppBudgetClear(app string, ackBy string) error {
	nsName := p.AppNamespace(app)
	ackBy = sanitizeAckBy(ackBy)

	var prevCfg *structs.AppBudget
	var prevState *structs.AppBudgetState
	var cleared bool

	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return errors.WithStack(structs.ErrNotFound("app not found: %s", app))
			}
			return errors.WithStack(err)
		}

		if ns.Annotations == nil {
			return nil
		}
		_, hasCfg := ns.Annotations[structs.BudgetConfigAnnotation]
		_, hasState := ns.Annotations[structs.BudgetStateAnnotation]
		if !hasCfg && !hasState {
			return nil
		}

		prevCfg, _ = readBudgetConfigAnnotation(ns.Annotations)
		prevState, _ = readBudgetStateAnnotation(ns.Annotations)

		delete(ns.Annotations, structs.BudgetConfigAnnotation)
		delete(ns.Annotations, structs.BudgetStateAnnotation)

		if _, err := p.Cluster.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}
		cleared = true
		break
	}
	if !cleared {
		return errors.WithStack(fmt.Errorf("failed to clear budget after %d retries", budgetWriteConflictRetries))
	}

	data := map[string]string{
		"app":        app,
		"ack_by":     ackBy,
		"cleared_at": time.Now().UTC().Format(time.RFC3339),
	}
	var prevCap, prevSpend float64
	var prevAction string
	if prevCfg != nil {
		prevCap = prevCfg.MonthlyCapUsd
		prevAction = prevCfg.AtCapAction
	}
	var prevBreaker bool
	var prevAckBy string
	var prevThreshFired, prevCapFired string
	if prevState != nil {
		prevSpend = prevState.CurrentMonthSpendUsd
		prevBreaker = prevState.CircuitBreakerTripped
		// Defense in depth: a direct kubectl-edit of the annotation could have
		// stored an unsanitized ackBy. Re-sanitize on read before echoing.
		prevAckBy = sanitizeAckBy(prevState.CircuitBreakerAckBy)
		if !prevState.AlertFiredAtThreshold.IsZero() {
			prevThreshFired = prevState.AlertFiredAtThreshold.UTC().Format(time.RFC3339)
		}
		if !prevState.AlertFiredAtCap.IsZero() {
			prevCapFired = prevState.AlertFiredAtCap.UTC().Format(time.RFC3339)
		}
	}
	data["prev_spend_usd"] = strconv.FormatFloat(prevSpend, 'f', 2, 64)
	data["prev_cap_usd"] = strconv.FormatFloat(prevCap, 'f', 2, 64)
	data["prev_at_cap_action"] = prevAction
	data["prev_breaker_tripped"] = strconv.FormatBool(prevBreaker)
	data["prev_ack_by"] = prevAckBy
	data["prev_alert_fired_at_threshold"] = prevThreshFired
	data["prev_alert_fired_at_cap"] = prevCapFired

	fmt.Printf("ns=budget_accumulator at=alert kind=clear app=%s ack_by=%q prev_spend_usd=%.2f prev_cap_usd=%.2f prev_action=%q prev_breaker_tripped=%t prev_ack_by=%q\n",
		app, ackBy, prevSpend, prevCap, prevAction, prevBreaker, prevAckBy)
	_ = p.EventSend("app:budget:clear", structs.EventSendOptions{Data: data})
	return nil
}

// AppBudgetReset atomically clears CircuitBreakerTripped, AlertFiredAtThreshold,
// and AlertFiredAtCap so the alert + breaker machinery re-arms for the rest
// of the month. Records ackBy + ackAt for audit and fires an
// app:budget:reset event. Resilient to missing config — if the user cleared
// config while the breaker was tripped, reset must still unblock deploys.
//
// Public entry point — acquires the per-app advisory lock and delegates
// to the locked helper. AppBudgetResetWithOptions calls the locked
// helper directly so Step 1 (breaker clear) and Step 2 (annotation
// restore + delete) execute atomically under a single lock acquisition.
func (p *Provider) AppBudgetReset(app string, ackBy string) error {
	// F-19 fix (catalog D-7): per-app advisory lock around the reset
	// path so the accumulator's reconcileAutoShutdown cannot race with
	// reset and emit two `:cancelled` events with different reasons.
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()
	return p.appBudgetResetLocked(app, ackBy)
}

// appBudgetResetLocked is the lock-already-held variant of
// AppBudgetReset. Caller MUST hold appBudgetLock(app) for the duration
// of the call. Internal helper — used by AppBudgetReset (which acquires
// the lock first) and AppBudgetResetWithOptions (which acquires the
// lock at the outer scope so Step 2 restoreFromAnnotation runs under
// the same critical section, closing the F-A06-1 race where a
// concurrent accumulator tick could fire its own emit between Step 1
// breaker clear and Step 2 annotation delete).
func (p *Provider) appBudgetResetLocked(app string, ackBy string) error {
	nsName := p.AppNamespace(app)

	ackBy = sanitizeAckBy(ackBy)

	// Capture a single stable timestamp for both the CircuitBreakerAckAt field
	// and the fireCancelledEvent call so all audit fields agree on when the
	// reset was requested. Using separate time.Now() calls could drift by
	// milliseconds if the namespace update conflicts and loops.
	now := time.Now().UTC()

	var prevSpend, capUsd float64

	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), nsName, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				return errors.WithStack(structs.ErrNotFound("app not found: %s", app))
			}
			return errors.WithStack(err)
		}

		cfg, _ := readBudgetConfigAnnotation(ns.Annotations)
		state, _ := readBudgetStateAnnotation(ns.Annotations)
		if state == nil {
			state = &structs.AppBudgetState{MonthStart: startOfMonth(time.Now().UTC())}
		}

		prevSpend = state.CurrentMonthSpendUsd
		if cfg != nil {
			capUsd = cfg.MonthlyCapUsd
		}

		state.CircuitBreakerTripped = false
		state.AlertFiredAtThreshold = time.Time{}
		state.AlertFiredAtCap = time.Time{}
		state.CircuitBreakerAckBy = ackBy
		state.CircuitBreakerAckAt = now

		data, err := json.Marshal(state)
		if err != nil {
			return errors.WithStack(err)
		}

		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetStateAnnotation] = string(data)

		if _, err := p.Cluster.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}

		fmt.Printf("ns=budget_accumulator at=alert kind=reset app=%s ack_by=%q prev_spend_usd=%.2f cap_usd=%.2f\n",
			app, ackBy, prevSpend, capUsd)
		_ = p.EventSend("app:budget:reset", structs.EventSendOptions{Data: map[string]string{
			"app":            app,
			"ack_by":         ackBy,
			"prev_spend_usd": strconv.FormatFloat(prevSpend, 'f', 2, 64),
			"cap_usd":        strconv.FormatFloat(capUsd, 'f', 2, 64),
			"reset_at":       state.CircuitBreakerAckAt.Format(time.RFC3339),
		}})

		// F1 + F8 fix: if a shutdown-state annotation exists in the
		// armed-window state (armedAt set, shutdownAt nil), reset
		// arrived during the notify window. Fire :cancelled
		// reason="reset-during-armed" and GC the orphan annotation so
		// next cap re-breach re-arms cleanly. Best-effort — do not
		// abort the reset on any annotation failure.
		// R7.5 F-3 fix: GC annotation BEFORE emit (matches F-20 dedup-
		// write-then-emit pattern). If delete fails, abort emit so the
		// next accumulator tick re-detects and retries cleanly via the
		// F8 manual-detected branch (or this site if reset reruns).
		// NotFound is treated as success (already GC'd).
		shutdownState, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
		if parseErr == nil && shutdownState != nil &&
			shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
			(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
			ctx := context.TODO()
			derr := p.deleteBudgetShutdownStateAnnotation(ctx, app)
			if derr == nil || ae.IsNotFound(derr) {
				p.fireCancelledEvent(app, cfg, state, shutdownState, ackBy, "reset-during-armed", 0, 0, "", now)
			}
		}
		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to reset budget after %d retries", budgetWriteConflictRetries))
}

// sanitizeAckBy caps the ack_by audit string and strips control characters.
// Guards against annotation-size DoS and webhook/log injection via
// unvalidated client input. Behavior + strip rules documented on the
// underlying canonical implementation at pkg/audit/sanitize.go.
//
// The implementation was promoted from this package into pkg/audit so the
// rack auth middleware (pkg/api) can sanitize header-supplied actor
// strings without crossing a layering boundary (importing provider/k8s
// from pkg/api would invert the dependency direction). This wrapper is
// retained so existing in-package callers keep their current call shape
// and the budget_cost_test.go suite continues to act as a regression
// guard against accidental sanitizer drift across the move.
func sanitizeAckBy(in string) string {
	return audit.SanitizeActor(in)
}

// AppCost returns the computed spend summary for the app.
func (p *Provider) AppCost(app string) (*structs.AppCost, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil, errors.WithStack(structs.ErrNotFound("app not found: %s", app))
		}
		return nil, errors.WithStack(err)
	}

	cfg, _ := readBudgetConfigAnnotation(ns.Annotations)
	state, _ := readBudgetStateAnnotation(ns.Annotations)
	if state == nil {
		state = &structs.AppBudgetState{MonthStart: startOfMonth(time.Now().UTC())}
	}

	adjustment := 1.0
	if cfg != nil && cfg.PricingAdjustment > 0 {
		adjustment = cfg.PricingAdjustment
	}

	return &structs.AppCost{
		App:                 app,
		MonthStart:          state.MonthStart,
		AsOf:                state.CurrentMonthSpendAsOf,
		SpendUsd:            state.CurrentMonthSpendUsd,
		Breakdown:           buildBreakdown(state),
		PricingSource:       "on-demand-static-table",
		PricingTableVersion: billing.PricingTableVersion(),
		PricingAdjustment:   adjustment,
		WarningCount:        state.WarningCount,
	}, nil
}

// buildBreakdown projects the persisted per-service totals into an
// AppCost.Breakdown slice. Always returns a non-nil slice (matching the
// existing wire shape); empty when no per-service data has accumulated.
//
// The state struct passed in must be a freshly-loaded copy, NOT a shared
// in-process pointer also visible to the accumulator goroutine —
// iterating PerServiceSpendUsd while the tick path is mutating the same
// map races. AppCost performs a fresh annotation read per call; the
// accumulator deserializes into its own *AppBudgetState; the two paths
// never share memory.
//
// GpuHours / CpuHours / MemGbHours are intentionally zero — the existing
// pricing model returns a single dominant-resource fraction so per-resource
// hours can't be derived without a redesign. The wire fields are retained
// without omitempty for forward-compatibility with a future per-resource
// pricing model; they always serialize as 0 in 3.24.6 so downstream
// parsers see a stable shape.
func buildBreakdown(state *structs.AppBudgetState) []structs.ServiceCostLine {
	if state == nil || len(state.PerServiceSpendUsd) == 0 {
		return []structs.ServiceCostLine{}
	}

	out := make([]structs.ServiceCostLine, 0, len(state.PerServiceSpendUsd))
	for svc, spend := range state.PerServiceSpendUsd {
		out = append(out, structs.ServiceCostLine{
			Service:      svc,
			SpendUsd:     spend,
			InstanceType: state.PerServiceInstanceType[svc],
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SpendUsd != out[j].SpendUsd {
			return out[i].SpendUsd > out[j].SpendUsd
		}
		return out[i].Service < out[j].Service
	})

	return out
}

func applyBudgetOptions(dst *structs.AppBudget, opts structs.AppBudgetOptions) error {
	if opts.MonthlyCapUsd != nil {
		v, err := strconv.ParseFloat(*opts.MonthlyCapUsd, 64)
		if err != nil {
			return structs.ErrBadRequest("monthly-cap-usd must be a number: %v", err)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return structs.ErrBadRequest("monthly-cap-usd must be a finite number")
		}
		dst.MonthlyCapUsd = v
	}
	if opts.AlertThresholdPercent != nil {
		dst.AlertThresholdPercent = float64(*opts.AlertThresholdPercent)
	}
	if opts.AtCapAction != nil {
		dst.AtCapAction = *opts.AtCapAction
	}
	if opts.PricingAdjustment != nil {
		v, err := strconv.ParseFloat(*opts.PricingAdjustment, 64)
		if err != nil {
			return structs.ErrBadRequest("pricing-adjustment must be a number: %v", err)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return structs.ErrBadRequest("pricing-adjustment must be a finite number")
		}
		dst.PricingAdjustment = v
	}
	return nil
}

func readBudgetConfigAnnotation(ann map[string]string) (*structs.AppBudget, error) {
	raw, ok := ann[structs.BudgetConfigAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	var b structs.AppBudget
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

func readBudgetStateAnnotation(ann map[string]string) (*structs.AppBudgetState, error) {
	raw, ok := ann[structs.BudgetStateAnnotation]
	if !ok || raw == "" {
		return nil, nil
	}
	var s structs.AppBudgetState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// runBudgetAccumulator is invoked by the leader-election callback; runs
// until ctx cancels. Panics are recovered per-tick so a single bad tick
// cannot silently kill the loop while this pod keeps the lease renewed.
//
// Lifecycle: each safeBudgetTick invocation runs in a tracked goroutine
// so a ctx cancellation arriving mid-tick (api-pod SIGTERM, leadership
// loss) interleaves with the for-select instead of waiting for the tick
// to complete synchronously. On ctx.Done the loop calls wg.Wait() with a
// budgetTickShutdownGrace deadline; if the in-flight tick honors ctx
// cancellation through the threaded ctx (B.2), wg.Wait returns promptly
// and the loop logs at=stop. If the tick is wedged past the grace
// window, the loop logs at=shutdown_timeout and returns anyway --
// blocking the api pod indefinitely on a stuck k8s call would defeat
// graceful shutdown.
func (p *Provider) runBudgetAccumulator(ctx context.Context) {
	interval := budgetDefaultPollInterval
	if v := os.Getenv("BUDGET_POLL_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			if d < budgetMinPollInterval {
				d = budgetMinPollInterval
			}
			if d > budgetMaxPollInterval {
				d = budgetMaxPollInterval
			}
			interval = d
		} else {
			fmt.Printf("ns=budget_accumulator at=interval_parse_failed value=%q error=%q falling_back=%s\n", v, err, interval)
		}
	}

	fmt.Printf("ns=budget_accumulator at=start interval=%s\n", interval)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.safeBudgetTick(ctx)
	}()

	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
				fmt.Printf("ns=budget_accumulator at=stop\n")
			case <-time.After(budgetTickShutdownGrace):
				fmt.Printf("ns=budget_accumulator at=shutdown_timeout grace=%s\n", budgetTickShutdownGrace)
			}
			return
		case <-tick.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				p.safeBudgetTick(ctx)
			}()
		}
	}
}

// safeBudgetTick runs one tick with its own panic-recovery scope so a
// goroutine that panics once can still run on the next interval. The ctx
// is the leader-election context threaded from runBudgetAccumulator so a
// graceful api-pod shutdown (SIGTERM during a rack update) can cancel any
// in-flight namespace Get/Update mid-tick instead of orphaning the
// goroutine on the client-go default timeout.
func (p *Provider) safeBudgetTick(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=budget_accumulator at=panic recover=%q stack=%q\n", r, debug.Stack())
		}
	}()
	if err := p.accumulateBudgetTick(ctx); err != nil {
		fmt.Printf("ns=budget_accumulator at=tick error=%q\n", err)
	}
}

// accumulateBudgetTick iterates all apps with budget configured and updates
// spend + alert + breaker state. The namespace list is rack-scoped so shared
// clusters with two V3 racks do not cross-attribute. The ctx is forwarded
// through accumulateBudgetApp into the namespace Get/Update RPCs so a
// graceful shutdown cancels any in-flight write at the client-go layer.
func (p *Provider) accumulateBudgetTick(ctx context.Context) error {
	// Defense in depth: an empty rack name would produce a selector like
	// `rack=` which matches nothing in a healthy cluster, but short-circuit
	// explicitly to avoid the wasted API call and to make the intent clear
	// in source.
	if p.Name == "" {
		return nil
	}

	// Cost tracking is universal when the rack-level switch is on — every
	// app namespace is walked, not just those with budget config set. The
	// per-app function (accumulateBudgetApp) routes the work: cost-only
	// tracking when cfg is nil; cost tracking PLUS budget enforcement
	// (threshold / cap / auto-shutdown) when cfg is set. Documented
	// behavior per docs/management/cost-tracking.md — cost_tracking_enable
	// gates the accumulator at the rack tier; per-app budget config is
	// only required for cap enforcement, not for spend visibility.
	if !p.costTrackingEnabled() {
		return nil
	}
	selector := fmt.Sprintf("system=convox,rack=%s,type=app", p.Name)
	ns, err := p.ListNamespacesFromInformer(selector)
	if err != nil {
		return errors.WithStack(err)
	}

	now := time.Now().UTC()

	for i := range ns.Items {
		// B.3: abort the per-app walk promptly if ctx cancels mid-tick
		// (api-pod SIGTERM, leadership loss). Without this check the
		// loop would walk every namespace before noticing cancellation
		// since the per-app k8s calls are the only natural cancellation
		// points and a cached informer Get returns instantly.
		if err := ctx.Err(); err != nil {
			return err
		}
		n := &ns.Items[i]
		app := n.Labels["app"]
		if app == "" {
			continue
		}
		if err := p.accumulateBudgetApp(ctx, app, now); err != nil {
			fmt.Printf("ns=budget_accumulator app=%s error=%q\n", app, err)
		}
	}
	return nil
}

func (p *Provider) accumulateBudgetApp(ctx context.Context, app string, now time.Time) error {
	nsName := p.AppNamespace(app)

	for i := 0; i < budgetWriteConflictRetries; i++ {
		// B.3: abort the retry loop promptly if ctx cancels between
		// retries (e.g. shutdown arrives during a backoff after a write
		// conflict). The Namespaces().Get below also honors ctx, but
		// checking here surfaces ctx.Err() as the return value rather
		// than burying it inside an errors.WithStack of an HTTP error.
		if err := ctx.Err(); err != nil {
			return err
		}
		ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
		if err != nil {
			return errors.WithStack(err)
		}
		cfg, err := readBudgetConfigAnnotation(ns.Annotations)
		if err != nil {
			fmt.Printf("ns=budget_accumulator at=config_parse app=%s error=%q\n", app, err)
			return nil
		}
		// cfg may be nil — that's the cost-tracking-only path (no budget
		// cap configured for this app). The accumulator still computes
		// spend deltas and updates PerServiceSpendUsd so `convox cost`
		// and the Console budget panel report real numbers; the threshold
		// / cap / auto-shutdown enforcement below is gated on cfg != nil.
		//
		// Defense in depth (config-set path only): a corrupt annotation
		// with MonthlyCapUsd <= 0, NaN, or Inf would otherwise fire a cap
		// alert on every tick or silently suppress alerts (NaN
		// comparisons return false). Validate() rejects these at write
		// time, but hand-edited annotations could bypass that.
		if cfg != nil && (cfg.MonthlyCapUsd <= 0 || math.IsNaN(cfg.MonthlyCapUsd) || math.IsInf(cfg.MonthlyCapUsd, 0)) {
			fmt.Printf("ns=budget_accumulator at=invalid_cap app=%s cap=%v\n", app, cfg.MonthlyCapUsd)
			return nil
		}
		state, err := readBudgetStateAnnotation(ns.Annotations)
		if err != nil {
			fmt.Printf("ns=budget_accumulator at=state_parse app=%s error=%q\n", app, err)
			state = nil
		}
		if state == nil {
			state = &structs.AppBudgetState{MonthStart: startOfMonth(now), CurrentMonthSpendAsOf: now}
		}

		if startOfMonth(now).After(state.MonthStart) {
			state = &structs.AppBudgetState{MonthStart: startOfMonth(now), CurrentMonthSpendAsOf: now}
		}

		// Pricing adjustment defaults to 1.0 when no budget config is set
		// (cost-tracking-only path). This matches AppCost's default and
		// the docs at docs/management/cost-tracking.md ("pricingAdjustment
		// of 1.0 = no adjustment").
		adjustment := 1.0
		if cfg != nil && cfg.PricingAdjustment > 0 {
			adjustment = cfg.PricingAdjustment
		}
		delta, perSvc, perSvcInst, warnings, err := p.computeBudgetDelta(ctx, app, state.CurrentMonthSpendAsOf, now, adjustment)
		if err != nil {
			return err
		}

		state.CurrentMonthSpendUsd += delta
		state.CurrentMonthSpendAsOf = now
		state.WarningCount = warnings

		// Merge per-service deltas into accumulating state. Pre-rc5
		// annotations parse with nil maps; lazy-init here.
		if state.PerServiceSpendUsd == nil {
			state.PerServiceSpendUsd = map[string]float64{}
		}
		if state.PerServiceInstanceType == nil {
			state.PerServiceInstanceType = map[string]string{}
		}
		truncated := 0
		for svc, dollars := range perSvc {
			if _, exists := state.PerServiceSpendUsd[svc]; !exists && len(state.PerServiceSpendUsd) >= perServiceMaxEntries {
				truncated++
				continue
			}
			state.PerServiceSpendUsd[svc] += dollars
			if _, hadIT := state.PerServiceInstanceType[svc]; !hadIT {
				if it := perSvcInst[svc]; it != "" {
					state.PerServiceInstanceType[svc] = it
				}
			}
		}
		if truncated > 0 {
			// Surface the truncation as a user-observable event AND a
			// log line. The event lands in `convox events list` so a
			// user who hits the cap learns that some services are
			// being dropped from this month's breakdown without needing
			// API-server log access. The log line keeps the operational
			// signal visible in rack logs for support diagnosis. Pin
			// actor=system to match the threshold/cap accumulator-fired
			// events below — without the pin, central injection falls
			// through to ContextActor() which is "unknown" in the
			// accumulator goroutine and would surface inconsistently.
			_ = p.EventSend("app:budget:per-service-truncated", structs.EventSendOptions{Data: map[string]string{
				"app":     app,
				"dropped": strconv.Itoa(truncated),
				"cap":     strconv.Itoa(perServiceMaxEntries),
				"actor":   "system",
			}})
			fmt.Printf("ns=budget_accumulator at=per_service_truncated app=%s count=%d cap=%d\n", app, truncated, perServiceMaxEntries)
		}

		// Threshold + cap enforcement only fires when a budget config is
		// set. Cost-tracking-only path (cfg == nil) skips straight to
		// state persistence.
		var willFireThreshold, willFireCap bool
		if cfg != nil {
			willFireThreshold = !state.CurrentMonthSpendAsOf.IsZero() &&
				state.CurrentMonthSpendUsd >= cfg.MonthlyCapUsd*(cfg.AlertThresholdPercent/100) &&
				state.AlertFiredAtThreshold.IsZero()
			willFireCap = state.CurrentMonthSpendUsd >= cfg.MonthlyCapUsd && state.AlertFiredAtCap.IsZero()

			if willFireThreshold {
				state.AlertFiredAtThreshold = now
			}
			if willFireCap {
				state.AlertFiredAtCap = now
				if cfg.AtCapAction == structs.BudgetAtCapActionBlockNewDeploys {
					state.CircuitBreakerTripped = true
				}
			}
		}

		data, err := json.Marshal(state)
		if err != nil {
			return errors.WithStack(err)
		}

		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetStateAnnotation] = string(data)

		if _, err := p.Cluster.CoreV1().Namespaces().Update(ctx, ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}

		if willFireThreshold {
			fmt.Printf("ns=budget_accumulator at=alert kind=threshold app=%s spend_usd=%.2f cap_usd=%.2f pct=%.0f\n",
				app, state.CurrentMonthSpendUsd, cfg.MonthlyCapUsd, cfg.AlertThresholdPercent)
			_ = p.EventSend("app:budget:threshold", structs.EventSendOptions{Data: map[string]string{
				"actor":     "system",
				"app":       app,
				"spend_usd": strconv.FormatFloat(state.CurrentMonthSpendUsd, 'f', 2, 64),
				"cap_usd":   strconv.FormatFloat(cfg.MonthlyCapUsd, 'f', 2, 64),
				"pct":       strconv.FormatFloat(cfg.AlertThresholdPercent, 'f', 0, 64),
			}})
		}
		if willFireCap {
			fmt.Printf("ns=budget_accumulator at=alert kind=cap app=%s spend_usd=%.2f cap_usd=%.2f action=%s tripped=%t\n",
				app, state.CurrentMonthSpendUsd, cfg.MonthlyCapUsd, cfg.AtCapAction, state.CircuitBreakerTripped)
			_ = p.EventSend("app:budget:cap", structs.EventSendOptions{Data: map[string]string{
				"actor":     "system",
				"app":       app,
				"spend_usd": strconv.FormatFloat(state.CurrentMonthSpendUsd, 'f', 2, 64),
				"cap_usd":   strconv.FormatFloat(cfg.MonthlyCapUsd, 'f', 2, 64),
				"action":    cfg.AtCapAction,
			}})
		}

		// Set G: per-tick auto-shutdown reconciliation. Runs AFTER the
		// state annotation is persisted so :armed and :fired can read
		// fresh AlertFiredAtCap. Runs UNCONDITIONALLY of willFireCap so
		// the armed-window-elapsed transition fires on later ticks (the
		// :armed write happens on the willFireCap tick; :fired happens
		// notifyBeforeMinutes later).
		//
		// Skipped on the cost-tracking-only path (cfg == nil): no cap
		// configured means no :armed transition is possible, so there's
		// nothing to reconcile.
		if cfg != nil {
			p.reconcileAutoShutdown(ctx, app, cfg, state, now)
		}

		// Stale-annotation GC runs unconditionally per spec §7.4 — defaults
		// to a 10-minute interval cleanup window. Best-effort; logs but
		// does not abort on any annotation read/write failure.
		_ = p.runStaleAnnotationGC(ctx, app, budgetDefaultPollInterval)

		return nil
	}
	return fmt.Errorf("failed to write budget state for %s after %d retries", app, budgetWriteConflictRetries)
}

// Reserved buckets for per-service cost attribution. Build pods carry a
// `service` label naming the service being built, so without explicit
// bucketing their spend would inflate that service's normal-operation
// total. perServiceBucketBuild routes those pods to a separate row.
// Pods with no service label (system pods like KEDA scalers, anything
// non-user-deployed) bucket to perServiceBucketUnattributed so their
// spend stays visible without polluting service totals.
const (
	perServiceBucketBuild        = "_build"
	perServiceBucketUnattributed = "_unattributed"
)

// perServiceMaxEntries caps the size of state.PerServiceSpendUsd to bound
// annotation growth (Kubernetes annotation limit is 256 KB total per
// object). At ~70 bytes per entry, 1000 entries fit well under the limit
// while covering any realistic app's service count. It is a var (not a
// const) so the cap test in budget_breakdown_internal_test.go can exercise
// the truncation path without constructing 1000 fixture pods.
var perServiceMaxEntries = 1000

// computeBudgetDelta walks pods in the app namespace and attributes cost
// over elapsed = now - lastTick. Returns:
//   - delta_usd: the total tick spend, summed across all running pods
//   - perSvc:    per-service tick spend (keys = pod.Labels["service"]
//     with reserved buckets _build and _unattributed)
//   - perSvcInst: per-service instance type observed this tick
//     (first observation wins at the merge site)
//   - warnings:  count of pods skipped because of unknown instance type
//     or missing pricing entry
//
// ctx is accepted for future non-informer fallback paths; the current
// ListNodesFromInformer / ListPodsFromInformer reads are cache-only and do
// not take a ctx parameter. Plumbing it now keeps signatures stable when a
// future patch adds a direct-API fallback for cold-cache scenarios.
func (p *Provider) computeBudgetDelta(ctx context.Context, app string, lastTick, now time.Time, adjustment float64) (float64, map[string]float64, map[string]string, int, error) {
	_ = ctx // see godoc above; reserved for non-informer fallback.
	if lastTick.IsZero() {
		lastTick = now
	}
	elapsed := now.Sub(lastTick).Hours()
	if elapsed <= 0 {
		return 0, nil, nil, 0, nil
	}
	if adjustment <= 0 {
		adjustment = 1.0
	}

	nodes, err := p.ListNodesFromInformer("")
	if err != nil {
		return 0, nil, nil, 0, errors.WithStack(err)
	}
	nodeByName := map[string]*v1.Node{}
	for i := range nodes.Items {
		nodeByName[nodes.Items[i].Name] = &nodes.Items[i]
	}

	pods, err := p.ListPodsFromInformer(p.AppNamespace(app), "")
	if err != nil {
		return 0, nil, nil, 0, errors.WithStack(err)
	}

	var delta float64
	perSvc := map[string]float64{}
	perSvcInst := map[string]string{}
	warnings := 0

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != v1.PodRunning {
			continue
		}
		node, ok := nodeByName[pod.Spec.NodeName]
		if !ok {
			continue
		}
		instanceType := nodeInstanceType(node)
		if instanceType == "" {
			warnings++
			continue
		}
		price, ok := billing.PriceForInstance(instanceType)
		if !ok {
			warnings++
			continue
		}

		capacityType := nodeCapacityType(node)
		hourlyRate := price.EffectiveUsdPerHour(capacityType)

		fraction := dominantResourceFraction(pod, node, price)
		podTickSpend := hourlyRate * fraction * elapsed * adjustment
		delta += podTickSpend

		// Bucket selection. Build pods carry service-type=build PLUS a
		// service label naming what they're building; route them to
		// _build so the named service's normal-operation cost stays
		// uninflated. Anything else without a service label buckets
		// to _unattributed so the spend remains visible.
		var svc string
		switch {
		case pod.Labels["service-type"] == "build":
			svc = perServiceBucketBuild
		case pod.Labels["service"] != "":
			svc = pod.Labels["service"]
		default:
			svc = perServiceBucketUnattributed
		}

		perSvc[svc] += podTickSpend
		if _, ok := perSvcInst[svc]; !ok {
			perSvcInst[svc] = instanceType
		}
	}

	return delta, perSvc, perSvcInst, warnings, nil
}

func nodeInstanceType(n *v1.Node) string {
	if n == nil || n.Labels == nil {
		return ""
	}
	for _, k := range []string{"node.kubernetes.io/instance-type", "beta.kubernetes.io/instance-type"} {
		if v, ok := n.Labels[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

// nodeCapacityType returns "spot" or "on-demand" by checking the
// karpenter.sh/capacity-type label first (Karpenter-managed nodes) and
// then falling back to the eks.amazonaws.com/capacityType annotation
// (EKS ANG-managed nodes). Returns "" if neither signal is present so
// the caller falls through to on-demand pricing — conservative under
// uncertainty, charges the user the higher rate when the node
// origin is unknown.
//
// Both signals are AWS-specific. On GCP / Azure / on-prem the labels
// are absent and the helper returns "" — EffectiveUsdPerHour then
// returns OnDemandUsdPerHour unchanged. Non-AWS spot pricing is a
// future patch.
func nodeCapacityType(n *v1.Node) string {
	if n == nil {
		return ""
	}
	if n.Labels != nil {
		if v := strings.ToLower(n.Labels["karpenter.sh/capacity-type"]); v == "spot" || v == "on-demand" {
			return v
		}
	}
	if n.Annotations != nil {
		switch strings.ToLower(n.Annotations["eks.amazonaws.com/capacityType"]) {
		case "spot":
			return "spot"
		case "on_demand", "on-demand":
			return "on-demand"
		}
	}
	return ""
}

// dominantResourceFraction returns the pod's share of the node, as the max
// across (gpu, cpu, mem) of requested/allocatable. GPU-allocated attribution
// takes precedence when the pod requests GPUs, since the instance price is
// almost entirely the GPU cost.
//
// Resource aggregation follows the standard Kubernetes cost-attribution
// formula: for each resource dimension the pod's reservation is
// max(sum-over-regular-containers, max-over-init-containers). Init
// containers run before regular containers but hold the same request
// ceiling, so this captures whichever is larger.
//
// All returned fractions are clamped to [0, 1] — a pod can never cost more
// than the instance it runs on, even under misconfigured time-sliced GPUs
// that advertise reqGpu > GpuCount.
func dominantResourceFraction(pod *v1.Pod, node *v1.Node, price billing.InstancePrice) float64 {
	allocCpu := node.Status.Allocatable.Cpu()
	allocMem := node.Status.Allocatable.Memory()

	reqCpu, reqMem, reqGpu := podResourceReservation(pod)

	if price.GpuCount > 0 && reqGpu > 0 {
		frac := float64(reqGpu) / float64(price.GpuCount)
		if frac > 1 {
			frac = 1
		}
		return frac
	}

	cpuFrac := 0.0
	if allocCpu != nil && allocCpu.MilliValue() > 0 {
		cpuFrac = float64(reqCpu) / float64(allocCpu.MilliValue())
	}
	memFrac := 0.0
	if allocMem != nil && allocMem.Value() > 0 {
		memFrac = float64(reqMem) / float64(allocMem.Value())
	}

	maxFrac := cpuFrac
	if memFrac > maxFrac {
		maxFrac = memFrac
	}
	if maxFrac > 1 {
		maxFrac = 1
	}
	return maxFrac
}

// podResourceReservation implements max(sum-regular, max-init) per dimension.
// GPU keys are restricted to the canonical set tracked by gpuKeyToVendor to
// avoid spuriously summing extended-resource keys whose names happen to
// contain "gpu".
func podResourceReservation(pod *v1.Pod) (cpuMilli, memBytes, gpu int64) {
	var regCpu, regMem, regGpu int64
	for i := range pod.Spec.Containers {
		c := &pod.Spec.Containers[i]
		if q, ok := c.Resources.Requests[v1.ResourceCPU]; ok {
			regCpu += q.MilliValue()
		}
		if q, ok := c.Resources.Requests[v1.ResourceMemory]; ok {
			regMem += q.Value()
		}
		for key := range gpuKeyToVendor {
			if q, ok := c.Resources.Requests[v1.ResourceName(key)]; ok {
				regGpu += q.Value()
			}
		}
	}

	var maxInitCpu, maxInitMem, maxInitGpu int64
	for i := range pod.Spec.InitContainers {
		c := &pod.Spec.InitContainers[i]
		if q, ok := c.Resources.Requests[v1.ResourceCPU]; ok {
			if v := q.MilliValue(); v > maxInitCpu {
				maxInitCpu = v
			}
		}
		if q, ok := c.Resources.Requests[v1.ResourceMemory]; ok {
			if v := q.Value(); v > maxInitMem {
				maxInitMem = v
			}
		}
		for key := range gpuKeyToVendor {
			if q, ok := c.Resources.Requests[v1.ResourceName(key)]; ok {
				if v := q.Value(); v > maxInitGpu {
					maxInitGpu = v
				}
			}
		}
	}

	cpuMilli = maxInt64(regCpu, maxInitCpu)
	memBytes = maxInt64(regMem, maxInitMem)
	gpu = maxInt64(regGpu, maxInitGpu)
	return
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// budgetCircuitBreakerTripped is the enforcement pre-flight used by
// ReleasePromote, ServiceUpdate, and ProcessRun. Returns ErrConflict with
// guidance text when the breaker is tripped; nil otherwise.
//
// Gated on cost_tracking_enable: when cost tracking is OFF (rack param
// cost_tracking_enable=false, or env var unset), the breaker reader returns
// nil unconditionally. This treats any persisted CircuitBreakerTripped
// annotation as inert when the accumulator that would otherwise reset it is
// not running. Without this gate, a user who disables cost tracking
// while a tripped breaker annotation is persisted on the namespace would be
// permanently blocked from deploying with no recovery path.
func (p *Provider) budgetCircuitBreakerTripped(app string) error {
	if !p.costTrackingEnabled() {
		return nil
	}
	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		if ae.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	state, _ := readBudgetStateAnnotation(ns.Annotations)
	if state == nil || !state.CircuitBreakerTripped {
		return nil
	}
	cfg, _ := readBudgetConfigAnnotation(ns.Annotations)
	capUsd := 0.0
	if cfg != nil {
		capUsd = cfg.MonthlyCapUsd
	}
	// When the breaker is tripped the user
	// has THREE recovery paths, not one. Spell them all out so the user
	// does not assume `budget reset` is the only option (a `cap raise` is
	// often what they actually want when traffic legitimately grew).
	// `auto-shutdown` carries the same 3-action message — services already
	// scaled to 0 will restore via the same `budget reset` path that
	// re-arms the breaker. F-25 fix: dropped the AtCapAction read because
	// the 409 text is uniform; the variable was previously assigned only
	// to satisfy "declared but not used" via `_ = atCapAction`.
	// F-2 fix: include the app name in the `cap raise` hint so the
	// user can paste the recommendation verbatim. The ARMED banner
	// at pkg/cli/budget.go:180 already cites the same command with
	// `<app>` populated; this keeps the two surfaces consistent.
	return structs.ErrConflict(
		"budget cap exceeded for app %s: spent $%.2f of $%.2f cap this month; "+
			"recovery options: (1) `convox budget reset %s` to acknowledge and re-enable deploys, "+
			"(2) `convox budget cap raise --monthly-cap-usd <higher> %s` to raise the cap (alias for `budget set --monthly-cap`), "+
			"or (3) wait for month rollover (caps reset automatically on the 1st)",
		app, state.CurrentMonthSpendUsd, capUsd, app, app,
	)
}

// costTrackingEnabled reports whether the rack is configured with
// cost_tracking_enable=true. The Terraform module injects
// COST_TRACKING_ENABLE on the api pod from the rack param; runtime reads
// the env var. This is the single canonical accessor used by the
// accumulator dispatch (k8s.go) and the breaker reader gate. Future
// stuck-state and enforcement gates should reuse it rather than inlining
// os.Getenv calls.
func (p *Provider) costTrackingEnabled() bool {
	return os.Getenv("COST_TRACKING_ENABLE") == "true"
}

// requireCostTrackingForBudget rejects enforcement-bearing budget options
// (caps, alerts, at-cap actions) when the rack-level cost accumulator is
// disabled. Without cost_tracking_enable=true the accumulator goroutine
// never runs, so saving budget config produces zero enforcement: no
// spend computed, no threshold crossings, no events, no auto-shutdown.
// Replacing that silent no-op with a loud, actionable error preserves
// the cap-enforcement contract — a user-visible knob must either
// take effect or fail loudly.
//
// Recovery operations (AppBudgetClear, AppBudgetReset) are NOT gated;
// users must always be able to clear or reset state. PricingAdjustment
// alone is also not gated — it is a pricing-model multiplier, not an
// enforcement field.
func (p *Provider) requireCostTrackingForBudget(opts structs.AppBudgetOptions) error {
	if opts.MonthlyCapUsd == nil && opts.AlertThresholdPercent == nil && opts.AtCapAction == nil {
		return nil
	}
	if !p.costTrackingEnabled() {
		return errors.WithStack(structs.ErrUnprocessable(
			"rack parameter cost_tracking_enable is false. Budget enforcement (caps, alerts, auto-shutdown) requires cost_tracking_enable=true.\n" +
				"  Set on the rack first:\n" +
				"    convox rack params set cost_tracking_enable=true\n" +
				"  Wait ~3 min for the apply to complete, then retry.",
		))
	}
	return nil
}
