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
	budgetTickShutdownGrace    = 5 * time.Second
)

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

func (p *Provider) AppBudgetSet(app string, opts structs.AppBudgetOptions, ackBy string) error {
	if err := p.requireCostTrackingForBudget(opts); err != nil {
		return err
	}

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
	var capRaiseArmedShutdownState *structs.AppBudgetShutdownState
	var capRaiseShutdownStateBaseState *structs.AppBudgetState

	for i := 0; i < budgetWriteConflictRetries; i++ {
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
		if opts.MonthlyCapUsd != nil {
			cfg.LastCapMutationBy = ackBy
		}
		cfg.ApplyDefaults()
		if err := cfg.Validate(); err != nil {
			return errors.WithStack(structs.ErrBadRequest("%s", err.Error()))
		}
		final = *cfg

		// Clear breaker atomically on cap raise above current spend
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

			// GC armed shutdown annotation to avoid stale "ARMED" banner
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

			if capRaiseArmedShutdownState != nil {
				p.fireCancelledEvent(app, &final, capRaiseShutdownStateBaseState, capRaiseArmedShutdownState, ackBy, "cap-raised", breakerClearedPrevCap, breakerClearedNewCap, "", breakerClearedAckAt)
			}
		}

		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to set budget after %d retries", budgetWriteConflictRetries))
}

func (p *Provider) AppBudgetClear(app string, ackBy string) error {
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()

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

func (p *Provider) AppBudgetReset(app string, ackBy string) error {
	mu := appBudgetLock(app)
	mu.Lock()
	defer mu.Unlock()
	return p.appBudgetResetLocked(app, ackBy, structs.AppBudgetResetOptions{})
}

// caller must hold appBudgetLock(app)
func (p *Provider) appBudgetResetLocked(app string, ackBy string, opts structs.AppBudgetResetOptions) error {
	nsName := p.AppNamespace(app)

	ackBy = sanitizeAckBy(ackBy)

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

		if opts.ResetPeriod {
			state.MonthStart = now
			state.CurrentMonthSpendUsd = 0
			state.CurrentMonthSpendAsOf = time.Time{}
			state.PerServiceSpendUsd = nil
			state.PerServiceInstanceType = nil
			state.PerServiceSpendByVariant = nil
			state.PerServiceVariantPodsLastTick = nil
		}

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

		resetKind := "breaker-clear"
		if opts.ResetPeriod {
			resetKind = "period-reset"
		}
		fmt.Printf("ns=budget_accumulator at=alert kind=%s app=%s ack_by=%q prev_spend_usd=%.2f cap_usd=%.2f\n",
			resetKind, app, ackBy, prevSpend, capUsd)
		_ = p.EventSend("app:budget:reset", structs.EventSendOptions{Data: map[string]string{
			"app":            app,
			"ack_by":         ackBy,
			"prev_spend_usd": strconv.FormatFloat(prevSpend, 'f', 2, 64),
			"cap_usd":        strconv.FormatFloat(capUsd, 'f', 2, 64),
			"reset_at":       state.CircuitBreakerAckAt.Format(time.RFC3339),
			"reset_kind":     resetKind,
		}})

		// GC armed shutdown annotation on reset
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

func sanitizeAckBy(in string) string {
	return audit.SanitizeActor(in)
}

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
		VariantBreakdown:    buildVariantBreakdown(state),
		PricingSource:       billing.PricingSourceStaticTable,
		PricingTableVersion: billing.PricingTableVersion(),
		PricingAdjustment:   adjustment,
		WarningCount:        state.WarningCount,
		TrackingEnabled:     p.costTrackingEnabled(),
	}, nil
}

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

func buildVariantBreakdown(state *structs.AppBudgetState) []structs.ServiceVariantCostLine {
	if state == nil || len(state.PerServiceSpendByVariant) == 0 {
		return []structs.ServiceVariantCostLine{}
	}

	out := make([]structs.ServiceVariantCostLine, 0)
	for svc, variants := range state.PerServiceSpendByVariant {
		for variantKey, spend := range variants {
			idx := strings.LastIndex(variantKey, ":")
			if idx <= 0 {
				continue
			}
			line := structs.ServiceVariantCostLine{
				Service:      svc,
				InstanceType: variantKey[:idx],
				CapacityType: variantKey[idx+1:],
				SpendUsd:     spend,
			}
			if pods, ok := state.PerServiceVariantPodsLastTick[svc]; ok {
				if n, ok := pods[variantKey]; ok {
					line.Replicas = n
				}
			}
			out = append(out, line)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SpendUsd != out[j].SpendUsd {
			return out[i].SpendUsd > out[j].SpendUsd
		}
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		if out[i].InstanceType != out[j].InstanceType {
			return out[i].InstanceType < out[j].InstanceType
		}
		return out[i].CapacityType < out[j].CapacityType
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

func (p *Provider) accumulateBudgetTick(ctx context.Context) error {
	if p.Name == "" {
		return nil
	}

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

		adjustment := 1.0
		if cfg != nil && cfg.PricingAdjustment > 0 {
			adjustment = cfg.PricingAdjustment
		}
		delta, perSvc, perSvcInst, perSvcVariant, perSvcVariantPods, warnings, err := p.computeBudgetDelta(ctx, app, state.CurrentMonthSpendAsOf, now, adjustment)
		if err != nil {
			return err
		}

		state.CurrentMonthSpendUsd += delta
		state.CurrentMonthSpendAsOf = now
		state.WarningCount = warnings

		if state.PerServiceSpendUsd == nil {
			state.PerServiceSpendUsd = map[string]float64{}
		}
		if state.PerServiceInstanceType == nil {
			state.PerServiceInstanceType = map[string]string{}
		}
		if state.PerServiceSpendByVariant == nil {
			state.PerServiceSpendByVariant = map[string]map[string]float64{}
		}
		if state.PerServiceVariantPodsLastTick == nil {
			state.PerServiceVariantPodsLastTick = map[string]map[string]int{}
		}
		for svc := range state.PerServiceVariantPodsLastTick {
			if _, ok := perSvcVariantPods[svc]; !ok {
				delete(state.PerServiceVariantPodsLastTick, svc)
			}
		}
		truncated := 0
		for svc, dollars := range perSvc {
			if _, exists := state.PerServiceSpendUsd[svc]; !exists && len(state.PerServiceSpendUsd) >= perServiceMaxEntries {
				truncated++
				continue
			}
			state.PerServiceSpendUsd[svc] += dollars
			if tickVariants, ok := perSvcVariant[svc]; ok {
				if state.PerServiceSpendByVariant[svc] == nil {
					state.PerServiceSpendByVariant[svc] = map[string]float64{}
				}
				for k, v := range tickVariants {
					state.PerServiceSpendByVariant[svc][k] += v
				}
			}
			if tickPods, ok := perSvcVariantPods[svc]; ok {
				clone := make(map[string]int, len(tickPods))
				for k, v := range tickPods {
					clone[k] = v
				}
				state.PerServiceVariantPodsLastTick[svc] = clone
			} else {
				delete(state.PerServiceVariantPodsLastTick, svc)
			}
			if it := dominantInstanceTypeFromVariants(state.PerServiceSpendByVariant[svc]); it != "" {
				state.PerServiceInstanceType[svc] = it
			} else if it := perSvcInst[svc]; it != "" {
				state.PerServiceInstanceType[svc] = it
			}
		}
		if truncated > 0 {
			_ = p.EventSend("app:budget:per-service-truncated", structs.EventSendOptions{Data: map[string]string{
				"app":     app,
				"dropped": strconv.Itoa(truncated),
				"cap":     strconv.Itoa(perServiceMaxEntries),
				"actor":   "system",
			}})
			fmt.Printf("ns=budget_accumulator at=per_service_truncated app=%s count=%d cap=%d\n", app, truncated, perServiceMaxEntries)
		}

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

		if cfg != nil {
			p.reconcileAutoShutdown(ctx, app, cfg, state, now)
		}

		_ = p.runStaleAnnotationGC(ctx, app, budgetDefaultPollInterval)

		return nil
	}
	return fmt.Errorf("failed to write budget state for %s after %d retries", app, budgetWriteConflictRetries)
}

const (
	perServiceBucketBuild        = "_build"
	perServiceBucketUnattributed = "_unattributed"
)

// var so tests can override
var perServiceMaxEntries = 1000

func (p *Provider) computeBudgetDelta(ctx context.Context, app string, lastTick, now time.Time, adjustment float64) (float64, map[string]float64, map[string]string, map[string]map[string]float64, map[string]map[string]int, int, error) {
	_ = ctx
	if lastTick.IsZero() {
		lastTick = now
	}
	elapsed := now.Sub(lastTick).Hours()
	if elapsed <= 0 {
		return 0, nil, nil, nil, nil, 0, nil
	}
	if adjustment <= 0 {
		adjustment = 1.0
	}

	nodes, err := p.ListNodesFromInformer("")
	if err != nil {
		return 0, nil, nil, nil, nil, 0, errors.WithStack(err)
	}
	nodeByName := map[string]*v1.Node{}
	for i := range nodes.Items {
		nodeByName[nodes.Items[i].Name] = &nodes.Items[i]
	}

	pods, err := p.ListPodsFromInformer(p.AppNamespace(app), "")
	if err != nil {
		return 0, nil, nil, nil, nil, 0, errors.WithStack(err)
	}

	var delta float64
	perSvc := map[string]float64{}
	perSvcInst := map[string]string{}
	perSvcVariant := map[string]map[string]float64{}
	perSvcVariantPods := map[string]map[string]int{}
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

		variantCap := capacityType
		if variantCap == "" {
			variantCap = structs.CapacityTypeUnknown
		}
		variantKey := instanceType + ":" + variantCap
		if perSvcVariant[svc] == nil {
			perSvcVariant[svc] = map[string]float64{}
		}
		perSvcVariant[svc][variantKey] += podTickSpend

		if perSvcVariantPods[svc] == nil {
			perSvcVariantPods[svc] = map[string]int{}
		}
		perSvcVariantPods[svc][variantKey]++
	}

	return delta, perSvc, perSvcInst, perSvcVariant, perSvcVariantPods, warnings, nil
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

func nodeCapacityType(n *v1.Node) string {
	if n == nil || n.Labels == nil {
		return ""
	}
	if v := strings.ToLower(n.Labels["karpenter.sh/capacity-type"]); v == "spot" || v == "on-demand" {
		return v
	}
	switch strings.ToUpper(n.Labels["eks.amazonaws.com/capacityType"]) {
	case "SPOT":
		return "spot"
	case "ON_DEMAND":
		return "on-demand"
	}
	return ""
}

func dominantInstanceTypeFromVariants(variants map[string]float64) string {
	if len(variants) == 0 {
		return ""
	}
	totalsByInstanceType := map[string]float64{}
	for variantKey, spend := range variants {
		idx := strings.LastIndex(variantKey, ":")
		if idx <= 0 {
			continue
		}
		it := variantKey[:idx]
		totalsByInstanceType[it] += spend
	}
	if len(totalsByInstanceType) == 0 {
		return ""
	}
	var best string
	var bestSpend float64
	for it, spend := range totalsByInstanceType {
		if spend > bestSpend || (spend == bestSpend && it < best) {
			best = it
			bestSpend = spend
		}
	}
	return best
}

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
	return structs.ErrConflict(
		"budget cap exceeded for app %s: spent $%.2f of $%.2f cap this month; "+
			"recovery options: (1) `convox budget reset %s` to acknowledge and re-enable deploys, "+
			"(2) `convox budget cap raise --monthly-cap-usd <higher> %s` to raise the cap (alias for `budget set --monthly-cap`), "+
			"or (3) wait for month rollover (caps reset automatically on the 1st)",
		app, state.CurrentMonthSpendUsd, capUsd, app, app,
	)
}

func (p *Provider) costTrackingEnabled() bool {
	return os.Getenv("COST_TRACKING_ENABLE") == "true"
}

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
