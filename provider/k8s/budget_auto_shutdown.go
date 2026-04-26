package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcileAutoShutdown runs the auto-shutdown lifecycle for a single
// app on the current accumulator tick. Called from accumulateBudgetApp
// AFTER the main spend / breaker / alert update has been written.
//
// Lifecycle mapping (per Set G v2 spec §6 + §8):
//
//	:armed             — cap breach with auto-shutdown configured + no prior annotation
//	:fired             — armed-window elapsed; PATCH services to 0 + persist shutdownAt
//	:expired           — month rollover with manual recovery + no customer reset
//	:flap-suppressed   — re-trip within 24h cooldown carry-over
//	:noop              — eligibility check returns 0 services
//	:restored (manual) — customer manually scaled all eligible services back up
//	:cancelled (cfg)   — manifestSha256 mismatch in armed window
//	:restored (cfg)    — manifestSha256 mismatch post-shutdown
//
// Best-effort: every annotation read/write that fails is logged but does
// not abort the tick. The accumulator is the only path that fires :armed
// / :fired / :expired / :flap-suppressed / :noop — other callers (CLI
// reset, simulate, dismiss-recovery) own the remaining 4 events.
func (p *Provider) reconcileAutoShutdown(ctx context.Context, app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, now time.Time) {
	if cfg == nil || cfg.AtCapAction != structs.BudgetAtCapActionAutoShutdown {
		return
	}
	if !p.costTrackingEnabled() {
		return
	}

	// F-19 fix (catalog D-7): hold the per-app advisory lock for the
	// duration of the reconcile tick so a concurrent AppBudgetReset
	// cannot interleave its `:cancelled` emit with this tick's emit.
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		fmt.Printf("ns=auto_shutdown at=ns_get_failed app=%s err=%q\n", app, err)
		return
	}

	shutdownState, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
	if parseErr != nil {
		fmt.Printf("ns=auto_shutdown at=state_corrupt app=%s err=%q\n", app, parseErr)
		// F10 fix: dedup state-corrupt :failed via SEPARATE annotation
		// since the main state annotation is unparseable. Skip emit if
		// dedup annotation already present and within tick window.
		if !p.stateCorruptDedupExpired(ns.Annotations, now) {
			return
		}
		// R8.5 F-1 fix: persist dedup annotation BEFORE emit (matches the
		// 9-of-9 lifecycle persist-then-emit symmetry locked at R7.5).
		// Without this gate, a silent annotation-write failure leaves the
		// dedup unset and the next tick re-emits :failed reason=state-corrupt
		// — duplicate event on the bus. F10 dedup window (~10 min) bounds
		// the practical risk but doesn't eliminate it.
		if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
			p.fireFailedEventStateCorrupt(app, cfg, baseState, now)
		}
		return
	}
	// State now parses cleanly; clear any stale state-corrupt dedup marker.
	if _, ok := ns.Annotations[structs.BudgetShutdownStateCorruptFiredAtAnnotation]; ok {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation)
	}

	// F8 fix: if customer scaled services back up DURING armed window
	// (no PATCH yet applied) — fire :cancelled reason="manual-detected"
	// and GC the orphan annotation so next cap re-breach re-arms cleanly.
	// R7.5 F-3 fix: GC annotation BEFORE emit (matches F-20 dedup-write-
	// then-emit pattern). The annotation deletion IS the dedup signal
	// here: with the annotation gone, next tick's armedWindowManuallyScaledUp
	// returns nil (no shutdownState) and the manual-detected branch is
	// skipped. If the delete fails, abort the emit so the next tick can
	// retry cleanly. NotFound is treated as success (already GC'd).
	if shutdownState != nil && shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
		(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
		if p.armedWindowManuallyScaledUp(ctx, app, shutdownState.Services) {
			derr := p.deleteBudgetShutdownStateAnnotation(ctx, app)
			if derr == nil || ae.IsNotFound(derr) {
				p.fireCancelledEventRich(app, cfg, baseState, shutdownState, "system", "manual-detected", 0, 0, "", now)
			}
			return
		}
	}

	if handled := p.reconcileAutoShutdownPreManifest(ctx, app, cfg, shutdownState, baseState, now); handled {
		return
	}

	// (3) Cap not breached → nothing else to do (no fresh :armed)
	if baseState == nil || baseState.AlertFiredAtCap.IsZero() {
		return
	}

	// (4) External-edit detection (manifestSha256 mismatch in any annotation state)
	m, _, mErr := p.releaseManifestForApp(app)
	if mErr != nil {
		// No manifest yet → cannot compute eligibility / SHA. Nothing to do.
		return
	}
	p.reconcileAutoShutdownWithManifest(ctx, app, cfg, baseState, ns, shutdownState, m, now)
}

// reconcileAutoShutdownPreManifest runs the lifecycle branches that do not
// require a release manifest — :expired (month rollover with manual mode)
// and :restored reason="manual-detected" (customer manually scaled
// everything back up). Returns true when one of these branches handled
// the tick (caller should not proceed to the manifest-bearing branches).
// Split out so end-to-end tests can drive these branches without
// mocking AppGet/ReleaseGet/Atom.
func (p *Provider) reconcileAutoShutdownPreManifest(ctx context.Context, app string, cfg *structs.AppBudget, shutdownState *structs.AppBudgetShutdownState, baseState *structs.AppBudgetState, now time.Time) bool {
	// (1) Month rollover :expired (only relevant when shutdown previously fired)
	if shutdownState != nil && shutdownState.ShutdownAt != nil && !shutdownState.ShutdownAt.IsZero() &&
		shutdownState.ExpiredAt == nil && shutdownState.RestoredAt == nil &&
		shutdownState.RecoveryMode == "manual" {
		if startOfMonth(*shutdownState.ShutdownAt).Before(startOfMonth(now)) {
			shutdownState.ExpiredAt = ptrTimePackage(now)
			if shutdownState.ExpiredNotificationFiredAt == nil {
				// MF-7 (F-20 extension): persist BEFORE emit so a silent
				// persist failure aborts the wire emission. Without this,
				// next tick reads ExpiredNotificationFiredAt==nil and
				// re-fires :expired — visible duplicate on the bus.
				shutdownState.ExpiredNotificationFiredAt = ptrTimePackage(now)
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					p.fireExpiredEvent(app, cfg, baseState, shutdownState, now)
				}
			} else {
				_ = p.persistShutdownState(ctx, app, shutdownState)
			}
			return true
		}
	}

	// (2) Manual-recovery detection (post-fire, pre-reset)
	if shutdownState != nil && shutdownState.ShutdownAt != nil && !shutdownState.ShutdownAt.IsZero() &&
		shutdownState.RestoredAt == nil {
		manualRecovered := p.allServicesScaledUp(ctx, app, shutdownState.Services)
		if manualRecovered {
			shutdownState.RestoredAt = ptrTimePackage(now)
			flap := now.Add(structs.BudgetFlapCooldown)
			shutdownState.FlapSuppressedUntil = ptrTimePackage(flap)
			if shutdownState.RestoredNotificationFiredAt == nil {
				// F13b fix (extended to :restored): persist BEFORE emit
				// so a leader crash between the two doesn't double-fire.
				// F-20 fix: abort emit if persist fails — without the
				// dedup write landing, the next tick re-fires :restored.
				shutdownState.RestoredNotificationFiredAt = ptrTimePackage(now)
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					p.fireRestoredEvent(app, cfg, baseState, shutdownState, "manual-detected", now)
				}
			} else {
				_ = p.persistShutdownState(ctx, app, shutdownState)
			}
			_ = p.writeFlapSuppressedUntilAnnotation(ctx, app, flap)
			return true
		}
	}
	return false
}

// reconcileAutoShutdownWithManifest is the post-manifest-load tail of
// reconcileAutoShutdown — split out so the lifecycle can be exercised
// end-to-end by tests that pre-build a manifest (avoiding the
// AppGet/ReleaseGet/Atom mocking surface). Production code path:
// reconcileAutoShutdown loads the manifest and forwards. Test hook:
// ReconcileAutoShutdownWithManifestForTest in export_test.go injects a
// pre-built manifest directly. Call ordering matches the spec §6/§8
// branches preserved from before the refactor.
func (p *Provider) reconcileAutoShutdownWithManifest(ctx context.Context, app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, ns *ac.Namespace, shutdownState *structs.AppBudgetShutdownState, m *manifest.Manifest, now time.Time) {
	plan, pErr := p.computeShutdownPlanForApp(ctx, app, m, cfg)
	if pErr != nil {
		fmt.Printf("ns=auto_shutdown at=plan_failed app=%s err=%q\n", app, pErr)
		return
	}

	if shutdownState != nil && shutdownState.ManifestSha256 != "" && shutdownState.ManifestSha256 != plan.manifestSha {
		// Config drift detected.
		armed := shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
			(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero())
		if armed {
			// F2 fix: classify as "cap-raised" if the new cap is HIGHER
			// such that spend now sits below it; otherwise treat the
			// manifest SHA mismatch as a generic "config-changed". Surface
			// prev_cap_usd / new_cap_usd for the cap-raised sub-case.
			reason := "config-changed"
			var prevCap, newCap float64
			if cfg != nil {
				newCap = cfg.MonthlyCapUsd
			}
			// best-effort prev_cap recovery: spend at-or-above prev cap
			// (otherwise breaker wouldn't have armed), so surface spend
			// as the floor estimate — better than 0.
			if cfg != nil && baseState != nil &&
				cfg.MonthlyCapUsd > baseState.CurrentMonthSpendUsd &&
				baseState.CurrentMonthSpendUsd > 0 {
				reason = "cap-raised"
				prevCap = baseState.CurrentMonthSpendUsd
			}
			if shutdownState.CancelledNotificationFiredAt == nil {
				// F13b fix (extended to :cancelled): persist BEFORE emit.
				// F-20 fix: abort emit if persist fails so the dedup write
				// is observable on the next tick.
				shutdownState.CancelledNotificationFiredAt = ptrTimePackage(now)
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					newAction := ""
					if reason == "config-changed" && cfg != nil {
						newAction = cfg.AtCapAction
					}
					// MF-6 fix: spec §8.4 line 777 mandates JWT-derived
					// actor for cap-raised. AppBudgetSet records the JWT
					// user in cfg.LastCapMutationBy on every cap mutation;
					// fall back to "system" if empty (older rack / state
					// predates 3.24.6). config-changed legitimately stays
					// "system" because manifest-mismatch detection has no
					// originating HTTP request.
					actor := "system"
					if reason == "cap-raised" && cfg != nil && cfg.LastCapMutationBy != "" {
						actor = cfg.LastCapMutationBy
					}
					p.fireCancelledEventRich(app, cfg, baseState, shutdownState, actor, reason, prevCap, newCap, newAction, now)
				}
			}
			// Drop the armed annotation since config no longer matches.
			_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
			return
		}
		// post-shutdown :restored reason=config-changed (customer-side cleanup)
		if shutdownState.RestoredAt == nil {
			shutdownState.RestoredAt = ptrTimePackage(now)
			if shutdownState.RestoredNotificationFiredAt == nil {
				// F13b fix (extended to :restored): persist BEFORE emit.
				// F-20 fix: abort emit if persist fails.
				shutdownState.RestoredNotificationFiredAt = ptrTimePackage(now)
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					p.fireRestoredEvent(app, cfg, baseState, shutdownState, "config-changed", now)
				}
			} else {
				_ = p.persistShutdownState(ctx, app, shutdownState)
			}
			return
		}
	}

	// (5) flap-suppressed: cap breached + cooldown carry-over active
	flap, _ := readFlapSuppressedUntilAnnotation(ns.Annotations)
	if flap != nil && flap.After(now) {
		// Suppress new arm; fire :flap-suppressed once via dedup annotation.
		// R7.5 F-3 fix: persist dedup annotation BEFORE emit (matches F-20
		// pattern). Without this, a silent annotation-write failure leaves
		// the dedup unset and the next tick re-fires :flap-suppressed —
		// duplicate event on the bus. Order: write annotation, only emit
		// on success.
		if _, fired := ns.Annotations[structs.BudgetFlapSuppressFiredAtAnnotation]; !fired {
			if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
				p.fireFlapSuppressedEvent(app, cfg, baseState, *flap, now)
			}
		}
		return
	}

	// (6) External-edit detection (spec §13.3): shutdownState==nil but the
	// app already has eligible services scaled to 0 — operator hand-recovery,
	// CD pipeline strip, or policy controller cleared the annotation mid-
	// shutdown. Treat as "discovered shutdown via external edit" and fire
	// :noop reason="external-edit-detected" instead of re-arming (which would
	// re-trip on the same outage).
	if shutdownState == nil && len(plan.ordered) > 0 {
		allZero := true
		for _, sp := range plan.ordered {
			if sp.Replicas > 0 {
				allZero = false
				break
			}
		}
		if allZero {
			if p.noopDedupExpired(ns.Annotations, now) {
				// MF-7 (F-20 extension): write dedup annotation BEFORE emit
				// so a silent annotation-write failure aborts the wire
				// emission. The :noop dedup window depends on the
				// BudgetShutdownNoopFiredAtAnnotation timestamp; without the
				// write landing, next tick re-fires :noop on every reconcile
				// loop until the annotation succeeds (visible duplicates).
				if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
					p.fireNoopEvent(app, cfg, baseState, "external-edit-detected", plan, now)
				}
			}
			return
		}
	}

	// (7) Empty eligibility → :noop (only when no annotation present).
	// 3 distinct reasons per spec §8.9:
	//   - "no-eligible-services" — manifest has 0 services OR all filtered
	//     for static-config reasons (in neverAutoShutdown / agent DaemonSet)
	//   - "runtime-drift"        — services exist in manifest but cluster
	//     state diverged (e.g., deployments not yet created on first deploy)
	//   - "external-edit-detected" — handled above at branch (6); falls
	//     through here only when plan.ordered is empty AND none of the
	//     filter reasons match runtime-drift
	if shutdownState == nil {
		if len(plan.ordered) == 0 {
			reason := classifyNoopReason(m, plan)
			// F9 fix: dedup :noop via dedicated annotation since
			// shutdownState==nil cannot carry the dedup tracker field.
			if p.noopDedupExpired(ns.Annotations, now) {
				// MF-7 (F-20 extension): write dedup annotation BEFORE emit
				// so a silent annotation-write failure aborts the wire
				// emission and avoids re-firing :noop on every tick.
				if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
					p.fireNoopEvent(app, cfg, baseState, reason, plan, now)
				}
			}
			return
		}
		// (8) Arm: write state annotation + fire :armed
		// Clear any stale noop dedup marker — we're about to arm so the
		// cap-fired→noop dedup window no longer applies on next tick.
		if _, ok := ns.Annotations[structs.BudgetShutdownNoopFiredAtAnnotation]; ok {
			_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation)
		}
		armedNow := now
		shutdownTickID := generateShutdownTickID(armedNow)
		// F-18 fix: persist NotifyBeforeMinutes from the manifest so the
		// CLI banner + STATUS countdown reflect the customer-configured
		// value rather than the 30-minute default. Falls back to default
		// when plan.notifyBeforeMinutes <= 0.
		notifyMin := plan.notifyBeforeMinutes
		if notifyMin <= 0 {
			notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
		}
		newState := &structs.AppBudgetShutdownState{
			SchemaVersion:            structs.BudgetShutdownStateSchemaVersion,
			ArmedAt:                  &armedNow,
			NotifyBeforeMinutes:      notifyMin,
			RecoveryMode:             plan.recoveryMode,
			ShutdownOrder:            plan.shutdownOrder,
			ShutdownTickId:           shutdownTickID,
			ManifestSha256:           plan.manifestSha,
			EligibleServiceCount:     len(plan.ordered),
			ArmedNotificationFiredAt: &armedNow,
		}
		// Snapshot service plans into annotation so :fired knows what to scale.
		newState.Services = make([]structs.AppBudgetShutdownStateService, 0, len(plan.ordered))
		for i, sp := range plan.ordered {
			newState.Services = append(newState.Services, structs.AppBudgetShutdownStateService{
				Name: sp.Service,
				OriginalScale: structs.AppBudgetShutdownStateOriginalScale{
					Count:    int(sp.Replicas),
					Replicas: int(sp.Replicas),
				},
				OriginalGracePeriodSeconds: sp.GraceSecs,
				ShutdownSequenceIndex:      i,
				KedaScaledObject:           kedaScaledObjectFromPlan(sp),
			})
		}
		// F-20 fix: gate :armed emit on persist success. Without this,
		// a silent persist failure means next tick has shutdownState==nil
		// and re-arms (re-fires :armed) — visible duplicate on the wire.
		if perr := p.persistShutdownState(ctx, app, newState); perr == nil {
			p.fireArmedEvent(app, cfg, baseState, newState, plan, now)
		}
		return
	}

	// (9) Already armed: check if firing window elapsed
	if shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
		(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
		notifyMin := plan.notifyBeforeMinutes
		if notifyMin <= 0 {
			notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
		}
		fireAt := shutdownState.ArmedAt.Add(time.Duration(notifyMin) * time.Minute)
		if !now.Before(fireAt) {
			// :fired — execute shutdown
			grace := plan.shutdownGracePeriod
			if grace <= 0 {
				if d, perr := time.ParseDuration(structs.BudgetDefaultShutdownGracePeriod); perr == nil {
					grace = d
				}
			}
			gracePeriodSeconds := int64(grace.Seconds())
			shutNow := now
			succeeded := []string{}
			failed := []string{}
			// F-26 fix: capture the most recent shutdownService error so
			// the FAILED branch can classify the canonical reason instead
			// of always reporting "k8s-api-failure". The error wraps the
			// underlying K8s API error preserved by errors.Wrapf in
			// shutdownService and the patch-retry helpers.
			var lastShutdownErr error
			for i := range shutdownState.Services {
				svc := &shutdownState.Services[i]
				if err := p.shutdownService(ctx, app, svc.Name, gracePeriodSeconds); err != nil {
					failed = append(failed, svc.Name)
					lastShutdownErr = err
					fmt.Printf("ns=auto_shutdown at=fire_failed app=%s service=%s err=%q\n", app, svc.Name, err)
					continue
				}
				succeeded = append(succeeded, svc.Name)
				svc.ShutdownAt = &shutNow
			}
			shutdownState.ShutdownAt = &shutNow

			// F3 fix + F13b fix: :fired and :failed are MUTUALLY
			// EXCLUSIVE per spec §8.10. If ANY service failed, emit
			// :failed (with partial_state=succeeded count) and DO NOT
			// emit :fired. Persist dedup BEFORE emit so a leader crash
			// between persist and emit doesn't double-fire on retry.
			if len(failed) > 0 {
				if shutdownState.FailedNotificationFiredAt == nil {
					// Persist FailureReason BEFORE firing the event so the
					// FAILED banner rendered by `convox budget show` reads
					// the canonical reason from the state annotation
					// (per Set G v2 spec §16.3 — γ-7 BLOCKER B3 fix).
					// F-20 fix: abort fireFailedEvent emit if persist
					// fails so the dedup write is observable on the next
					// tick — otherwise next reconcile re-fires :failed.
					// F-26 fix: classify the underlying K8s API error via
					// classifyPatchError so the FailureReason reflects the
					// canonical sub-case (admission-rejected,
					// annotation-rejected, schema-incompatible) rather
					// than the generic k8s-api-failure fallback.
					reason := classifyPatchError(lastShutdownErr, false)
					if reason == "" {
						reason = structs.BudgetShutdownReasonK8sApiFailure
					}
					shutdownState.FailureReason = reason
					shutdownState.FailedNotificationFiredAt = &shutNow
					if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
						p.fireFailedEvent(app, cfg, baseState, "system", shutdownState.ShutdownTickId, now, failed, reason, len(succeeded))
					}
				} else {
					_ = p.persistShutdownState(ctx, app, shutdownState)
				}
				return
			}

			if shutdownState.FiredNotificationFiredAt == nil {
				// F13b fix: persist BEFORE emit (was: emit then persist).
				// On leader crash between persist and emit the new leader
				// sees FiredNotificationFiredAt set and skips re-emit;
				// without F13b a crash window let the new leader re-fire.
				// F-20 fix: abort fireFiredEvent emit if persist fails so
				// the dedup write is observable on the next tick.
				shutdownState.FiredNotificationFiredAt = &shutNow
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					p.fireFiredEvent(app, cfg, baseState, shutdownState, succeeded, plan, now)
				}
			} else {
				_ = p.persistShutdownState(ctx, app, shutdownState)
			}
			return
		}
	}
}

// allServicesScaledUp returns true when every saved service shows
// Replicas > 0 in the cluster (customer manually restored). Used by the
// manual-recovery detection path. Best-effort: a missing deployment counts
// as "scaled up" because there's nothing for us to restore.
func (p *Provider) allServicesScaledUp(ctx context.Context, app string, svcs []structs.AppBudgetShutdownStateService) bool {
	if len(svcs) == 0 {
		return false
	}
	nsName := p.AppNamespace(app)
	for i := range svcs {
		dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svcs[i].Name, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				continue
			}
			return false
		}
		if dep.Spec.Replicas == nil || *dep.Spec.Replicas == 0 {
			return false
		}
	}
	return true
}

// armedWindowManuallyScaledUp returns true when the customer has scaled
// services back up DURING the armed window beyond the original snapshot
// (before any PATCH to 0 has applied). Distinct from allServicesScaledUp
// which checks post-fired recovery. F8 fix.
func (p *Provider) armedWindowManuallyScaledUp(ctx context.Context, app string, svcs []structs.AppBudgetShutdownStateService) bool {
	if len(svcs) == 0 {
		return false
	}
	nsName := p.AppNamespace(app)
	for i := range svcs {
		dep, err := p.Cluster.AppsV1().Deployments(nsName).Get(ctx, svcs[i].Name, am.GetOptions{})
		if err != nil {
			if ae.IsNotFound(err) {
				continue
			}
			return false
		}
		if dep.Spec.Replicas == nil {
			return false
		}
		// Customer scaled UP from the snapshot value — explicit intervention.
		if int(*dep.Spec.Replicas) > svcs[i].OriginalScale.Replicas {
			return true
		}
	}
	return false
}

// noopDedupExpired returns true when the noop dedup window has elapsed
// (or there's no prior noop fired-at marker). The window matches the
// default tick interval — one :noop emission per tick at most. Per F9
// fix and spec §7.2 dedup tracker semantics.
func (p *Provider) noopDedupExpired(ann map[string]string, now time.Time) bool {
	raw, ok := ann[structs.BudgetShutdownNoopFiredAtAnnotation]
	if !ok || raw == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return true
	}
	// One emission per default tick interval (10 min). Allows a late
	// re-trip (after recovery) to re-fire on a fresh window.
	return now.Sub(t) > budgetDefaultPollInterval-time.Second
}

// stateCorruptDedupExpired returns true when the state-corrupt dedup
// window has elapsed (or there's no prior state-corrupt fired-at marker).
// Per F10 fix: the marker is written to a SEPARATE annotation key (NOT
// inside the corrupt state JSON) so it survives the unparseable state
// annotation.
func (p *Provider) stateCorruptDedupExpired(ann map[string]string, now time.Time) bool {
	raw, ok := ann[structs.BudgetShutdownStateCorruptFiredAtAnnotation]
	if !ok || raw == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return true
	}
	return now.Sub(t) > budgetDefaultPollInterval-time.Second
}

// kedaScaledObjectFromPlan returns the KEDA snapshot for a shutdownPlan,
// or nil when the service has no ScaledObject. The accumulator path
// records HasKeda but not the resolved name; the saved name equals the
// service name for the v1 KEDA naming convention.
func kedaScaledObjectFromPlan(sp shutdownPlan) *structs.AppBudgetShutdownStateKeda {
	if !sp.HasKeda {
		return nil
	}
	return &structs.AppBudgetShutdownStateKeda{
		Name:                        sp.Service,
		PausedReplicasAnnotationSet: false, // set during :fired's PATCH
	}
}

// persistShutdownState writes (or rewrites) the shutdown-state annotation
// using the resourceVersion-based path so concurrent reset+tick races
// resolve cleanly. Logs but does not abort on conflict — the next tick
// will retry.
//
// F-20 fix: returns an error so callers gating event emission on dedup
// trackers (FiredNotificationFiredAt, FailedNotificationFiredAt,
// CancelledNotificationFiredAt, RestoredNotificationFiredAt) can abort
// emit when persist failed. Without this, a silent persist failure left
// the dedup tracker in-memory only — next accumulator tick re-fires the
// event because the persisted annotation never recorded the dedup write.
// Callers that do not gate on dedup intentionally swallow the error with
// `_ = p.persistShutdownState(...)`.
func (p *Provider) persistShutdownState(ctx context.Context, app string, s *structs.AppBudgetShutdownState) error {
	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, p.AppNamespace(app), am.GetOptions{})
		if err != nil {
			fmt.Printf("ns=auto_shutdown at=persist_get_failed app=%s err=%q\n", app, err)
			return err
		}
		if err := p.writeBudgetShutdownStateAnnotation(ctx, app, s, ns.ResourceVersion); err != nil {
			if ae.IsConflict(errors.Cause(err)) {
				continue
			}
			fmt.Printf("ns=auto_shutdown at=persist_failed app=%s err=%q\n", app, err)
			return err
		}
		return nil
	}
	return fmt.Errorf("persistShutdownState: exhausted %d write-conflict retries for app %s", budgetWriteConflictRetries, app)
}

// capUsdFor returns the current monthly cap from the budget config, or
// 0 when cfg is nil. Used as the universal payload's `cap_usd` value.
func capUsdFor(cfg *structs.AppBudget) float64 {
	if cfg == nil {
		return 0
	}
	return cfg.MonthlyCapUsd
}

// spendUsdFor returns the current month-to-date spend from the base
// budget state, or 0 when state is nil. Used as the universal payload's
// `spend_usd` value.
func spendUsdFor(baseState *structs.AppBudgetState) float64 {
	if baseState == nil {
		return 0
	}
	return baseState.CurrentMonthSpendUsd
}

// fireArmedEvent emits :armed with the universal payload + per-event fields per spec §8.1.
func (p *Provider) fireArmedEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, plan *shutdownPlanResult, now time.Time) {
	// F4 fix: populate cap_usd from cfg, spend_usd from baseState.
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	// F6 fix: rename armed_at → scheduled_at; fire_at → expected_shutdown_at per spec §8.1.
	data["scheduled_at"] = now.Format(time.RFC3339)
	notifyMin := plan.notifyBeforeMinutes
	if notifyMin <= 0 {
		notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
	}
	data["expected_shutdown_at"] = now.Add(time.Duration(notifyMin) * time.Minute).Format(time.RFC3339)
	data["notify_before_minutes"] = strconv.Itoa(notifyMin)
	data["eligible_service_count"] = strconv.Itoa(state.EligibleServiceCount)
	names := make([]string, 0, len(state.Services))
	for _, s := range state.Services {
		names = append(names, s.Name)
	}
	data["eligible_services"] = formatServiceList(names)
	data["shutdown_order"] = plan.shutdownOrder
	data["recovery_mode"] = plan.recoveryMode
	// F7 fix (extended to :armed per spec §8.1 lines 702-703): webhook_url
	// + over_cap_usd.
	// F-1 fix: redact webhook_url to scheme+host (e.g. https://hooks.slack.com)
	// so receivers parsing the field with new URL(...) get a valid RFC 3986
	// URL without embedded tokens. Helper redactedWebhookURL lives in event.go;
	// distinct from redactURLHost (bare host) used for log lines.
	if plan.webhookUrl != "" {
		data["webhook_url"] = redactedWebhookURL(plan.webhookUrl)
	}
	overCap := spendUsdFor(baseState) - capUsdFor(cfg)
	if overCap < 0 {
		overCap = 0
	}
	data["over_cap_usd"] = strconv.FormatFloat(overCap, 'f', 2, 64)
	_ = p.EventSend(shutdownEventName("armed"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=armed app=%s tick_id=%s eligible=%d notify_min=%d\n",
		app, state.ShutdownTickId, state.EligibleServiceCount, notifyMin)
}

// fireFiredEvent emits :fired with the universal payload + per-event fields per spec §8.2.
func (p *Provider) fireFiredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, succeeded []string, plan *shutdownPlanResult, now time.Time) {
	// F4 fix: populate cap_usd from cfg, spend_usd from baseState.
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	// F7 fix: rename fired_at → shutdown_at per spec §8.2 line 729.
	data["shutdown_at"] = now.Format(time.RFC3339)
	data["shut_down_services"] = formatServiceList(succeeded)
	data["shut_down_count"] = strconv.Itoa(len(succeeded))
	data["shutdown_order"] = plan.shutdownOrder
	// F7 fix: snapshot_annotation (full state JSON), recovery_command,
	// keda_managed_count, deployment_only_count, webhook_url, over_cap_usd.
	if snap, err := json.Marshal(state); err == nil {
		data["snapshot_annotation"] = string(snap)
	}
	data["recovery_command"] = fmt.Sprintf("convox budget reset %s", app)
	keda := 0
	depOnly := 0
	for _, svc := range state.Services {
		if svc.KedaScaledObject != nil {
			keda++
		} else {
			depOnly++
		}
	}
	data["keda_managed_count"] = strconv.Itoa(keda)
	data["deployment_only_count"] = strconv.Itoa(depOnly)
	// F-1 fix: scheme+host redaction (see :armed site for rationale).
	if plan.webhookUrl != "" {
		data["webhook_url"] = redactedWebhookURL(plan.webhookUrl)
	}
	overCap := spendUsdFor(baseState) - capUsdFor(cfg)
	if overCap < 0 {
		overCap = 0
	}
	data["over_cap_usd"] = strconv.FormatFloat(overCap, 'f', 2, 64)
	_ = p.EventSend(shutdownEventName("fired"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=fired app=%s tick_id=%s succeeded=%d\n",
		app, state.ShutdownTickId, len(succeeded))
}

// fireExpiredEvent emits :expired per spec §8.5 (manual-mode month rollover, customer absent).
func (p *Provider) fireExpiredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, now time.Time) {
	// F4 fix: cap_usd from cfg (new month's cap). spend_usd per spec §8.5
	// line 809 is "0.00" (new month resets); final_spend_usd carries the
	// previous month's spend.
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), 0)
	data["app"] = app
	data["expired_at"] = now.Format(time.RFC3339)
	data["recovery_mode"] = state.RecoveryMode
	// F6 fix: rename originally_shut_down_at → original_shutdown_at;
	// services_left_at_zero → services_still_at_zero.
	if state.ShutdownAt != nil {
		data["original_shutdown_at"] = state.ShutdownAt.UTC().Format(time.RFC3339)
		// F12 fix: prev_month_label = month of original ShutdownAt.
		data["prev_month_label"] = state.ShutdownAt.UTC().Format("2006-01")
	}
	// F12 fix: original_armed_at, prev_month_label, new_month_label,
	// flap_suppressed_until, requires_manual_action, manual_action_hint,
	// final_spend_usd.
	if state.ArmedAt != nil {
		data["original_armed_at"] = state.ArmedAt.UTC().Format(time.RFC3339)
	}
	data["new_month_label"] = now.UTC().Format("2006-01")
	if state.FlapSuppressedUntil != nil {
		data["flap_suppressed_until"] = state.FlapSuppressedUntil.UTC().Format(time.RFC3339)
	}
	data["requires_manual_action"] = "true"
	data["manual_action_hint"] = fmt.Sprintf("convox services update --count <N> -a %s", app)
	if baseState != nil {
		data["final_spend_usd"] = strconv.FormatFloat(baseState.CurrentMonthSpendUsd, 'f', 2, 64)
	}
	names := make([]string, 0, len(state.Services))
	for _, s := range state.Services {
		names = append(names, s.Name)
	}
	data["services_still_at_zero"] = formatServiceList(names)
	_ = p.EventSend(shutdownEventName("expired"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=expired app=%s tick_id=%s\n", app, state.ShutdownTickId)
}

// fireFlapSuppressedEvent emits :flap-suppressed per spec §8.6.
func (p *Provider) fireFlapSuppressedEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, suppressedUntil, now time.Time) {
	// F4 fix: cap_usd from cfg, spend_usd from baseState.
	data := universalEventData("system", generateShutdownTickID(now), false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["suppressed_at"] = now.Format(time.RFC3339)
	// F6 fix: rename flap_suppressed_until → cooldown_expires_at per spec §8.6 line 841.
	data["cooldown_expires_at"] = suppressedUntil.UTC().Format(time.RFC3339)
	data["cooldown_remaining_min"] = strconv.Itoa(int(time.Until(suppressedUntil).Minutes()))
	_ = p.EventSend(shutdownEventName("flap-suppressed"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=flap_suppressed app=%s suppressed_until=%s\n", app, suppressedUntil)
}

// fireNoopEvent emits :noop per spec §8.9.
func (p *Provider) fireNoopEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, reason string, plan *shutdownPlanResult, now time.Time) {
	// F4 fix: cap_usd from cfg, spend_usd from baseState.
	data := universalEventData("system", generateShutdownTickID(now), false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	// F6 fix: rename noop_at → evaluated_at per spec §8.9 line 915.
	data["evaluated_at"] = now.Format(time.RFC3339)
	data["reason"] = reason
	data["eligible_service_count"] = "0"
	if plan != nil {
		data["total_services"] = strconv.Itoa(len(plan.eligibility))
		exempted := 0
		for _, e := range plan.eligibility {
			if !e.Eligible {
				exempted++
			}
		}
		data["exempted_count"] = strconv.Itoa(exempted)
	}
	_ = p.EventSend(shutdownEventName("noop"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=noop app=%s reason=%s\n", app, reason)
}

// fireCancelledEventRich emits :cancelled with a reason + the full §8.4
// payload field set per spec. F11 fix: armed_at, expected_shutdown_at,
// prev_cap_usd, new_cap_usd (cap-raised case), eligible_services,
// new_action (config-changed case). Plus the universal payload.
func (p *Provider) fireCancelledEventRich(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, actor string, reason string, prevCapUsd, newCapUsd float64, newAction string, now time.Time) {
	// F4 fix: cap_usd from cfg (or new cap if cap-raised), spend_usd from baseState.
	capValue := capUsdFor(cfg)
	if reason == "cap-raised" && newCapUsd > 0 {
		capValue = newCapUsd
	}
	tickID := ""
	if state != nil {
		tickID = state.ShutdownTickId
	}
	if tickID == "" {
		tickID = generateShutdownTickID(now)
	}
	// F-3 fix: actor is passed in by the caller. Spec §8.4 line 777
	// mandates JWT-derived actor for sub-cases where an HTTP request is
	// in scope at the trigger point: reset-during-armed (caller is
	// AppBudgetReset, ackBy is the JWT user) and cap-raised (caller is
	// the accumulator, but the originating user is recovered from
	// cfg.LastCapMutationBy which AppBudgetSet persisted on the cap
	// mutation — see MF-6 fix). Accumulator-only sub-cases without any
	// caller-side identity (manual-detected, config-changed) legitimately
	// keep "system" because no user-identity is recoverable at detection.
	data := universalEventData(actor, tickID, false, capValue, spendUsdFor(baseState))
	data["app"] = app
	data["cancelled_at"] = now.Format(time.RFC3339)
	data["cancel_reason"] = reason
	if state != nil {
		if state.ArmedAt != nil {
			data["armed_at"] = state.ArmedAt.UTC().Format(time.RFC3339)
			// expected_shutdown_at = armedAt + notifyBeforeMinutes (default).
			expected := state.ArmedAt.Add(time.Duration(structs.BudgetDefaultNotifyBeforeMinutes) * time.Minute)
			data["expected_shutdown_at"] = expected.UTC().Format(time.RFC3339)
		}
		names := make([]string, 0, len(state.Services))
		for _, s := range state.Services {
			names = append(names, s.Name)
		}
		data["eligible_services"] = formatServiceList(names)
	}
	if reason == "cap-raised" {
		data["prev_cap_usd"] = strconv.FormatFloat(prevCapUsd, 'f', 0, 64)
		data["new_cap_usd"] = strconv.FormatFloat(newCapUsd, 'f', 0, 64)
	}
	if reason == "config-changed" && newAction != "" {
		data["new_action"] = newAction
	}
	_ = p.EventSend(shutdownEventName("cancelled"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=cancelled app=%s reason=%s\n", app, reason)
}

// fireRestoredEvent emits :restored with a recovery-trigger reason per spec §8.10.
// F4 fix: populates cap_usd and spend_usd. F16 advisory partial fix:
// also surfaces restored_services / restored_count when state has them.
func (p *Provider) fireRestoredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, trigger string, now time.Time) {
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["recovery_at"] = now.Format(time.RFC3339)
	data["recovery_trigger"] = trigger
	if state.FlapSuppressedUntil != nil {
		data["flap_suppressed_until"] = state.FlapSuppressedUntil.UTC().Format(time.RFC3339)
	}
	// Surface service list + counts when present (post-fired, manual-detected).
	names := make([]string, 0, len(state.Services))
	keda := 0
	depOnly := 0
	for _, s := range state.Services {
		names = append(names, s.Name)
		if s.KedaScaledObject != nil {
			keda++
		} else {
			depOnly++
		}
	}
	if len(names) > 0 {
		data["restored_services"] = formatServiceList(names)
		data["restored_count"] = strconv.Itoa(len(names))
		data["restored_to_keda"] = strconv.Itoa(keda)
		data["restored_to_deployment"] = strconv.Itoa(depOnly)
	}
	if baseState != nil {
		data["final_spend_usd"] = strconv.FormatFloat(baseState.CurrentMonthSpendUsd, 'f', 2, 64)
	}
	data["drift_detected"] = "false"
	_ = p.EventSend(shutdownEventName("restored"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=restored app=%s trigger=%s\n", app, trigger)
}

// fireFailedEvent emits :failed per spec §8.7. F4 fix: populates cap_usd
// and spend_usd from cfg/baseState (was 0, 0).
func (p *Provider) fireFailedEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, actor, tickID string, now time.Time, failed []string, reason string, partialState int) {
	data := universalEventData(actor, tickID, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["failed_at"] = now.Format(time.RFC3339)
	data["failed_services"] = formatServiceList(failed)
	data["failure_reason"] = reason
	data["partial_state"] = strconv.Itoa(partialState)
	data["retry_count"] = strconv.Itoa(budgetShutdownPatchRetries)
	_ = p.EventSend(shutdownEventName("failed"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=failed app=%s reason=%s\n", app, reason)
}

// fireFailedEventStateCorrupt is the corrupt-state-specific :failed
// emission used by the accumulator path before reading the state
// annotation. Since the state is unparseable, neither cap nor spend is
// derivable from state — we use the live cfg + baseState (which the
// accumulator reads directly from the namespace, NOT via the corrupt
// shutdown-state annotation). Per F10 fix.
func (p *Provider) fireFailedEventStateCorrupt(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, now time.Time) {
	p.fireFailedEvent(app, cfg, baseState, "system", generateShutdownTickID(now), now, []string{}, structs.BudgetShutdownReasonStateCorrupt, 0)
}

// classifyNoopReason returns the spec §8.9 reason enum for the :noop event
// when len(plan.ordered) == 0 (zero-eligibility case). Three values:
//
//   - "no-eligible-services" — manifest has 0 services OR every service was
//     filtered for a STATIC-config reason (in neverAutoShutdown / agent
//     DaemonSet). Customer's webhook receiver should treat this as a
//     persistent configuration outcome.
//   - "runtime-drift"        — manifest has services but at least one was
//     filtered for a RUNTIME reason (Deployment not yet created on first
//     deploy, etc.). The cluster state has not converged on the manifest yet;
//     customer should expect the :armed firing on a later tick.
//   - "external-edit-detected" — branch handled by the caller before this
//     function (plan.ordered > 0 but all replicas == 0). Documented here for
//     completeness; this function returns one of the first two for the
//     plan.ordered == 0 case.
//
// The third reason "external-edit-detected" is documented but never returned
// here — see branch (6) in reconcileAutoShutdown for that classification path.
func classifyNoopReason(m *manifest.Manifest, plan *shutdownPlanResult) string {
	if m == nil || len(m.Services) == 0 {
		return "no-eligible-services"
	}
	if plan == nil {
		return "no-eligible-services"
	}
	for _, e := range plan.eligibility {
		if e.Eligible {
			continue
		}
		// Runtime-drift signals: the cluster has not converged on the
		// manifest. "no deployment yet" is the canonical first-deploy case.
		if e.Reason == "no deployment yet (pending first deploy)" {
			return "runtime-drift"
		}
	}
	return "no-eligible-services"
}

// ptrTimePackage is the in-package time pointer helper. (ptrTime in tests
// is _test.go-only; production callers cannot reach it.)
func ptrTimePackage(t time.Time) *time.Time {
	return &t
}
