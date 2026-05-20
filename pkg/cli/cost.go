package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/billing"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("cost", "show cost breakdown for an app", Cost, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagRack,
			stdcli.BoolFlag("aggregate", "", "show app-level total instead of per-service breakdown"),
			stdcli.StringFlag("start", "", "include only spend from this date onward (YYYY-MM-DD)"),
			stdcli.StringFlag("end", "", "include only spend up to this date (YYYY-MM-DD)"),
			stdcli.StringFlag("format", "", `output format: "table" (default) or "json"`),
		},
		Validate: stdcli.Args(0),
	}, WithCloud())
}

func Cost(rack sdk.Interface, c *stdcli.Context) error {
	appName := app(c)

	start, end, err := parseCostDateRange(c)
	if err != nil {
		return err
	}

	format := c.String("format")
	if format == "" {
		format = "table"
	}
	if format != "table" && format != "json" {
		return fmt.Errorf(`--format must be "table" or "json"`)
	}

	cost, err := rack.AppCost(appName)
	if err != nil {
		if isRackVersionGated(err) {
			return fmt.Errorf("cost tracking requires rack version 3.24.6 or later (V2 racks use a separate cost-tracking surface). See https://docs.convox.com/management/cost-tracking")
		}
		return fmt.Errorf("failed to fetch cost for app %s: %v", appName, err)
	}

	if cost == nil {
		return fmt.Errorf("no cost data returned for app %s", appName)
	}

	// client-side filter — API does not accept date range yet
	if !start.IsZero() || !end.IsZero() {
		cost = filterCostRange(cost, start, end)
	}

	if format == "json" {
		return printCostJSON(c, cost)
	}
	if c.Bool("aggregate") {
		return printCostAggregate(c, appName, cost)
	}
	return printCostBreakdown(c, cost)
}

func parseCostDateRange(c *stdcli.Context) (time.Time, time.Time, error) {
	var start, end time.Time
	if v := c.String("start"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--start must be YYYY-MM-DD")
		}
		start = t
	}
	if v := c.String("end"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("--end must be YYYY-MM-DD")
		}
		end = t
	}
	if !start.IsZero() && !end.IsZero() && end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("--start must not be after --end")
	}
	return start, end, nil
}

func filterCostRange(cost *structs.AppCost, start, end time.Time) *structs.AppCost {
	if cost == nil {
		return nil
	}
	if cost.AsOf.IsZero() {
		return cost
	}
	if !start.IsZero() && cost.AsOf.Before(start) {
		out := *cost
		out.Breakdown = []structs.ServiceCostLine{}
		out.SpendUsd = 0
		return &out
	}
	// --end is inclusive of the calendar day
	if !end.IsZero() && cost.AsOf.After(end.Add(24*time.Hour)) {
		out := *cost
		out.Breakdown = []structs.ServiceCostLine{}
		out.SpendUsd = 0
		return &out
	}
	return cost
}

const lowSpendFootnote = "— : low-spend rates rounded to —; see Spend column for actual cost."

// Rates below $0.001 render as em-dash to avoid misleading "$0.00".
func formatRateUsdPerHour(rate float64) (string, bool) {
	if rate > 0 && rate < 0.001 {
		return "—", true
	}
	return fmt.Sprintf("$%.2f", rate), false
}

const spotLegend = `Spot pricing applies a discount automatically when nodes are provisioned via Karpenter or an EKS spot ASG. Capacity "unknown" means the node carried neither label.`

const accumulationNote = `Cost accumulates per (instance-type, capacity-type) combination across the month. A row may show 0 active replicas if pods previously ran on that variant but have since migrated or been removed.`

const trackingDisabledNotice = `Cost tracking is disabled on this rack. Values shown are the most-recent persisted snapshot and may be empty or stale. To enable: convox rack params set cost_tracking_enable=true`

func printCostBreakdown(c *stdcli.Context, cost *structs.AppCost) error {
	if len(cost.VariantBreakdown) > 0 {
		return printCostVariantBreakdown(c, cost)
	}
	if !cost.TrackingEnabled {
		fmt.Fprintln(c.Writer(), trackingDisabledNotice)
	}
	t := c.Table("SERVICE", "GPU-HOURS", "CPU-HOURS", "MEM-GB-HOURS", "INSTANCE", "SPEND-USD")
	sawEmDash := false
	for _, line := range cost.Breakdown {
		spend, dashed := formatRateUsdPerHour(line.SpendUsd)
		if dashed {
			sawEmDash = true
		}
		t.AddRow(
			line.Service,
			fmt.Sprintf("%.2f", line.GpuHours),
			fmt.Sprintf("%.2f", line.CpuHours),
			fmt.Sprintf("%.2f", line.MemGbHours),
			line.InstanceType,
			spend,
		)
	}
	if err := t.Print(); err != nil {
		return err
	}
	if sawEmDash {
		fmt.Fprintln(c.Writer(), lowSpendFootnote)
	}
	return nil
}

func printCostVariantBreakdown(c *stdcli.Context, cost *structs.AppCost) error {
	if !cost.TrackingEnabled {
		fmt.Fprintln(c.Writer(), trackingDisabledNotice)
	}
	t := c.Table("SERVICE", "INSTANCE", "CAPACITY", "ACTIVE-REPLICAS", "SPEND-USD")
	sawEmDash := false
	var total float64
	for _, line := range cost.VariantBreakdown {
		spend, dashed := formatRateUsdPerHour(line.SpendUsd)
		if dashed {
			sawEmDash = true
		}
		total += line.SpendUsd
		replicas := "—"
		if line.Replicas > 0 {
			replicas = fmt.Sprintf("%d", line.Replicas)
		}
		t.AddRow(
			line.Service,
			line.InstanceType,
			line.CapacityType,
			replicas,
			spend,
		)
	}
	if err := t.Print(); err != nil {
		return err
	}
	fmt.Fprintf(c.Writer(), "TOTAL: $%.2f\n", total)
	fmt.Fprintln(c.Writer(), accumulationNote)
	fmt.Fprintln(c.Writer(), spotLegend)
	if sawEmDash {
		fmt.Fprintln(c.Writer(), lowSpendFootnote)
	}
	return nil
}

func printCostAggregate(c *stdcli.Context, appName string, cost *structs.AppCost) error {
	if !cost.TrackingEnabled {
		fmt.Fprintln(c.Writer(), trackingDisabledNotice)
	}
	t := c.Table("APP", "SPEND-USD", "AS-OF", "PRICING-SOURCE")
	t.AddRow(
		appName,
		fmt.Sprintf("$%.2f", cost.SpendUsd),
		common.Ago(cost.AsOf),
		formatPricingTableLabel(cost.PricingSource, cost.PricingTableVersion),
	)
	return t.Print()
}

// Surfaces pricing-table vintage, not the raw wire token which misleads spot users.
func formatPricingTableLabel(pricingSource, pricingTableVersion string) string {
	vintage := strings.TrimSpace(pricingTableVersion)
	if vintage != "" {
		return fmt.Sprintf("pricing-table:%s", vintage)
	}
	source := strings.TrimSpace(pricingSource)
	if source != "" && source != billing.PricingSourceStaticTable {
		return fmt.Sprintf("pricing-table:%s", source)
	}
	return "pricing-table:unknown"
}

func printCostJSON(c *stdcli.Context, cost *structs.AppCost) error {
	data, err := json.MarshalIndent(cost, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Writer(), string(data))
	return nil
}
