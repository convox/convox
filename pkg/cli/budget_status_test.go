package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

// fxAppBudgetCap returns a budget config in the cap-enforcing state.
func fxAppBudgetCap(action string) *structs.AppBudget {
	b := fxAppBudget()
	b.AtCapAction = action
	return b
}

// fxAppBudgetStateTripped returns a budget state with circuit breaker tripped.
func fxAppBudgetStateTripped() *structs.AppBudgetState {
	s := fxAppBudgetState()
	s.CircuitBreakerTripped = true
	return s
}

// fxServiceKedaEnabled returns a Service with Autoscale.Enabled = true,
// the CLI's KEDA-bypass-disclosure heuristic.
func fxServiceKedaEnabled(name string) structs.Service {
	gpu := 70
	s := structs.Service{
		Name:      name,
		Domain:    "domain",
		Autoscale: &structs.ServiceAutoscaleState{Enabled: true, GpuThreshold: &gpu},
	}
	return s
}

// fxServicePlain returns a Service with no autoscale.
func fxServicePlain(name string) structs.Service {
	return structs.Service{Name: name, Domain: "domain"}
}

// ============================================================================
// Happy path
// ============================================================================

// TestDecorateStatus_AtCapPlainNonKeda_AppendsAtCap covers the most common
// case: budget block-new-deploys is breached, the service is NOT KEDA-driven,
// Set G auto-shutdown is NOT configured.
func TestDecorateStatus_AtCapPlainNonKeda_AppendsAtCap(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("name")}, nil).Maybe()

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// fxProcess has Status="running" and Name="name"
		require.Contains(t, res.Stdout, "running at-cap")
		// Negative guards: should NOT have at-cap-keda or at-cap-auto here.
		require.NotContains(t, res.Stdout, "at-cap-keda")
		require.NotContains(t, res.Stdout, "at-cap-auto")
		// Verify the long-form is NOT in the ps decoration path (negative guard).
		require.NotContains(t, res.Stdout, "(keda")
		require.NotContains(t, res.Stdout, "(auto")
	})
}

// TestDecorateStatus_AtCapKedaService_AppendsAtCapKeda — service has KEDA
// autoscaler, helper emits at-cap-keda regardless of AtCapAction value.
func TestDecorateStatus_AtCapKedaService_AppendsAtCapKeda(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServiceKedaEnabled("name")}, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "running at-cap-keda")
		require.NotContains(t, res.Stdout, "running at-cap ")
		require.NotContains(t, res.Stdout, "(keda")
	})
}

// TestDecorateStatus_AtCapAutoShutdownActive_AppendsAtCapAuto — Set G's
// auto-shutdown is configured but the service is NOT KEDA-driven, so the
// short form is at-cap-auto. Helper accepts the value already; Set G's
// lander does not need to revisit this path.
func TestDecorateStatus_AtCapAutoShutdownActive_AppendsAtCapAuto(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap("auto-shutdown"), fxAppBudgetStateTripped(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(nil, nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("name")}, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "running at-cap-auto")
		require.NotContains(t, res.Stdout, "(auto")
	})
}

// TestBudgetCapStatus_NoBudget_ReturnsClear — no budget configured, no
// decoration applied to any pod. Status column renders unchanged.
func TestBudgetCapStatus_NoBudget_ReturnsClear(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.NotContains(t, res.Stdout, "at-cap")
	})
}

// TestBudgetCapStatus_BudgetSetButNotBreached_ReturnsClear — budget is set
// but circuit breaker not tripped. No decoration applied.
func TestBudgetCapStatus_BudgetSetButNotBreached_ReturnsClear(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetState(), nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.NotContains(t, res.Stdout, "at-cap")
	})
}

// ============================================================================
// Error path
// ============================================================================

// TestBudgetCapStatus_AppBudgetGetError_LogsAndReturnsClear — budget API
// returns an error. Helper logs to stderr only and returns a clear capStatus
// so the user-visible STATUS column renders unchanged. Budget API hiccups
// must NEVER make `convox ps` worse for the customer.
func TestBudgetCapStatus_AppBudgetGetError_LogsAndReturnsClear(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, fmt.Errorf("budget API down"))

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code, "ps must succeed despite budget API failure")
		require.NotContains(t, res.Stdout, "at-cap", "no decoration applied on budget API failure")
		// stderr namespace tag is the contract for log scrapers.
		require.Contains(t, res.Stderr, "ns=cli_budget at=fetch-error")
	})
}

// TestBudgetShowBanner_KedaServiceDetected_PrintsLongFormBanner — when the
// app has at least one KEDA-driven service AND the budget is at-cap, the
// long-form disclosure banner appears in `convox budget show` stdout.
// Banner text is R3-pinned exact verbatim.
func TestBudgetShowBanner_KedaServiceDetected_PrintsLongFormBanner(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServiceKedaEnabled("worker")}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// R3-pinned banner text — must match verbatim.
		require.Contains(t, res.Stdout, "KEDA-managed services may scale despite block-new-deploys")
		require.Contains(t, res.Stdout, "v1 limitation; auto-shutdown closes gap in 3.24.6")
		require.Contains(t, res.Stdout, "see release notes")
	})
}

// TestBudgetShowBanner_NoKedaService_NoBanner — at-cap but no service has
// KEDA → no banner emitted. The cap-bypass disclosure is service-specific.
func TestBudgetShowBanner_NoKedaService_NoBanner(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("worker")}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.NotContains(t, res.Stdout, "KEDA-managed services")
	})
}

// ============================================================================
// Negative / regression guard
// ============================================================================

// TestDecorateStatus_LongFormStringsRejectedFromPsColumn — the `convox ps`
// STATUS column NEVER emits the long forms. Long forms are reserved for
// `convox budget show` banner only (per R3 pin).
func TestDecorateStatus_LongFormStringsRejectedFromPsColumn(t *testing.T) {
	for _, action := range []string{structs.BudgetAtCapActionBlockNewDeploys, "auto-shutdown"} {
		t.Run("action="+action, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
				i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(action), fxAppBudgetStateTripped(), nil)
				i.On("ServiceList", "app1").Return(structs.Services{fxServiceKedaEnabled("name")}, nil)

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				// Forbidden long forms — the parens are the canary.
				require.NotContains(t, res.Stdout, "(keda-managed)")
				require.NotContains(t, res.Stdout, "(auto-shutdown)")
				require.NotContains(t, res.Stdout, "at-cap (")
			})
		})
	}
}

// TestStatusColumnWidth_AtCapKedaFitsInDefaultWidth — render `convox ps`
// with STATUS = "running at-cap-keda" (longest sub-state form). Assert no
// truncation occurs. The 12-char `at-cap-keda` plus space plus 7-char
// `running` is 20 chars — well within stdcli's column ceiling.
func TestStatusColumnWidth_AtCapKedaFitsInDefaultWidth(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServiceKedaEnabled("name")}, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// The full token must appear unbroken.
		require.Contains(t, res.Stdout, "at-cap-keda")
		// Specifically, no ellipsis (truncation marker) should appear.
		require.NotContains(t, res.Stdout, "...")
		require.NotContains(t, res.Stdout, "at-cap-ked\n")
	})
}

// TestPsRowOrderUnchanged_OnlyStatusFieldDecorated — render with one at-cap
// row and verify the other columns (ID, SERVICE, RELEASE, COMMAND) are
// untouched. Only STATUS differs.
func TestPsRowOrderUnchanged_OnlyStatusFieldDecorated(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("name")}, nil).Maybe()

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// Verify column order/content from fxProcess: Id=pid1, Name=name,
		// Release=release1, Command=command. STATUS is decorated.
		require.Contains(t, res.Stdout, "pid1")
		require.Contains(t, res.Stdout, "name")
		require.Contains(t, res.Stdout, "release1")
		require.Contains(t, res.Stdout, "command")
		require.Contains(t, res.Stdout, "running at-cap")
		// Header line should be unchanged.
		require.True(t, strings.Contains(res.Stdout, "ID") &&
			strings.Contains(res.Stdout, "SERVICE") &&
			strings.Contains(res.Stdout, "STATUS") &&
			strings.Contains(res.Stdout, "RELEASE") &&
			strings.Contains(res.Stdout, "STARTED") &&
			strings.Contains(res.Stdout, "COMMAND"),
			"all six column headers must remain present and in default order")
	})
}

// TestServicesBudgetColumn_OnlyEmittedWhenAnyServiceAtCap — call `services`
// with budget != at-cap → no BUDGET column header. Re-call with at-cap
// state → BUDGET column appears with cells populated.
func TestServicesBudgetColumn_OnlyEmittedWhenAnyServiceAtCap(t *testing.T) {
	t.Run("not at cap — no BUDGET column", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("worker")}, nil)
			i.On("AppBudgetGet", "app1").Return(nil, nil, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			require.NotContains(t, res.Stdout, "BUDGET")
			require.NotContains(t, res.Stdout, "at-cap")
		})
	})
	t.Run("at cap — BUDGET column appears with cells", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("worker"), fxServiceKedaEnabled("vllm")}, nil)
			i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			require.Contains(t, res.Stdout, "BUDGET")
			require.Contains(t, res.Stdout, "at-cap-keda")
			require.Contains(t, res.Stdout, "at-cap")
		})
	})
}


// TestDecorateStatus_ArmedWithNotifyMin_AppendsArmedToken — F-15 fix
// (catalog F-15). Locks in the `armed-Nm` STATUS column token. The
// other STATUS tokens (at-cap, at-cap-keda, at-cap-auto) all have tests;
// only `armed-Nm` lacked one until now. The mock response carries an
// armed-state shutdownState with ArmedAt set ~25 minutes ago and the
// default 30-minute notify window — countdown should be ~5 minutes.
func TestDecorateStatus_ArmedWithNotifyMin_AppendsArmedToken(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// Armed 25 minutes ago with default 30-minute notify window =
		// ~5 minutes remaining. Use NotifyBeforeMinutes=30 explicit so
		// the test does not depend on the default fallback.
		now := time.Now().UTC()
		armed := now.Add(-25 * time.Minute)
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetStateTripped(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(&structs.AppBudgetShutdownState{
			SchemaVersion:        1,
			ArmedAt:              &armed,
			NotifyBeforeMinutes:  30,
			RecoveryMode:         "auto-on-reset",
			ShutdownOrder:        "largest-cost",
			ShutdownTickId:       "tick-armed-status-test",
			EligibleServiceCount: 1,
			Services:             []structs.AppBudgetShutdownStateService{{Name: "name"}},
		}, nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("name")}, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// Token should appear as `armed-<N>m` where N is somewhere in
		// 1..6 (clock drift between assert and execution can shift by a
		// minute or two; allow a window).
		armedFound := false
		for _, suffix := range []string{"armed-1m", "armed-2m", "armed-3m", "armed-4m", "armed-5m", "armed-6m"} {
			if strings.Contains(res.Stdout, suffix) {
				armedFound = true
				break
			}
		}
		require.True(t, armedFound, "STATUS column must render armed-Nm token (got: %q)", res.Stdout)
		// armed-Nm wins over at-cap-auto when in the armed window.
		require.NotContains(t, res.Stdout, "at-cap-auto", "armed-Nm precedence wins over at-cap-auto when ArmedCountdownMinutes > 0")
	})
}
