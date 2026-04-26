package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
)

// capStatus is a CLI-only display value derived from per-app budget state +
// service list. It overlays a sub-state token on the existing STATUS column
// in `convox ps`, `convox services`, and the `convox scale --watch` table
// when the app's budget cap is breached. No new API surface is introduced.
//
// The struct is deliberately package-private — Set G's auto-shutdown work
// (3.24.6 follow-up) and Console's status surfacing (out of scope here)
// will derive their own equivalents.
type capStatus struct {
	// AtCap is true when the budget cap is breached AND the configured
	// AtCapAction is one we surface to the user (block-new-deploys today,
	// auto-shutdown when Set G lands). When false, no decoration runs.
	AtCap bool

	// AutoShutdown is true when AtCapAction == "auto-shutdown" — the value
	// Set G will write. Pre-Set-G, this is always false. The helper accepts
	// the value already so Set G's lander does not need to revisit this file.
	AutoShutdown bool

	// KedaServiceSet maps service-name -> has-keda-or-equivalent-autoscaler.
	// nil if ServiceList failed; missing entries default to false (plain
	// at-cap). Values come from a heuristic on Service.Autoscale.Enabled —
	// the user-facing concern is "an autoscaler can scale despite the cap"
	// and KEDA is the v1 implementation.
	KedaServiceSet map[string]bool

	// ArmedCountdownMinutes is the integer remaining minutes until
	// auto-shutdown :fired (per Set G v2 spec §16.3 + corrective scope).
	// Populated only when an active shutdown-state annotation has ArmedAt
	// set and ShutdownAt empty. -1 means "not currently armed". Values < 1
	// when armed clamp to 1m (the customer cannot see "armed-0m" — that
	// transitions to "fired" before the next render).
	ArmedCountdownMinutes int
}

// budgetCapStatus fetches the app's budget config + state and a service list,
// returning a capStatus suitable for decoration. Errors are logged to stderr
// (ns=cli_budget) and swallowed — a budget-API hiccup must never make
// `convox ps` worse for the customer.
func budgetCapStatus(rack sdk.Interface, appName string, stderr io.Writer) (capStatus, error) {
	cs, ok := budgetCapStatusBase(rack, appName, stderr)
	if !ok {
		return cs, nil
	}
	if ss, e := rack.ServiceList(appName); e == nil {
		populateKedaServiceSet(&cs, ss)
	} else {
		fmt.Fprintf(stderr, "ns=cli_budget at=service-list-error err=%q\n", e)
	}
	return cs, nil
}

// budgetCapStatusWithServices is the variant for callers that already have
// a Services slice in hand (Services, Scale watch). Avoids a redundant
// rack.ServiceList round-trip and the matching mock churn.
func budgetCapStatusWithServices(rack sdk.Interface, appName string, services structs.Services, stderr io.Writer) (capStatus, error) {
	cs, ok := budgetCapStatusBase(rack, appName, stderr)
	if !ok {
		return cs, nil
	}
	populateKedaServiceSet(&cs, services)
	return cs, nil
}

// budgetCapStatusBase fetches budget config + state and returns the base
// capStatus. Returns (cs, false) when no decoration should run (no budget,
// not breached, alert-only, fetch error). The boolean is preferred over
// an error here because the helper deliberately swallows API hiccups.
func budgetCapStatusBase(rack sdk.Interface, appName string, stderr io.Writer) (capStatus, bool) {
	cfg, state, err := rack.AppBudgetGet(appName)
	if err != nil {
		fmt.Fprintf(stderr, "ns=cli_budget at=fetch-error err=%q\n", err)
		return capStatus{}, false
	}
	if cfg == nil || state == nil || !state.CircuitBreakerTripped {
		return capStatus{}, false
	}
	// Surface only when AtCapAction is one of the cap-enforcing values.
	// alert-only does NOT surface — the customer asked us not to enforce.
	// F-8 fix: use the canonical constant from pkg/structs so any future
	// rename of the at-cap-action value lands here automatically.
	cs := capStatus{
		AtCap:                 cfg.AtCapAction == structs.BudgetAtCapActionBlockNewDeploys || cfg.AtCapAction == structs.BudgetAtCapActionAutoShutdown,
		AutoShutdown:          cfg.AtCapAction == structs.BudgetAtCapActionAutoShutdown,
		ArmedCountdownMinutes: -1,
	}
	if !cs.AtCap {
		return capStatus{}, false
	}
	// Compute armed countdown if shutdown-state annotation is fetchable.
	// Best-effort: a server that pre-dates the new endpoint or a missing
	// annotation both fall through with ArmedCountdownMinutes = -1. Only
	// auto-shutdown configs need this lookup — block-new-deploys never
	// arms, so skip the round-trip in the common case.
	if !cs.AutoShutdown {
		return cs, true
	}
	shutdownState := safeShutdownStateGetFromBudgetStatus(rack, appName, stderr)
	if shutdownState != nil {
		if shutdownState.ArmedAt != nil && !shutdownState.ArmedAt.IsZero() &&
			(shutdownState.ShutdownAt == nil || shutdownState.ShutdownAt.IsZero()) {
			// F-18 fix: read NotifyBeforeMinutes from the persisted state
			// so the STATUS countdown reflects the customer-configured
			// notify window. Older-rack state without the field falls
			// back to the 30-minute default.
			notifyMin := shutdownState.NotifyBeforeMinutes
			if notifyMin <= 0 {
				notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
			}
			fireAt := shutdownState.ArmedAt.Add(time.Duration(notifyMin) * time.Minute)
			remaining := time.Until(fireAt)
			countdown := int(remaining.Minutes())
			if countdown < 1 {
				countdown = 1
			}
			cs.ArmedCountdownMinutes = countdown
		}
	}
	return cs, true
}

// safeShutdownStateGetFromBudgetStatus mirrors safeShutdownStateGet in
// budget.go: a downgraded rack or an older mock SDK that did not
// register the new endpoint must NEVER crash `convox ps` / `services` /
// `scale --watch`. Panic recovery + nil return on any error.
func safeShutdownStateGetFromBudgetStatus(rack sdk.Interface, app string, stderr io.Writer) (state *structs.AppBudgetShutdownState) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "ns=cli_budget at=shutdown-state-fetch-recover err=%v\n", r)
			state = nil
		}
	}()
	s, err := rack.AppBudgetShutdownStateGet(app)
	if err != nil {
		fmt.Fprintf(stderr, "ns=cli_budget at=shutdown-state-fetch-error err=%q\n", err)
		return nil
	}
	return s
}

// populateKedaServiceSet fills cs.KedaServiceSet from a Services slice.
func populateKedaServiceSet(cs *capStatus, services structs.Services) {
	cs.KedaServiceSet = make(map[string]bool, len(services))
	for i := range services {
		cs.KedaServiceSet[services[i].Name] = serviceHasKedaSurface(&services[i])
	}
}

// serviceHasKedaSurface is the CLI-side heuristic for "this service has a
// KEDA-style autoscaler that may bypass block-new-deploys." In v1 the only
// autoscaler is KEDA (and the equivalent v2 surface), so the in-tree
// Service.Autoscale.Enabled flag is the closest signal exposed via the
// stable API. If a future autoscaler is added, this heuristic broadens
// automatically without breaking the wire format.
func serviceHasKedaSurface(s *structs.Service) bool {
	return s != nil && s.Autoscale != nil && s.Autoscale.Enabled
}

// decorateStatusForBudgetCap appends a short-form sub-state token to the
// pod's existing STATUS column when the app is at-cap. Token selection:
//
//	at-cap-keda  — service is KEDA-driven (autoscaler-bypass disclosure)
//	at-cap-auto  — Set G auto-shutdown configured
//	at-cap       — plain block-new-deploys
//
// Long forms (e.g. `at-cap (keda-managed)`) are reserved for the
// `convox budget show` banner and MUST NOT appear here — they would
// regress STATUS column width.
func decorateStatusForBudgetCap(podStatus, serviceName string, cs capStatus) string {
	if !cs.AtCap {
		return podStatus
	}
	sub := capSubStateToken(serviceName, cs)
	if podStatus == "" {
		return sub
	}
	return podStatus + " " + sub
}

// capSubStateToken returns the short-form sub-state token for a service.
// Precedence (most specific first):
//
//	armed-Nm     — auto-shutdown configured AND in armed window (countdown N min)
//	at-cap-keda  — service is KEDA-driven (autoscaler-bypass disclosure)
//	at-cap-auto  — Set G auto-shutdown configured (post-armed-window OR fired)
//	at-cap       — plain block-new-deploys
//
// KEDA presence wins over auto-shutdown when NOT armed (the bypass is the
// more surprising value); armed wins over both because the imminent scale-to-0
// is the most important thing the customer must see.
//
// STATUS column tokens are formally pinned by Set G v2 spec §16.4 "STATUS
// token formal enumeration" (MF-9 amendment). The four tokens are stable
// contract surface — column-fixed scripts and JSON parsers depend on the
// exact byte sequence. Precedence (top-down):
//
//	armed-Nm     — armed and counting down (most important; trust-critical)
//	at-cap-keda  — KEDA-bypass disclosure (more surprising than auto-shutdown)
//	at-cap-auto  — Set G auto-shutdown configured (post-armed-window OR fired)
//	at-cap       — plain block-new-deploys (existing 3.24.5)
//
// KEDA presence wins over auto-shutdown when NOT armed because the bypass is
// the more surprising value; armed wins over both because the imminent
// scale-to-0 is the most important thing the customer must see.
func capSubStateToken(serviceName string, cs capStatus) string {
	switch {
	case cs.AutoShutdown && cs.ArmedCountdownMinutes > 0:
		return fmt.Sprintf("armed-%dm", cs.ArmedCountdownMinutes)
	case cs.KedaServiceSet[serviceName]:
		return "at-cap-keda"
	case cs.AutoShutdown:
		return "at-cap-auto"
	default:
		return "at-cap"
	}
}

// kedaCapBypassBanner is the long-form disclosure shown by `convox budget
// show` when the app has at least one KEDA-driven service. Pinned by R3:
// exact text, multi-line OK, must wrap at 80 cols. DO NOT edit without
// updating the matching test in budget_status_test.go.
//
// #nosec G101 -- This is human-readable user-facing copy, not a credential.
const kedaCapBypassBanner = "KEDA-managed services may scale despite block-new-deploys\n" +
	"(v1 limitation; auto-shutdown closes gap in 3.24.6 — see release notes)"
