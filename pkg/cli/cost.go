package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	// F-11 fix: register `cost` with WithCloud() so the command appears
	// in `convox cloud` listings, matching other admin-cloud commands.
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

// Cost renders the per-app cost breakdown returned by the AppCost API.
//
// Modes:
//   - default: per-service table (SERVICE | GPU-HOURS | CPU-HOURS | MEM-GB-HOURS | INSTANCE | SPEND-USD)
//   - --aggregate: single-row app totals (APP | SPEND-USD | AS-OF | PRICING-SOURCE)
//   - --format json: indented JSON of the raw *structs.AppCost (for jq consumption)
//
// Date-range flags --start / --end accept YYYY-MM-DD only; absent flags
// fall through to the API's current-month-to-date default.
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
		// F-27 fix: detect V2-rack lacks /apps/{app}/cost endpoint and
		// return a friendly migration hint instead of the generic 404.
		// Pattern matches isAppConfigUnsupported in env.go.
		if isCostUnsupported(err) {
			return fmt.Errorf("cost tracking requires a V3 rack. The current rack appears to be V2 — see https://docs.convox.com/management/cost-tracking for details on cost tracking availability")
		}
		return fmt.Errorf("failed to fetch cost for app %s: %v", appName, err)
	}

	if cost == nil {
		return fmt.Errorf("no cost data returned for app %s", appName)
	}

	// Apply client-side date-range filter when either bound is set. The
	// AppCost API does not currently accept a date range, so the CLI filters
	// the returned breakdown by AsOf rather than introducing a new API
	// surface.
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

// parseCostDateRange parses --start / --end into time.Time values. Empty
// flags return zero values so the caller can detect "not set".
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

// filterCostRange returns a copy of cost with breakdown rows filtered to
// the requested date window. The AppCost API does not currently accept
// date-range parameters, so the CLI applies the filter at the AppCost-level
// AsOf timestamp: if the snapshot's AsOf falls outside the requested window
// the breakdown is zeroed out (preserving metadata so users still see
// pricing-source context).
//
// When either bound is zero it is treated as open-ended on that side.
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
	// Treat --end as inclusive of the calendar day: AsOf must not fall
	// after end+24h.
	if !end.IsZero() && cost.AsOf.After(end.Add(24*time.Hour)) {
		out := *cost
		out.Breakdown = []structs.ServiceCostLine{}
		out.SpendUsd = 0
		return &out
	}
	return cost
}

func printCostBreakdown(c *stdcli.Context, cost *structs.AppCost) error {
	t := c.Table("SERVICE", "GPU-HOURS", "CPU-HOURS", "MEM-GB-HOURS", "INSTANCE", "SPEND-USD")
	for _, line := range cost.Breakdown {
		t.AddRow(
			line.Service,
			fmt.Sprintf("%.2f", line.GpuHours),
			fmt.Sprintf("%.2f", line.CpuHours),
			fmt.Sprintf("%.2f", line.MemGbHours),
			line.InstanceType,
			fmt.Sprintf("$%.2f", line.SpendUsd),
		)
	}
	return t.Print()
}

func printCostAggregate(c *stdcli.Context, appName string, cost *structs.AppCost) error {
	t := c.Table("APP", "SPEND-USD", "AS-OF", "PRICING-SOURCE")
	t.AddRow(
		appName,
		fmt.Sprintf("$%.2f", cost.SpendUsd),
		common.Ago(cost.AsOf),
		cost.PricingSource,
	)
	return t.Print()
}

func printCostJSON(c *stdcli.Context, cost *structs.AppCost) error {
	data, err := json.MarshalIndent(cost, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(c.Writer(), string(data))
	return nil
}

// isCostUnsupported detects errors from racks that don't have the AppCost
// endpoint (V2 racks). Same substring pattern as isAppConfigUnsupported.
//
// F-27 fix: V2 racks return a bare 404 from `/apps/{app}/cost` because the
// endpoint is V3-only. The CLI surfaces a friendly migration hint instead
// of leaking the raw "response status 404" error.
func isCostUnsupported(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "response status 404")
}
