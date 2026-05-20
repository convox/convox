package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("budget show", "show the app's budget config and state", BudgetShow, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget set", "set the app's budget config", BudgetSet, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("monthly-cap", "", "monthly cap in USD (required)"),
			stdcli.IntFlag("alert-at", "", "alert threshold percent (default 80)"),
			stdcli.StringFlag("at-cap-action", "", "action at cap: alert-only (default), block-new-deploys, or auto-shutdown"),
			stdcli.StringFlag("pricing-adjustment", "", "price multiplier, e.g. 0.7 for 30% EDP discount (default 1.0)"),
			stdcli.StringFlag("ack-by", "", "DEPRECATED: ack_by is now derived from your JWT identity; flag will be rejected in 3.25.0"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget clear", "remove the app's budget config", BudgetClear, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("ack-by", "", "DEPRECATED: ack_by is now derived from your JWT identity; flag will be rejected in 3.25.0"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget reset", "acknowledge cap breach and re-enable deploys", BudgetReset, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.BoolFlag("force", "f", "skip interactive confirmation"),
			stdcli.BoolFlag("force-clear-cooldown", "", "also clear the 24h auto-restore flap cooldown (Admin role required)"),
			stdcli.StringFlag("ack-by", "", "DEPRECATED: ack_by is now derived from your JWT identity; flag will be rejected in 3.25.0"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget simulate-shutdown", "dry-run an auto-shutdown without modifying the app", BudgetSimulateShutdown, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget dismiss-recovery", "dismiss the sticky auto-shutdown recovery banner", BudgetDismissRecovery, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	// cited by ARMED banner and 409 breaker message; must exist as a real command
	register("budget cap raise", "raise the monthly cap (alias for `budget set --monthly-cap`)", BudgetCapRaise, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.StringFlag("monthly-cap-usd", "", "new monthly cap in USD (required)"),
			stdcli.StringFlag("monthly-cap", "", "alias for --monthly-cap-usd"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	}, WithCloud())
}

func BudgetShow(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	cfg, state, err := rack.AppBudgetGet(app)
	if err != nil {
		return wrapVersionGate(err, "budget caps")
	}

	if cfg == nil {
		fmt.Fprintf(c.Writer(), "no budget configured for %s\n", app)
		return nil
	}

	if state != nil && state.CircuitBreakerTripped {
		if ss, e := rack.ServiceList(app); e == nil {
			for i := range ss {
				if serviceHasKedaSurface(&ss[i]) {
					fmt.Fprintln(c.Writer(), kedaCapBypassBanner)
					break
				}
			}
		}
	}

	if shutdownState := safeShutdownStateGet(rack, app); shutdownState != nil {
		renderShutdownStateBanner(c.Writer(), app, cfg, shutdownState)
	}

	out := map[string]interface{}{
		"config": cfg,
		"state":  state,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Writer(), string(data))
	return nil
}

func renderShutdownStateBanner(w io.Writer, app string, cfg *structs.AppBudget, s *structs.AppBudgetShutdownState) {
	if s == nil {
		return
	}
	switch {
	case s.ShutdownAt != nil && !s.ShutdownAt.IsZero() && (s.RestoredAt == nil || s.RestoredAt.IsZero()) &&
		(s.FailedNotificationFiredAt == nil || s.FailedNotificationFiredAt.IsZero()):
		fmt.Fprintf(w, "[ACTIVE] Auto-shutdown ACTIVE for %s. %d services scaled to 0 at %s. Run `convox budget reset %s` to restore.\n",
			app, len(s.Services), s.ShutdownAt.UTC().Format("2006-01-02T15:04:05Z"), app)
	case s.RestoredAt != nil && !s.RestoredAt.IsZero():
		flap := ""
		if s.FlapSuppressedUntil != nil && !s.FlapSuppressedUntil.IsZero() {
			flap = fmt.Sprintf(" Cooldown until %s.", s.FlapSuppressedUntil.UTC().Format("2006-01-02T15:04:05Z"))
		}
		fmt.Fprintf(w, "[RECOVERED] Auto-shutdown RECOVERED for %s at %s.%s Run `convox budget dismiss-recovery %s` to clear banner.\n",
			app, s.RestoredAt.UTC().Format("2006-01-02T15:04:05Z"), flap, app)
	case s.FailedNotificationFiredAt != nil && !s.FailedNotificationFiredAt.IsZero():
		if s.FailureReason != "" {
			fmt.Fprintf(w, "[FAILED] Auto-shutdown FAILED for %s. Reason: %s. Run `convox budget reset %s` to clear state.\n", app, s.FailureReason, app)
		} else {
			fmt.Fprintf(w, "[FAILED] Auto-shutdown FAILED for %s. Run `convox budget reset %s` to clear state.\n", app, app)
		}
	case s.ArmedAt != nil && !s.ArmedAt.IsZero():
		notifyMin := s.NotifyBeforeMinutes
		if notifyMin <= 0 {
			notifyMin = structs.BudgetDefaultNotifyBeforeMinutes
		}
		_ = cfg
		fireAt := s.ArmedAt.Add(time.Duration(notifyMin) * time.Minute)
		fmt.Fprintf(w, "[ARMED] Auto-shutdown ARMED for %s. Services will scale to 0 at %s. Run `convox budget cap raise --monthly-cap-usd <higher> %s` or `convox budget reset %s` to abort.\n",
			app, fireAt.UTC().Format("2006-01-02T15:04:05Z"), app, app)
	}
}

func BudgetSet(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	capStr := c.String("monthly-cap")
	paStr := c.String("pricing-adjustment")
	alertAtRaw := c.Int("alert-at")
	actionRaw := c.String("at-cap-action")

	// pricing-adjustment alone is accepted without --monthly-cap (non-enforcement)
	if capStr == "" {
		if alertAtRaw != 0 || actionRaw != "" {
			return fmt.Errorf("--alert-at and --at-cap-action require --monthly-cap")
		}
		if paStr == "" {
			return fmt.Errorf("--monthly-cap or --pricing-adjustment is required")
		}
	}

	var capVal float64
	if capStr != "" {
		var err error
		capVal, err = strconv.ParseFloat(capStr, 64)
		if err != nil {
			return fmt.Errorf("--monthly-cap must be a number: %v", err)
		}
		if math.IsNaN(capVal) || math.IsInf(capVal, 0) {
			return fmt.Errorf("--monthly-cap must be a finite number")
		}
	}

	opts := structs.AppBudgetOptions{}

	if capStr != "" {
		alertAt := alertAtRaw
		if alertAt == 0 {
			alertAt = int(structs.BudgetDefaultAlertThresholdPercent)
		}

		action := actionRaw
		if action == "" {
			action = structs.BudgetDefaultAtCapAction
		}
		switch action {
		case structs.BudgetAtCapActionAlertOnly, structs.BudgetAtCapActionBlockNewDeploys, structs.BudgetAtCapActionAutoShutdown:
		default:
			return fmt.Errorf("--at-cap-action must be %q, %q, or %q",
				structs.BudgetAtCapActionAlertOnly, structs.BudgetAtCapActionBlockNewDeploys, structs.BudgetAtCapActionAutoShutdown)
		}

		if action == structs.BudgetAtCapActionAutoShutdown {
			fmt.Fprintln(c.Writer().Stderr,
				"WARNING: auto-shutdown will scale eligible services to 0 replicas at cap breach. "+
					"Verify your atCapWebhookUrl is configured (configured in convox.yml budget block) and your team is paged on :armed events. "+
					"Run 'convox budget simulate-shutdown <app>' to validate the configuration.")
			fmt.Fprintln(c.Writer().Stderr,
				"NOTE: services with KEDA idleReplicaCount: 0 will return to KEDA-driven scaling at restore "+
					"and may scale back to 0 if triggers are inactive. This is the user's KEDA config working as intended.")
		}

		opts.MonthlyCapUsd = &capStr
		opts.AlertThresholdPercent = &alertAt
		opts.AtCapAction = &action
	}

	// omission preserves the prior persisted value (partial-merge)
	if paStr != "" {
		paVal, err := strconv.ParseFloat(paStr, 64)
		if err != nil {
			return fmt.Errorf("--pricing-adjustment must be a number: %v", err)
		}
		if math.IsNaN(paVal) || math.IsInf(paVal, 0) {
			return fmt.Errorf("--pricing-adjustment must be a finite number")
		}
		opts.PricingAdjustment = &paStr
	}

	// warn if cap is already below current MTD spend (best-effort)
	if capStr != "" {
		if cost, err := rack.AppCost(app); err == nil && cost != nil && cost.SpendUsd > capVal {
			fmt.Fprintf(c.Writer().Stderr,
				"WARNING: --monthly-cap=$%.2f is below current month-to-date spend $%.2f. Cap will trip immediately on next accumulator tick.\n",
				capVal, cost.SpendUsd)
		}
	}

	ackBy := c.String("ack-by")
	explicit := ackBy != ""
	if !explicit {
		ackBy = currentActorIdentifier()
	}

	c.Startf("Setting budget for <app>%s</app>", app)
	if err := rack.AppBudgetSet(app, opts, ackBy); err != nil {
		return wrapVersionGate(err, "budget caps")
	}
	if explicit {
		fmt.Fprintln(c.Writer().Stderr, "WARNING: --ack-by is deprecated; ack_by is now derived from your JWT identity. Flag will be rejected in 3.25.0.")
	}
	return c.OK()
}

func BudgetClear(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	ackBy := c.String("ack-by")
	explicit := ackBy != ""
	if !explicit {
		ackBy = currentActorIdentifier()
	}

	c.Startf("Clearing budget for <app>%s</app>", app)
	if err := rack.AppBudgetClear(app, ackBy); err != nil {
		return wrapVersionGate(err, "budget caps")
	}
	if explicit {
		fmt.Fprintln(c.Writer().Stderr, "WARNING: --ack-by is deprecated; ack_by is now derived from your JWT identity. Flag will be rejected in 3.25.0.")
	}
	return c.OK()
}

func BudgetReset(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	if !c.Bool("force") {
		if !c.Reader().IsTerminal() {
			return fmt.Errorf("refusing to prompt for confirmation on non-interactive stdin; pass --force to proceed")
		}
		fmt.Fprintf(c.Writer(), "This will acknowledge the current spend and re-enable deploys for %s. Continue? [y/N]: ", app)
		scanner := bufio.NewScanner(c.Reader())
		scanner.Scan()
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("aborted")
		}
	}

	ackBy := c.String("ack-by")
	explicit := ackBy != ""
	if !explicit {
		ackBy = currentActorIdentifier()
	}

	c.Startf("Resetting budget for <app>%s</app>", app)
	if c.Bool("force-clear-cooldown") {
		if err := rack.AppBudgetResetWithOptions(app, ackBy, structs.AppBudgetResetOptions{ForceClearCooldown: true}); err != nil {
			return wrapVersionGate(err, "budget caps")
		}
	} else {
		if err := rack.AppBudgetReset(app, ackBy); err != nil {
			return wrapVersionGate(err, "budget caps")
		}
	}
	if explicit {
		fmt.Fprintln(c.Writer().Stderr, "WARNING: --ack-by is deprecated; ack_by is now derived from your JWT identity. Flag will be rejected in 3.25.0.")
	}
	return c.OK()
}

// safeShutdownStateGet wraps the SDK call with panic recovery for downgraded racks.
func safeShutdownStateGet(rack sdk.Interface, app string) (state *structs.AppBudgetShutdownState) {
	defer func() {
		if r := recover(); r != nil {
			state = nil
		}
	}()
	s, err := rack.AppBudgetShutdownStateGet(app)
	if err != nil {
		return nil
	}
	return s
}

func currentActorIdentifier() string {
	for _, env := range []string{"CONVOX_ACTOR", "USER", "USERNAME"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return "cli"
}

func BudgetSimulateShutdown(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)
	res, err := rack.AppBudgetSimulate(app)
	if err != nil {
		return wrapVersionGate(err, "budget simulate-shutdown")
	}
	if res == nil {
		fmt.Fprintf(c.Writer(), "no simulation result for app %s\n", app)
		return c.OK()
	}
	w := c.Writer()
	fmt.Fprintf(w, "Simulating auto-shutdown for %s...\n\n", app)
	fmt.Fprintln(w, "Configuration:")
	fmt.Fprintf(w, "  at_cap_action: %s\n", res.AtCapAction)
	fmt.Fprintf(w, "  webhook URL: %s\n", res.WebhookUrl)
	fmt.Fprintf(w, "  notify_before_minutes: %d\n", res.NotifyBeforeMinutes)
	fmt.Fprintf(w, "  shutdown_grace_period: %s\n", res.ShutdownGracePeriod)
	fmt.Fprintf(w, "  shutdown_order: %s\n", res.ShutdownOrder)
	fmt.Fprintf(w, "  recovery_mode: %s\n", res.RecoveryMode)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Eligibility:")
	sawEmDash := false
	for _, e := range res.Eligibility {
		if e.Eligible {
			rate, dashed := formatRateUsdPerHour(e.CostUsdPerHour)
			if dashed {
				sawEmDash = true
			}
			fmt.Fprintf(w, "  %s: ELIGIBLE -- replicas=%d, cost=%s/hr\n", e.Service, e.Replicas, rate)
		} else {
			fmt.Fprintf(w, "  %s: EXEMPT (%s)\n", e.Service, e.Reason)
		}
	}
	if sawEmDash {
		fmt.Fprintln(w, lowSpendFootnote)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Shutdown order (%s):\n", res.ShutdownOrder)
	for i, name := range res.WouldShutDownServices {
		fmt.Fprintf(w, "  %d. %s -- would scale to 0\n", i+1, name)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Estimated savings: $%.2f/hr\n\n", res.EstimatedCostSavedUsdPerHour)
	fmt.Fprintln(w, "Webhook payload sent (dry_run=true):")
	fmt.Fprintln(w, "  See app:budget:auto-shutdown:simulated event in your atCapWebhookUrl webhook delivery and rack log aggregation")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Status: SIMULATION COMPLETE. No changes made.")
	return nil
}

func BudgetDismissRecovery(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)
	ackBy := currentActorIdentifier()
	res, err := rack.AppBudgetDismissRecoveryWithResult(app, ackBy)
	if err != nil {
		return wrapVersionGate(err, "budget dismiss-recovery")
	}
	if res == nil {
		fmt.Fprintf(c.Writer(), "Banner dismissed for %s.\n", app)
		return nil
	}
	switch res.Status {
	case structs.BudgetDismissRecoveryStatusDismissed:
		fmt.Fprintf(c.Writer(), "Banner dismissed for %s.\n", app)
	case structs.BudgetDismissRecoveryStatusAlreadyDismissed:
		fmt.Fprintf(c.Writer(), "Banner already dismissed for %s.\n", app)
	case structs.BudgetDismissRecoveryStatusNoBanner:
		fmt.Fprintf(c.Writer(), "No recovery banner active for %s.\n", app)
	default:
		fmt.Fprintf(c.Writer(), "Banner status for %s: %s.\n", app, res.Status)
	}
	return nil
}

func BudgetCapRaise(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	capStr := c.String("monthly-cap-usd")
	if capStr == "" {
		capStr = c.String("monthly-cap")
	}
	if capStr == "" {
		return fmt.Errorf("--monthly-cap-usd is required")
	}
	capVal, err := strconv.ParseFloat(capStr, 64)
	if err != nil {
		return fmt.Errorf("--monthly-cap-usd must be a number: %v", err)
	}
	if math.IsNaN(capVal) || math.IsInf(capVal, 0) {
		return fmt.Errorf("--monthly-cap-usd must be a finite number")
	}

	opts := structs.AppBudgetOptions{
		MonthlyCapUsd: &capStr,
	}

	if cost, err := rack.AppCost(app); err == nil && cost != nil && cost.SpendUsd > capVal {
		fmt.Fprintf(c.Writer().Stderr,
			"WARNING: --monthly-cap-usd=$%.2f is below current month-to-date spend $%.2f. Cap will trip immediately on next accumulator tick.\n",
			capVal, cost.SpendUsd)
	}

	ackBy := currentActorIdentifier()

	c.Startf("Raising monthly cap for <app>%s</app>", app)
	if err := rack.AppBudgetSet(app, opts, ackBy); err != nil {
		return wrapVersionGate(err, "budget caps")
	}
	return c.OK()
}
