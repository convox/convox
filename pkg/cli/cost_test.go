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

		// Aggregate header is locked.
		require.Contains(t, res.Stdout, "APP")
		require.Contains(t, res.Stdout, "SPEND-USD")
		require.Contains(t, res.Stdout, "AS-OF")
		require.Contains(t, res.Stdout, "PRICING-SOURCE")

		// Aggregate row content.
		require.Contains(t, res.Stdout, "app1")
		require.Contains(t, res.Stdout, "$12.34")
		require.Contains(t, res.Stdout, "embedded")

		// Aggregate mode must NOT include per-service column headers.
		require.NotContains(t, res.Stdout, "GPU-HOURS")
		require.NotContains(t, res.Stdout, "MEM-GB-HOURS")
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
