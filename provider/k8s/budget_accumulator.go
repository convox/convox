package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"time"

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
	nsName := p.AppNamespace(app)
	ackBy = sanitizeAckBy(ackBy)

	var prev structs.AppBudget
	var final structs.AppBudget

	for i := 0; i < budgetWriteConflictRetries; i++ {
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
		cfg.ApplyDefaults()
		if err := cfg.Validate(); err != nil {
			return errors.WithStack(structs.ErrBadRequest("%s", err.Error()))
		}
		final = *cfg

		data, err := json.Marshal(cfg)
		if err != nil {
			return errors.WithStack(err)
		}

		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[structs.BudgetConfigAnnotation] = string(data)

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
func (p *Provider) AppBudgetReset(app string, ackBy string) error {
	nsName := p.AppNamespace(app)

	ackBy = sanitizeAckBy(ackBy)

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
		state.CircuitBreakerAckAt = time.Now().UTC()

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
		return nil
	}
	return errors.WithStack(fmt.Errorf("failed to reset budget after %d retries", budgetWriteConflictRetries))
}

// sanitizeAckBy caps the ack_by audit string and strips control characters.
// Guards against annotation-size DoS and webhook/log injection via
// unvalidated client input.
func sanitizeAckBy(in string) string {
	const maxLen = 256
	out := make([]rune, 0, len(in))
	for _, r := range in {
		if r < 0x20 || r == 0x7f {
			continue
		}
		out = append(out, r)
		if len(out) >= maxLen {
			break
		}
	}
	if len(out) == 0 {
		return "unknown"
	}
	return string(out)
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
		Breakdown:           []structs.ServiceCostLine{},
		PricingSource:       "on-demand-static-table",
		PricingTableVersion: billing.PricingTableVersion(),
		PricingAdjustment:   adjustment,
		WarningCount:        state.WarningCount,
	}, nil
}

func applyBudgetOptions(dst *structs.AppBudget, opts structs.AppBudgetOptions) error {
	if opts.MonthlyCapUsd != nil {
		v, err := strconv.ParseFloat(*opts.MonthlyCapUsd, 64)
		if err != nil {
			return structs.ErrBadRequest("monthly_cap_usd must be a number: %v", err)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return structs.ErrBadRequest("monthly_cap_usd must be a finite number")
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
			return structs.ErrBadRequest("pricing_adjustment must be a number: %v", err)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return structs.ErrBadRequest("pricing_adjustment must be a finite number")
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
	p.safeBudgetTick()

	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("ns=budget_accumulator at=stop\n")
			return
		case <-tick.C:
			p.safeBudgetTick()
		}
	}
}

// safeBudgetTick runs one tick with its own panic-recovery scope so a
// goroutine that panics once can still run on the next interval.
func (p *Provider) safeBudgetTick() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=budget_accumulator at=panic recover=%q stack=%q\n", r, debug.Stack())
		}
	}()
	if err := p.accumulateBudgetTick(); err != nil {
		fmt.Printf("ns=budget_accumulator at=tick error=%q\n", err)
	}
}

// accumulateBudgetTick iterates all apps with budget configured and updates
// spend + alert + breaker state. The namespace list is rack-scoped so shared
// clusters with two V3 racks do not cross-attribute.
func (p *Provider) accumulateBudgetTick() error {
	// Defense in depth: an empty rack name would produce a selector like
	// `rack=` which matches nothing in a healthy cluster, but short-circuit
	// explicitly to avoid the wasted API call and to make the intent clear
	// in source.
	if p.Name == "" {
		return nil
	}
	selector := fmt.Sprintf("system=convox,rack=%s,type=app", p.Name)
	ns, err := p.ListNamespacesFromInformer(selector)
	if err != nil {
		return errors.WithStack(err)
	}

	now := time.Now().UTC()

	for i := range ns.Items {
		n := &ns.Items[i]
		cfg, err := readBudgetConfigAnnotation(n.Annotations)
		if err != nil {
			fmt.Printf("ns=budget_accumulator at=config_parse namespace=%s error=%q\n", n.Name, err)
			continue
		}
		if cfg == nil {
			continue
		}
		app := n.Labels["app"]
		if app == "" {
			continue
		}
		if err := p.accumulateBudgetApp(app, now); err != nil {
			fmt.Printf("ns=budget_accumulator app=%s error=%q\n", app, err)
		}
	}
	return nil
}

func (p *Provider) accumulateBudgetApp(app string, now time.Time) error {
	nsName := p.AppNamespace(app)

	for i := 0; i < budgetWriteConflictRetries; i++ {
		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), nsName, am.GetOptions{})
		if err != nil {
			return errors.WithStack(err)
		}
		cfg, err := readBudgetConfigAnnotation(ns.Annotations)
		if err != nil {
			fmt.Printf("ns=budget_accumulator at=config_parse app=%s error=%q\n", app, err)
			return nil
		}
		if cfg == nil {
			return nil
		}
		// Defense in depth: a corrupt annotation with MonthlyCapUsd <= 0, NaN,
		// or Inf would otherwise fire a cap alert on every tick or silently
		// suppress alerts (NaN comparisons return false). Validate() rejects
		// these at write time, but hand-edited annotations could bypass that.
		if cfg.MonthlyCapUsd <= 0 || math.IsNaN(cfg.MonthlyCapUsd) || math.IsInf(cfg.MonthlyCapUsd, 0) {
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

		delta, warnings, err := p.computeBudgetDelta(app, state.CurrentMonthSpendAsOf, now, cfg.PricingAdjustment)
		if err != nil {
			return err
		}

		state.CurrentMonthSpendUsd += delta
		state.CurrentMonthSpendAsOf = now
		state.WarningCount = warnings

		willFireThreshold := !state.CurrentMonthSpendAsOf.IsZero() &&
			state.CurrentMonthSpendUsd >= cfg.MonthlyCapUsd*(cfg.AlertThresholdPercent/100) &&
			state.AlertFiredAtThreshold.IsZero()
		willFireCap := state.CurrentMonthSpendUsd >= cfg.MonthlyCapUsd && state.AlertFiredAtCap.IsZero()

		if willFireThreshold {
			state.AlertFiredAtThreshold = now
		}
		if willFireCap {
			state.AlertFiredAtCap = now
			if cfg.AtCapAction == structs.BudgetAtCapActionBlockNewDeploys {
				state.CircuitBreakerTripped = true
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

		if _, err := p.Cluster.CoreV1().Namespaces().Update(context.TODO(), ns, am.UpdateOptions{}); err != nil {
			if ae.IsConflict(err) {
				continue
			}
			return errors.WithStack(err)
		}

		if willFireThreshold {
			fmt.Printf("ns=budget_accumulator at=alert kind=threshold app=%s spend_usd=%.2f cap_usd=%.2f pct=%.0f\n",
				app, state.CurrentMonthSpendUsd, cfg.MonthlyCapUsd, cfg.AlertThresholdPercent)
			_ = p.EventSend("app:budget:threshold", structs.EventSendOptions{Data: map[string]string{
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
				"app":       app,
				"spend_usd": strconv.FormatFloat(state.CurrentMonthSpendUsd, 'f', 2, 64),
				"cap_usd":   strconv.FormatFloat(cfg.MonthlyCapUsd, 'f', 2, 64),
				"action":    cfg.AtCapAction,
			}})
		}
		return nil
	}
	return fmt.Errorf("failed to write budget state for %s after %d retries", app, budgetWriteConflictRetries)
}

// computeBudgetDelta walks pods in the app namespace and attributes cost
// over elapsed = now - lastTick. Returns (delta_usd, warnings, err).
func (p *Provider) computeBudgetDelta(app string, lastTick, now time.Time, adjustment float64) (float64, int, error) {
	if lastTick.IsZero() {
		lastTick = now
	}
	elapsed := now.Sub(lastTick).Hours()
	if elapsed <= 0 {
		return 0, 0, nil
	}
	if adjustment <= 0 {
		adjustment = 1.0
	}

	nodes, err := p.ListNodesFromInformer("")
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}
	nodeByName := map[string]*v1.Node{}
	for i := range nodes.Items {
		nodeByName[nodes.Items[i].Name] = &nodes.Items[i]
	}

	pods, err := p.ListPodsFromInformer(p.AppNamespace(app), "")
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	var delta float64
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

		fraction := dominantResourceFraction(pod, node, price)
		delta += price.OnDemandUsdPerHour * fraction * elapsed * adjustment
	}

	return delta, warnings, nil
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
// ReleasePromote and ServiceUpdate. Returns ErrConflict with guidance text
// when the breaker is tripped; nil otherwise.
func (p *Provider) budgetCircuitBreakerTripped(app string) error {
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
		"budget cap exceeded for app %s: spent $%.2f of $%.2f cap this month; run 'convox budget reset %s' to acknowledge and re-enable deploys",
		app, state.CurrentMonthSpendUsd, capUsd, app,
	)
}
