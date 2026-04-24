package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

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
			stdcli.StringFlag("at-cap-action", "", "action at cap: alert-only (default) or block-new-deploys"),
			stdcli.StringFlag("pricing-adjustment", "", "price multiplier, e.g. 0.7 for 30% EDP discount (default 1.0)"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget clear", "remove the app's budget config", BudgetClear, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("budget reset", "acknowledge cap breach and re-enable deploys", BudgetReset, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.BoolFlag("force", "f", "skip interactive confirmation"),
		},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})
}

func BudgetShow(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	cfg, state, err := rack.AppBudgetGet(app)
	if err != nil {
		return err
	}

	if cfg == nil {
		fmt.Fprintf(c.Writer(), "no budget configured for %s\n", app)
		return nil
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

func BudgetSet(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	capStr := c.String("monthly-cap")
	if capStr == "" {
		return fmt.Errorf("--monthly-cap is required")
	}
	capVal, err := strconv.ParseFloat(capStr, 64)
	if err != nil {
		return fmt.Errorf("--monthly-cap must be a number: %v", err)
	}
	if math.IsNaN(capVal) || math.IsInf(capVal, 0) {
		return fmt.Errorf("--monthly-cap must be a finite number")
	}

	alertAt := c.Int("alert-at")
	if alertAt == 0 {
		alertAt = int(structs.BudgetDefaultAlertThresholdPercent)
	}

	action := c.String("at-cap-action")
	if action == "" {
		action = structs.BudgetDefaultAtCapAction
	}
	switch action {
	case structs.BudgetAtCapActionAlertOnly, structs.BudgetAtCapActionBlockNewDeploys:
	default:
		return fmt.Errorf("--at-cap-action must be %q or %q", structs.BudgetAtCapActionAlertOnly, structs.BudgetAtCapActionBlockNewDeploys)
	}

	paStr := c.String("pricing-adjustment")
	if paStr == "" {
		paStr = strconv.FormatFloat(structs.BudgetDefaultPricingAdjustment, 'f', -1, 64)
	}
	paVal, err := strconv.ParseFloat(paStr, 64)
	if err != nil {
		return fmt.Errorf("--pricing-adjustment must be a number: %v", err)
	}
	if math.IsNaN(paVal) || math.IsInf(paVal, 0) {
		return fmt.Errorf("--pricing-adjustment must be a finite number")
	}

	opts := structs.AppBudgetOptions{
		MonthlyCapUsd:         &capStr,
		AlertThresholdPercent: &alertAt,
		AtCapAction:           &action,
		PricingAdjustment:     &paStr,
	}

	c.Startf("Setting budget for <app>%s</app>", app)
	if err := rack.AppBudgetSet(app, opts, currentActorIdentifier()); err != nil {
		return err
	}
	return c.OK()
}

func BudgetClear(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Arg(0)

	c.Startf("Clearing budget for <app>%s</app>", app)
	if err := rack.AppBudgetClear(app, currentActorIdentifier()); err != nil {
		return err
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

	ackBy := currentActorIdentifier()

	c.Startf("Resetting budget for <app>%s</app>", app)
	if err := rack.AppBudgetReset(app, ackBy); err != nil {
		return err
	}
	return c.OK()
}

func currentActorIdentifier() string {
	for _, env := range []string{"CONVOX_ACTOR", "USER", "USERNAME"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return "cli"
}
