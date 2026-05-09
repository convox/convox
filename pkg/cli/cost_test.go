package cli_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

// fxAppCost returns an AppCost fixture with two services in the breakdown.
// Use this when a test wants to assert per-service rendering.
func fxAppCost() *structs.AppCost {
	return &structs.AppCost{
		App:                 "app1",
		MonthStart:          time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		AsOf:                time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		SpendUsd:            12.34,
		PricingSource:       "embedded",
		PricingTableVersion: "2026-01",
		PricingAdjustment:   1.0,
		Breakdown: []structs.ServiceCostLine{
			{
				Service:      "web",
				GpuHours:     0,
				CpuHours:     24,
				MemGbHours:   48,
				InstanceType: "m5.large",
				SpendUsd:     2.34,
			},
			{
				Service:      "trainer",
				GpuHours:     6,
				CpuHours:     6,
				MemGbHours:   24,
				InstanceType: "p3.2xlarge",
				SpendUsd:     10.00,
			},
		},
	}
}

// fxAppCostWithVariants returns an AppCost where one service is split
// across capacity types (web on m5.large on-demand AND m5.large spot).
// Replicas reflect a realistic mixed-placement deployment so the count
// column is exercised end-to-end.
func fxAppCostWithVariants() *structs.AppCost {
	c := fxAppCost()
	c.VariantBreakdown = []structs.ServiceVariantCostLine{
		{Service: "trainer", InstanceType: "p3.2xlarge", CapacityType: "on-demand", SpendUsd: 10.00, Replicas: 1},
		{Service: "web", InstanceType: "m5.large", CapacityType: "on-demand", SpendUsd: 1.40, Replicas: 3},
		{Service: "web", InstanceType: "m5.large", CapacityType: "spot", SpendUsd: 0.94, Replicas: 2},
	}
	return c
}

// fxAppCostEmpty returns an AppCost with cost-tracking enabled but zero
// services in the breakdown (no spend yet).
func fxAppCostEmpty() *structs.AppCost {
	return &structs.AppCost{
		App:                 "app1",
		MonthStart:          time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		AsOf:                time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC),
		SpendUsd:            0,
		Breakdown:           []structs.ServiceCostLine{},
		PricingSource:       "embedded",
		PricingTableVersion: "2026-01",
		PricingAdjustment:   1.0,
	}
}

// --- Happy path -----------------------------------------------------------

func TestCost_DefaultMode_RendersServiceByServiceTable(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		// Header line is locked: this is the tooling-stability contract.
		require.Contains(t, res.Stdout, "SERVICE")
		require.Contains(t, res.Stdout, "GPU-HOURS")
		require.Contains(t, res.Stdout, "CPU-HOURS")
		require.Contains(t, res.Stdout, "MEM-GB-HOURS")
		require.Contains(t, res.Stdout, "INSTANCE")
		require.Contains(t, res.Stdout, "SPEND-USD")

		// Per-service rows.
		require.Contains(t, res.Stdout, "web")
		require.Contains(t, res.Stdout, "trainer")
		require.Contains(t, res.Stdout, "m5.large")
		require.Contains(t, res.Stdout, "p3.2xlarge")
		require.Contains(t, res.Stdout, "$2.34")
		require.Contains(t, res.Stdout, "$10.00")
	})
}

func TestCost_AggregateFlag_RendersOneRow(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1 --aggregate", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		// Aggregate header is locked. Column name PRICING-SOURCE is
		// retained so existing scripts that grep this column header
		// keep working; the cell value is the user-facing pricing-
		// table label rather than the raw rack token.
		require.Contains(t, res.Stdout, "APP")
		require.Contains(t, res.Stdout, "SPEND-USD")
		require.Contains(t, res.Stdout, "AS-OF")
		require.Contains(t, res.Stdout, "PRICING-SOURCE")

		// Aggregate row content. The pricing-table vintage is
		// surfaced verbatim; the on-demand-static-table token is
		// dropped from the user-facing string.
		require.Contains(t, res.Stdout, "app1")
		require.Contains(t, res.Stdout, "$12.34")
		require.Contains(t, res.Stdout, "pricing-table:2026-01",
			"aggregate row must surface the vintage label, not the raw rack token")
		require.NotContains(t, res.Stdout, "on-demand-static-table",
			"raw rack-side token must NOT appear in user-facing aggregate output")

		// Aggregate mode must NOT include per-service column headers.
		require.NotContains(t, res.Stdout, "GPU-HOURS")
		require.NotContains(t, res.Stdout, "MEM-GB-HOURS")
	})
}

// TestFormatPricingTableLabel locks the helper that translates the
// rack-side AppCost.PricingSource token into a user-facing label.
// Mirrors the web-side helper at web/src/utils/pricingLabel.js — keep
// both implementations in sync.
func TestFormatPricingTableLabel(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// Vintage present: surface only the vintage; raw token is
		// dropped from the user-facing label.
		fx := fxAppCost()
		fx.PricingSource = "on-demand-static-table"
		fx.PricingTableVersion = "2026-04-29"
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1 --aggregate", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "pricing-table:2026-04-29")
		require.NotContains(t, res.Stdout, "on-demand-static-table",
			"raw rack token must never appear in user-facing output")
	})
}

// TestFormatPricingTableLabel_NoVintageSurfacesUnknown asserts the
// helper falls back to the literal "pricing-table:unknown" string when
// neither a vintage NOR a non-canonical source is present. Pre-3.24.6
// racks emit no vintage; we still want a stable label so users see
// something rather than a blank cell.
func TestFormatPricingTableLabel_NoVintageSurfacesUnknown(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.PricingSource = "on-demand-static-table"
		fx.PricingTableVersion = ""
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1 --aggregate", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "pricing-table:unknown")
		require.NotContains(t, res.Stdout, "on-demand-static-table",
			"raw rack token must never appear in user-facing output")
	})
}

func TestCost_FormatJson_EmitsIndentedJSON(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1 --format json", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		expected, err := json.MarshalIndent(fx, "", "  ")
		require.NoError(t, err)
		// Output is the marshalled JSON followed by a trailing newline.
		require.Equal(t, string(expected)+"\n", res.Stdout)
	})
}

// TestCost_DateRangeDefaultsToMonthStart asserts that without --start/--end
// the SDK is invoked with the app name only (no extra range parameters).
// The current AppCost API has signature AppCost(app string), so passing
// nothing for date filters is the contract.
func TestCost_DateRangeDefaultsToMonthStart(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		// AssertExpectations in testClient confirms AppCost was called with
		// "app1" and no extra arguments — i.e. we did not silently introduce
		// new SDK parameters.
	})
}

// TestCost_DateRangeExplicit_PassesThrough exercises --start / --end
// parsing. The AppCost API does not yet accept date-range parameters, so
// the CLI applies a client-side filter on AsOf. With AsOf inside the
// requested window, the breakdown should render unchanged.
func TestCost_DateRangeExplicit_PassesThrough(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1 --start 2026-04-01 --end 2026-04-30", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		// AsOf 2026-04-24 falls inside [2026-04-01, 2026-04-30+24h], so the
		// breakdown is preserved.
		require.Contains(t, res.Stdout, "web")
		require.Contains(t, res.Stdout, "trainer")
	})
}

// TestCost_TableStyleMatchesReleases asserts that cost (default mode) and
// releases use the same c.Table(...) rendering: both produce
// space-separated columns with a header line followed by data rows. We
// inspect the rendered output for tab-free space-padded layout.
func TestCost_TableStyleMatchesReleases(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		// stdcli c.Table() renders with space padding, no tabs.
		require.NotContains(t, res.Stdout, "\t", "cost table must not use tab characters")
		// Header line must contain at least two consecutive spaces between
		// columns (table padding).
		lines := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
		require.NotEmpty(t, lines)
		require.Contains(t, lines[0], "  ", "header should have space-padded columns")
	})
}

// --- Error path -----------------------------------------------------------

func TestCost_AppCostFails_ReturnsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(nil, fmt.Errorf("rack offline"))

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "failed to fetch cost for app app1")
		require.Contains(t, res.Stderr, "rack offline")
	})
}

func TestCost_InvalidStartDate_ReturnsClearError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// AppCost not called: validation rejects before SDK call.
		res, err := testExecute(e, "cost -a app1 --start 04/01/2026", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--start must be YYYY-MM-DD")
	})
}

func TestCost_InvalidEndDate_ReturnsClearError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// AppCost not called: validation rejects before SDK call.
		res, err := testExecute(e, "cost -a app1 --end 2026-04-32", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--end must be YYYY-MM-DD")
	})
}

func TestCost_StartAfterEnd_RejectsWithError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// AppCost not called: validation rejects before SDK call.
		res, err := testExecute(e, "cost -a app1 --start 2026-04-15 --end 2026-04-01", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "--start must not be after --end")
	})
}

func TestCost_InvalidFormat_RejectsWithError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// AppCost not called: format validation rejects before SDK call.
		res, err := testExecute(e, "cost -a app1 --format yaml", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, `--format must be "table" or "json"`)
	})
}

// --- Negative / regression guard ------------------------------------------

// TestCost_NoBudgetSet_DoesNotFail confirms cost tracking is independent of
// budget configuration: AppCost can return a valid breakdown even when no
// budget is set, and Cost should render it without error.
func TestCost_NoBudgetSet_DoesNotFail(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "web")
	})
}

// TestCost_EmptyBreakdown_RendersHeaderOnly: an AppCost with zero
// breakdown rows must still render the header without panicking.
func TestCost_EmptyBreakdown_RendersHeaderOnly(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostEmpty(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "SERVICE")
		require.Contains(t, res.Stdout, "SPEND-USD")
		// No data rows — count of newlines should match header-only rendering.
		dataLines := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
		require.Equal(t, 1, len(dataLines), "expected header-only output, got: %q", res.Stdout)
	})
}

// TestCost_OutputFormatStability_ServiceColumnNamesUnchanged locks the
// exact column-header line for the default mode. Downstream tooling
// (scripts, dashboards) parses these column names; changing them is a
// breaking change. If this test fails, you have changed the table contract.
func TestCost_OutputFormatStability_ServiceColumnNamesUnchanged(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		lines := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
		require.NotEmpty(t, lines)
		header := lines[0]
		// Locked column names, in order, with stdcli's two-space padding.
		require.True(t, strings.HasPrefix(header, "SERVICE"), "header must start with SERVICE: %q", header)
		// Order must match the locked contract.
		require.Less(t, strings.Index(header, "SERVICE"), strings.Index(header, "GPU-HOURS"))
		require.Less(t, strings.Index(header, "GPU-HOURS"), strings.Index(header, "CPU-HOURS"))
		require.Less(t, strings.Index(header, "CPU-HOURS"), strings.Index(header, "MEM-GB-HOURS"))
		require.Less(t, strings.Index(header, "MEM-GB-HOURS"), strings.Index(header, "INSTANCE"))
		require.Less(t, strings.Index(header, "INSTANCE"), strings.Index(header, "SPEND-USD"))
	})
}

// TestCost_Subcommand_IntegrationViaRegister exercises the register("cost",
// ...) plumbing by invoking the command through the engine the same way
// any external caller would. A typo in the register call would surface
// here as "cost: unknown command".
func TestCost_Subcommand_IntegrationViaRegister(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "register('cost', ...) plumbing failed; stderr: %s", res.Stderr)
		// Smoke: command produced output.
		require.NotEmpty(t, res.Stdout)
	})
}

// --- em-dash low-rate format -----------
//
// Locks the format-helper threshold and the disambiguation footnote
// behavior shared by `convox cost` and `convox budget simulate-shutdown`.
// Helper is internal to the cli package — exercised here through the
// public `cost` command so we keep the assertions tied to user-visible
// output rather than the helper signature.

// C1: TestCostFormat_LowRateAsEmDash — well below 0.001 threshold renders
// as em-dash; SPEND-USD column shows "—" instead of "$0.00".
func TestCostFormat_LowRateAsEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{{
			Service:      "web",
			GpuHours:     0,
			CpuHours:     1,
			MemGbHours:   1,
			InstanceType: "m5.large",
			SpendUsd:     0.0005,
		}}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "—", "low-rate row must render em-dash, not $0.00")
		require.NotContains(t, strings.SplitN(res.Stdout, "\n", 2)[1], "$0.00",
			"the data row must NOT render $0.00 alongside the em-dash") // header line excluded
	})
}

// C2: TestCostFormat_NormalRateNoEmDash — non-trivial spend renders as
// $X.XX with no footnote.
func TestCostFormat_NormalRateNoEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{{
			Service:      "web",
			InstanceType: "m5.large",
			SpendUsd:     1.23,
		}}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "$1.23")
		require.NotContains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must NOT appear when no row used em-dash")
	})
}

// C3: TestCostFormat_ExplicitZero — exact zero stays as "$0.00", NOT
// em-dashed (zero is meaningful user state).
func TestCostFormat_ExplicitZero(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{{
			Service:      "web",
			InstanceType: "m5.large",
			SpendUsd:     0.0,
		}}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "$0.00",
			"explicit zero must render as $0.00 — em-dash threshold is strictly greater than 0")
		require.NotContains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must NOT appear for exact-zero rows")
	})
}

// C4: TestCostFormat_ThresholdBoundary — at-threshold (0.001 exactly)
// renders as $0.00 because the helper uses strict less-than.
func TestCostFormat_ThresholdBoundary(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{{
			Service:      "web",
			InstanceType: "m5.large",
			SpendUsd:     0.001,
		}}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "$0.00",
			"threshold boundary 0.001 must render as $0.00 — strictly less-than gating")
		require.NotContains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must NOT appear at the threshold")
	})
}

// C5: TestPrintCostBreakdown_FootnoteAppearsWhenAnyEmDash — multi-row
// table where one row triggers em-dash AND a normal row coexists.
// Footnote prints once below the table.
func TestPrintCostBreakdown_FootnoteAppearsWhenAnyEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{
			{Service: "web", InstanceType: "m5.large", SpendUsd: 0.0005},
			{Service: "trainer", InstanceType: "p3.2xlarge", SpendUsd: 2.50},
		}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "—",
			"low-rate row must render em-dash")
		require.Contains(t, res.Stdout, "$2.50",
			"normal-rate row must render as cents")
		require.Contains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must appear when any row used the em-dash")
		// Footnote should appear AFTER the table (below the trainer row).
		require.Less(t, strings.Index(res.Stdout, "$2.50"), strings.Index(res.Stdout, "low-spend rates"),
			"footnote must appear below the table rows")
	})
}

// C6: TestPrintCostBreakdown_NoFootnoteWhenNoEmDash — regression guard
// against spurious footnote noise on tables with all rows above threshold.
func TestPrintCostBreakdown_NoFootnoteWhenNoEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.Breakdown = []structs.ServiceCostLine{
			{Service: "web", InstanceType: "m5.large", SpendUsd: 0.05},
			{Service: "trainer", InstanceType: "p3.2xlarge", SpendUsd: 2.50},
		}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.NotContains(t, res.Stdout, "low-spend rates rounded to —",
			"footnote must NOT appear when no row used em-dash")
		require.Contains(t, res.Stdout, "$0.05")
		require.Contains(t, res.Stdout, "$2.50")
	})
}

// --- Per-variant rendering ---------------------------------------------------
//
// VariantBreakdown is the (service, instance-type, capacity-type) projection
// of the same accumulated dollars surfaced by Breakdown. When the rack
// emits VariantBreakdown rows, the CLI renders one row per variant and
// adds a CAPACITY column. Older racks (3.24.5 and earlier) emit empty
// VariantBreakdown — the CLI falls back to the legacy aggregated table.

// V1: TestCost_WithVariants_RendersCapacityColumn — happy path: a rack
// emitting VariantBreakdown produces a CAPACITY column with on-demand /
// spot values.
func TestCost_WithVariants_RendersCapacityColumn(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostWithVariants(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		require.Contains(t, res.Stdout, "CAPACITY",
			"variant-aware output must surface a CAPACITY column header")
		require.Contains(t, res.Stdout, "on-demand", "on-demand variant must render")
		require.Contains(t, res.Stdout, "spot", "spot variant must render")

		// One row per variant — web appears twice (on-demand $1.40 + spot $0.94).
		require.Contains(t, res.Stdout, "$1.40")
		require.Contains(t, res.Stdout, "$0.94")
		// Trainer is single-variant and still appears.
		require.Contains(t, res.Stdout, "$10.00")
	})
}

// V2: TestCost_WithoutVariants_RendersLegacyTable — pre-3.24.6 rack
// emits VariantBreakdown empty/nil. CLI falls back to the existing
// aggregated columns; no CAPACITY column appears.
func TestCost_WithoutVariants_RendersLegacyTable(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// fxAppCost has no VariantBreakdown set.
		i.On("AppCost", "app1").Return(fxAppCost(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		require.NotContains(t, res.Stdout, "CAPACITY",
			"legacy table must NOT include a CAPACITY column when rack emits no variant data")
		require.Contains(t, res.Stdout, "GPU-HOURS", "legacy table keeps original columns")
	})
}

// V3: TestCost_WithVariants_SpotLegendPresent — variant-mode output
// includes a one-line spot legend so users understand the discount.
func TestCost_WithVariants_SpotLegendPresent(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostWithVariants(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "spot",
			"spot rows render with the literal capacity value")
		// The legend mentions the discount mechanism so first-time users
		// understand what they're seeing. Locked phrasing helps tooling
		// that scrapes for the marker.
		require.Contains(t, res.Stdout, "Spot pricing",
			"variant-mode output must include the spot-discount legend")
	})
}

// V4: TestCost_AggregateMode_IgnoresVariants — --aggregate continues to
// produce the single-row APP / SPEND-USD / AS-OF / PRICING-SOURCE
// summary regardless of variant data. Variant breakdown is per-row
// detail, not aggregate context.
func TestCost_AggregateMode_IgnoresVariants(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostWithVariants(), nil)

		res, err := testExecute(e, "cost -a app1 --aggregate", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "APP")
		require.NotContains(t, res.Stdout, "CAPACITY",
			"aggregate row is single-line summary; CAPACITY belongs to per-row detail")
	})
}

// V5: TestCost_FormatJson_IncludesVariantBreakdown — JSON output emits
// the variant_breakdown field verbatim so downstream tooling (jq,
// scripts) can consume it without parsing tables.
func TestCost_FormatJson_IncludesVariantBreakdown(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCostWithVariants()
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1 --format json", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		require.Contains(t, res.Stdout, `"variant-breakdown"`,
			"JSON output must surface the variant-breakdown field for jq consumers")
		require.Contains(t, res.Stdout, `"capacity-type": "spot"`,
			"capacity-type field must serialize per-row")
	})
}

// V6: TestCost_VariantTable_UnknownCapacityRendered — variant rows where
// detection failed render the CAPACITY column as "unknown" verbatim
// (not "—" or blank). Operators must see the actual signal value so
// they know whether a node is genuinely unlabeled vs. simply elided.
func TestCost_VariantTable_UnknownCapacityRendered(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.VariantBreakdown = []structs.ServiceVariantCostLine{
			{Service: "worker", InstanceType: "t3.large", CapacityType: "unknown", SpendUsd: 0.42},
		}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "unknown",
			"unknown-capacity rows render the literal value so operators see detection failed")
	})
}

// TestCost_VariantTable_RendersReplicasColumn asserts that the
// ACTIVE-REPLICAS column is present and populated when the rack emits
// pod counts on each variant. Heterogeneous services display
// "3" / "2" pods so users can audit where their replicas landed.
// The "ACTIVE" prefix communicates that 0 means "no pods running on
// this variant right now" — a row may persist with 0 if pods migrated
// off that (instance-type, capacity-type) combination during the
// current accumulation window.
func TestCost_VariantTable_RendersReplicasColumn(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostWithVariants(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)

		require.Contains(t, res.Stdout, "ACTIVE-REPLICAS",
			"variant-aware output must surface an ACTIVE-REPLICAS column header")
		// Pod counts from fxAppCostWithVariants: trainer=1, web-od=3, web-sp=2.
		require.Contains(t, res.Stdout, "3", "web on-demand has 3 replicas in fixture")
		require.Contains(t, res.Stdout, "2", "web spot has 2 replicas in fixture")
	})
}

// TestCost_VariantTable_AccumulationNotePresent asserts that the
// per-(instance-type, capacity-type) accumulation note prints below
// the variant table so users understand why a row may show 0 active
// replicas. Locked phrasing helps tooling that scrapes for the marker.
func TestCost_VariantTable_AccumulationNotePresent(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppCost", "app1").Return(fxAppCostWithVariants(), nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "Cost accumulates per (instance-type, capacity-type)",
			"variant-mode output must include the per-variant accumulation note")
		require.Contains(t, res.Stdout, "0 active replicas",
			"accumulation note must explain the 0-active-replicas case")
	})
}

// TestCost_VariantTable_ZeroReplicasRendersEmDash asserts that the
// REPLICAS column emits an em-dash when the rack predates pod-count
// tracking (Replicas serializes as 0). Zero is indistinguishable from
// "no data yet" on the wire so the renderer leans toward em-dash to
// avoid implying "0 pods running" — that case (a service with spend
// but no pods) only happens transiently mid-tick and is better
// communicated as "data not yet captured".
func TestCost_VariantTable_ZeroReplicasRendersEmDash(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.VariantBreakdown = []structs.ServiceVariantCostLine{
			// Replicas: 0 (default) — pre-3.24.6 racks emit nothing.
			{Service: "web", InstanceType: "m5.large", CapacityType: "on-demand", SpendUsd: 1.40},
		}
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "stderr: %s", res.Stderr)
		require.Contains(t, res.Stdout, "—",
			"zero-replicas rows must render an em-dash placeholder")
	})
}
