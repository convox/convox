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

func (p *Provider) reconcileAutoShutdown(ctx context.Context, app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, now time.Time) {
	if cfg == nil || cfg.AtCapAction != structs.BudgetAtCapActionAutoShutdown {
		return
	}
	if !p.costTrackingEnabled() {
		return
	}

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
		if !p.stateCorruptDedupExpired(ns.Annotations, now) {
			return
		}
		// persist dedup before emit to avoid duplicate :failed on next tick
		if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
			p.fireFailedEventStateCorrupt(app, cfg, baseState, now)
		}
		return
	}
	if _, ok := ns.Annotations[structs.BudgetShutdownStateCorruptFiredAtAnnotation]; ok {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation)
	}

	if shutdownState != nil && shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
		(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
		if p.armedWindowManuallyScaledUp(ctx, app, shutdownState.Services) {
			derr := p.deleteBudgetShutdownStateAnnotation(ctx, app)
			if derr == nil || ae.IsNotFound(derr) {
				p.fireCancelledEvent(app, cfg, baseState, shutdownState, "system", "manual-detected", 0, 0, "", now)
			}
			return
		}
	}

	if handled := p.reconcileAutoShutdownPreManifest(ctx, app, cfg, shutdownState, baseState, now); handled {
		return
	}

	if baseState == nil || baseState.AlertFiredAtCap.IsZero() {
		return
	}

	m, _, mErr := p.releaseManifestForApp(app)
	if mErr != nil {
		return
	}
	p.reconcileAutoShutdownWithManifest(ctx, app, cfg, baseState, ns, shutdownState, m, now)
}

func (p *Provider) reconcileAutoShutdownPreManifest(ctx context.Context, app string, cfg *structs.AppBudget, shutdownState *structs.AppBudgetShutdownState, baseState *structs.AppBudgetState, now time.Time) bool {
	if shutdownState != nil && shutdownState.ShutdownAt != nil && !shutdownState.ShutdownAt.IsZero() &&
		shutdownState.ExpiredAt == nil && shutdownState.RestoredAt == nil &&
		shutdownState.RecoveryMode == "manual" {
		if startOfMonth(*shutdownState.ShutdownAt).Before(startOfMonth(now)) {
			shutdownState.ExpiredAt = ptrTimePackage(now)
			if shutdownState.ExpiredNotificationFiredAt == nil {
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

	if shutdownState != nil && shutdownState.ShutdownAt != nil && !shutdownState.ShutdownAt.IsZero() &&
		shutdownState.RestoredAt == nil {
		manualRecovered := p.allServicesScaledUp(ctx, app, shutdownState.Services)
		if manualRecovered {
			shutdownState.RestoredAt = ptrTimePackage(now)
			flap := now.Add(structs.BudgetFlapCooldown)
			shutdownState.FlapSuppressedUntil = ptrTimePackage(flap)
			if shutdownState.RestoredNotificationFiredAt == nil {
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

func (p *Provider) reconcileAutoShutdownWithManifest(ctx context.Context, app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, ns *ac.Namespace, shutdownState *structs.AppBudgetShutdownState, m *manifest.Manifest, now time.Time) {
	plan, pErr := p.computeShutdownPlanForApp(ctx, app, m, cfg)
	if pErr != nil {
		fmt.Printf("ns=auto_shutdown at=plan_failed app=%s err=%q\n", app, pErr)
		return
	}

	if shutdownState != nil && shutdownState.ManifestSha256 != "" && shutdownState.ManifestSha256 != plan.manifestSha {
		armed := shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
			(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero())
		if armed {
			reason := "config-changed"
			var prevCap, newCap float64
			if cfg != nil {
				newCap = cfg.MonthlyCapUsd
			}
			// spend is the floor estimate for prev cap (breaker wouldn't have armed below it)
			if cfg != nil && baseState != nil &&
				cfg.MonthlyCapUsd > baseState.CurrentMonthSpendUsd &&
				baseState.CurrentMonthSpendUsd > 0 {
				reason = "cap-raised"
				prevCap = baseState.CurrentMonthSpendUsd
			}
			if shutdownState.CancelledNotificationFiredAt == nil {
				shutdownState.CancelledNotificationFiredAt = ptrTimePackage(now)
				if perr := p.persistShutdownState(ctx, app, shutdownState); perr == nil {
					newAction := ""
					if reason == "config-changed" && cfg != nil {
						newAction = cfg.AtCapAction
					}
					actor := "system"
					if reason == "cap-raised" && cfg != nil && cfg.LastCapMutationBy != "" {
						actor = cfg.LastCapMutationBy
					}
					p.fireCancelledEvent(app, cfg, baseState, shutdownState, actor, reason, prevCap, newCap, newAction, now)
				}
			}
			_ = p.deleteBudgetShutdownStateAnnotation(ctx, app)
			return
		}
		if shutdownState.RestoredAt == nil {
			shutdownState.RestoredAt = ptrTimePackage(now)
			if shutdownState.RestoredNotificationFiredAt == nil {
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

	flap, _ := readFlapSuppressedUntilAnnotation(ns.Annotations)
	if flap != nil && flap.After(now) {
		if _, fired := ns.Annotations[structs.BudgetFlapSuppressFiredAtAnnotation]; !fired {
			if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetFlapSuppressFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
				p.fireFlapSuppressedEvent(app, cfg, baseState, *flap, now)
			}
		}
		return
	}

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
				if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
					p.fireNoopEvent(app, cfg, baseState, "external-edit-detected", plan, now)
				}
			}
			return
		}
	}

	if shutdownState == nil {
		if len(plan.ordered) == 0 {
			reason := classifyNoopReason(m, plan)
			if p.noopDedupExpired(ns.Annotations, now) {
				if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
					p.fireNoopEvent(app, cfg, baseState, reason, plan, now)
				}
			}
			return
		}
		if _, ok := ns.Annotations[structs.BudgetShutdownNoopFiredAtAnnotation]; ok {
			_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetShutdownNoopFiredAtAnnotation)
		}
		armedNow := now
		shutdownTickID := generateShutdownTickID(armedNow)
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
		if perr := p.persistShutdownState(ctx, app, newState); perr == nil {
			p.fireArmedEvent(app, cfg, baseState, newState, plan, now)
		}
		return
	}

	if shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
		(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
		notifyMin := plan.notifyBeforeMinutes
		if notifyMin <= 0 {
			notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
		}
		fireAt := shutdownState.ArmedAt.Add(time.Duration(notifyMin) * time.Minute)
		if !now.Before(fireAt) {
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

			if len(failed) > 0 {
				if shutdownState.FailedNotificationFiredAt == nil {
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
		if int(*dep.Spec.Replicas) > svcs[i].OriginalScale.Replicas {
			return true
		}
	}
	return false
}

func (p *Provider) noopDedupExpired(ann map[string]string, now time.Time) bool {
	raw, ok := ann[structs.BudgetShutdownNoopFiredAtAnnotation]
	if !ok || raw == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return true
	}
	return now.Sub(t) > budgetDefaultPollInterval-time.Second
}

// marker stored in separate annotation key because state JSON is unparseable
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

func kedaScaledObjectFromPlan(sp shutdownPlan) *structs.AppBudgetShutdownStateKeda {
	if !sp.HasKeda {
		return nil
	}
	return &structs.AppBudgetShutdownStateKeda{
		Name:                        sp.Service,
		PausedReplicasAnnotationSet: false,
	}
}

// returns error so callers can gate event emit on persist success
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

func capUsdFor(cfg *structs.AppBudget) float64 {
	if cfg == nil {
		return 0
	}
	return cfg.MonthlyCapUsd
}

func spendUsdFor(baseState *structs.AppBudgetState) float64 {
	if baseState == nil {
		return 0
	}
	return baseState.CurrentMonthSpendUsd
}

func (p *Provider) fireArmedEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, plan *shutdownPlanResult, now time.Time) {
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
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

func (p *Provider) fireFiredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, succeeded []string, plan *shutdownPlanResult, now time.Time) {
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["shutdown_at"] = now.Format(time.RFC3339)
	data["shut_down_services"] = formatServiceList(succeeded)
	data["shut_down_count"] = strconv.Itoa(len(succeeded))
	data["shutdown_order"] = plan.shutdownOrder
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

func (p *Provider) fireExpiredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, now time.Time) {
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), 0)
	data["app"] = app
	data["expired_at"] = now.Format(time.RFC3339)
	data["recovery_mode"] = state.RecoveryMode
	if state.ShutdownAt != nil {
		data["original_shutdown_at"] = state.ShutdownAt.UTC().Format(time.RFC3339)
		data["prev_month_label"] = state.ShutdownAt.UTC().Format("2006-01")
	}
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

func (p *Provider) fireFlapSuppressedEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, suppressedUntil, now time.Time) {
	data := universalEventData("system", generateShutdownTickID(now), false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["suppressed_at"] = now.Format(time.RFC3339)
	data["cooldown_expires_at"] = suppressedUntil.UTC().Format(time.RFC3339)
	data["cooldown_remaining_min"] = strconv.Itoa(int(time.Until(suppressedUntil).Minutes()))
	_ = p.EventSend(shutdownEventName("flap-suppressed"), structs.EventSendOptions{Data: data})
	fmt.Printf("ns=auto_shutdown at=flap_suppressed app=%s suppressed_until=%s\n", app, suppressedUntil)
}

func (p *Provider) fireNoopEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, reason string, plan *shutdownPlanResult, now time.Time) {
	data := universalEventData("system", generateShutdownTickID(now), false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
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

func (p *Provider) fireCancelledEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, actor string, reason string, prevCapUsd, newCapUsd float64, newAction string, now time.Time) {
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
	data := universalEventData(actor, tickID, false, capValue, spendUsdFor(baseState))
	data["app"] = app
	data["cancelled_at"] = now.Format(time.RFC3339)
	data["cancel_reason"] = reason
	if state != nil {
		if state.ArmedAt != nil {
			data["armed_at"] = state.ArmedAt.UTC().Format(time.RFC3339)
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

func (p *Provider) fireRestoredEvent(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, state *structs.AppBudgetShutdownState, trigger string, now time.Time) {
	data := universalEventData("system", state.ShutdownTickId, false, capUsdFor(cfg), spendUsdFor(baseState))
	data["app"] = app
	data["recovery_at"] = now.Format(time.RFC3339)
	data["recovery_trigger"] = trigger
	if state.FlapSuppressedUntil != nil {
		data["flap_suppressed_until"] = state.FlapSuppressedUntil.UTC().Format(time.RFC3339)
	}
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

func (p *Provider) fireFailedEventStateCorrupt(app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, now time.Time) {
	p.fireFailedEvent(app, cfg, baseState, "system", generateShutdownTickID(now), now, []string{}, structs.BudgetShutdownReasonStateCorrupt, 0)
}

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
		if e.Reason == "no deployment yet (pending first deploy)" {
			return "runtime-drift"
		}
	}
	return "no-eligible-services"
}

func ptrTimePackage(t time.Time) *time.Time {
	return &t
}
