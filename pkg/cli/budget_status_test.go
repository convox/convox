package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
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

func TestDecorateStatus_BlockNewDeploysKedaService_AppendsAtCap(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "running at-cap")
		require.NotContains(t, res.Stdout, "at-cap-keda")
	})
}

func TestDecorateStatus_AutoShutdownFired_AppendsAtCapAuto(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(fxShutdownStateFired(), nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "running at-cap-auto")
		require.NotContains(t, res.Stdout, "(auto")
	})
}

func TestDecorateStatus_StaleBreakerAnyAction_AppendsAtCap(t *testing.T) {
	for _, action := range []string{structs.BudgetAtCapActionAlertOnly, structs.BudgetAtCapActionAutoShutdown} {
		t.Run(action, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
				i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(action), fxAppBudgetStateTripped(), nil)
				i.On("AppBudgetShutdownStateGet", "app1").Return(nil, nil).Maybe()

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				require.Contains(t, res.Stdout, "running at-cap")
			})
		})
	}
}

func TestDecorateStatus_AutoShutdownTerminalOrAbsent_NoToken(t *testing.T) {
	restored := fxShutdownStateFired()
	restored.RestoredAt = restored.ShutdownAt
	expired := fxShutdownStateFired()
	expired.ExpiredAt = expired.ShutdownAt

	for name, state := range map[string]*structs.AppBudgetShutdownState{"absent": nil, "restored": restored, "expired": expired} {
		t.Run(name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
				i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
				i.On("AppBudgetShutdownStateGet", "app1").Return(state, nil)

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				require.NotContains(t, res.Stdout, "at-cap")
				require.NotContains(t, res.Stdout, "armed-")
			})
		})
	}
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
// must NEVER make `convox ps` worse for the user.
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

func TestBudgetCapStatus_RackWithoutBudgetRoute_Silent(t *testing.T) {
	tests := []struct {
		cmd    string
		setup  func(*mocksdk.Interface)
		stdout []string
	}{
		{
			cmd: "ps -a app1",
			setup: func(i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), *fxProcessPending()}, nil)
			},
			stdout: []string{
				"ID    SERVICE  STATUS   RELEASE   STARTED     COMMAND",
				"pid1  name     running  release1  2 days ago  command",
				"pid1  name     pending  release1  2 days ago  command",
			},
		},
		{
			cmd: "services -a app1",
			setup: func(i *mocksdk.Interface) {
				i.On("ServiceList", "app1").Return(structs.Services{*fxService(), *fxService()}, nil)
			},
			stdout: []string{
				"SERVICE   DOMAIN  PORTS",
				"service1  domain  1:2 1:2",
				"service1  domain  1:2 1:2",
			},
		},
		{
			cmd: "scale -a app1",
			setup: func(i *mocksdk.Interface) {
				i.On("ServiceList", "app1").Return(structs.Services{*fxService(), *fxService()}, nil)
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), *fxProcess()}, nil)
			},
			stdout: []string{
				"SERVICE   DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
				"service1  1        0        2    3       -    -    -    ",
				"service1  1        0        2    3       -    -    -    ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				tt.setup(i)
				i.On("AppBudgetGet", "app1").Return(nil, nil, fmt.Errorf("response status 404"))

				res, err := testExecute(e, tt.cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{""})
				res.RequireStdout(t, tt.stdout)
			})
		})
	}
}

func TestBudgetCapStatus_RealErrorsStillLogged(t *testing.T) {
	for _, msg := range []string{"app not found: app1", "response status 503", "you are unauthorized to access this"} {
		t.Run(msg, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
				i.On("AppBudgetGet", "app1").Return(nil, nil, fmt.Errorf("%s", msg))

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{fmt.Sprintf("ns=cli_budget at=fetch-error err=%q", msg)})
			})
		})
	}
}

func TestPs_RackWithoutBudgetRoute_RealClient(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		testRealRack(t, func(s *stdapi.Server) {
			s.Route("GET", "/apps/{app}/processes", func(c *stdapi.Context) error {
				return c.RenderJSON(structs.Processes{*fxProcess()})
			})
		})

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID    SERVICE  STATUS   RELEASE   STARTED     COMMAND",
			"pid1  name     running  release1  2 days ago  command",
		})
	})
}

func TestPs_BudgetAppNotFound_RealClient(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		testRealRack(t, func(s *stdapi.Server) {
			s.Route("GET", "/apps/{app}/processes", func(c *stdapi.Context) error {
				return c.RenderJSON(structs.Processes{*fxProcess()})
			})
			s.Route("GET", "/apps/{app}/budget", func(c *stdapi.Context) error {
				return structs.ErrNotFound("app not found: %s", c.Var("app"))
			})
		})

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{`ns=cli_budget at=fetch-error err="app not found: app1"`})
	})
}

func TestBudgetShowBanner_KedaServiceDetected_PrintsLongFormBanner(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ServiceList", "app1").Return(structs.Services{fxServiceKedaEnabled("worker")}, nil)

		res, err := testExecute(e, "budget show app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "KEDA-managed services may scale despite block-new-deploys; auto-shutdown stops them at the cap")
		require.NotContains(t, res.Stdout, "3.24.6")
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

func TestDecorateStatus_LongFormStringsRejectedFromPsColumn(t *testing.T) {
	for _, action := range []string{structs.BudgetAtCapActionBlockNewDeploys, "auto-shutdown"} {
		t.Run("action="+action, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
				i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(action), fxAppBudgetStateTripped(), nil)
				i.On("AppBudgetShutdownStateGet", "app1").Return(fxShutdownStateFired(), nil).Maybe()

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				require.NotContains(t, res.Stdout, "(keda-managed)")
				require.NotContains(t, res.Stdout, "(auto-shutdown)")
				require.NotContains(t, res.Stdout, "at-cap (")
			})
		})
	}
}

func TestStatusColumnWidth_AtCapKedaFitsInDefaultWidth(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		state := fxShutdownStateFired()
		state.Services[0].KedaScaledObject = &structs.AppBudgetShutdownStateKeda{Name: "name", PausedReplicasAnnotationSet: true}
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(state, nil)

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "running at-cap-keda")
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
	t.Run("block-new-deploys every service at-cap", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("worker"), fxServiceKedaEnabled("vllm")}, nil)
			i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE  DOMAIN  PORTS  AUTOSCALE    BUDGET",
				"worker   domain         -            at-cap",
				"vllm     domain         gpu-util>70  at-cap",
			})
		})
	})
	t.Run("auto-shutdown fired per service", func(t *testing.T) {
		testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
			now := time.Now().UTC()
			state := fxShutdownStateFired()
			state.Services = []structs.AppBudgetShutdownStateService{
				{Name: "web", ShutdownAt: &now},
				{Name: "vllm", ShutdownAt: &now, KedaScaledObject: &structs.AppBudgetShutdownStateKeda{Name: "vllm"}},
				{Name: "failed"},
			}
			i.On("ServiceList", "app1").Return(structs.Services{fxServicePlain("web"), fxServicePlain("vllm"), fxServicePlain("failed"), fxServicePlain("db")}, nil)
			i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
			i.On("AppBudgetShutdownStateGet", "app1").Return(state, nil)

			res, err := testExecute(e, "services -a app1", nil)
			require.NoError(t, err)
			require.Equal(t, 0, res.Code)
			res.RequireStdout(t, []string{
				"SERVICE  DOMAIN  PORTS  BUDGET",
				"web      domain         at-cap-auto",
				"vllm     domain         at-cap-keda",
				"failed   domain         -",
				"db       domain         -",
			})
		})
	})
}

func TestScaleStatus_AutoShutdownFired_AppendsTokenAfterCold(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		cold := true
		web := *fxService()
		web.Name = "web"
		web.ColdStart = &cold
		other := *fxService()
		other.Name = "other"
		now := time.Now().UTC()
		state := fxShutdownStateFired()
		state.Services = []structs.AppBudgetShutdownStateService{{Name: "web", ShutdownAt: &now}}
		i.On("ServiceList", "app1").Return(structs.Services{web, other}, nil)
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
		i.On("AppBudgetShutdownStateGet", "app1").Return(state, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"other    1        0        2    3       -    -    -    ",
			"web      1        0        2    3       -    -    -    COLD (~2-5m first req) at-cap-auto",
		})
	})
}

func TestDecorateStatus_ArmedWithNotifyMin_AppendsArmedToken(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		now := time.Now().UTC()
		armed := now.Add(-25 * time.Minute)
		exempt := *fxProcess()
		exempt.Id = "pid2"
		exempt.Name = "exempt"
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), exempt}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionAutoShutdown), fxAppBudgetState(), nil)
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

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		var nameLine, exemptLine string
		for _, l := range strings.Split(res.Stdout, "\n") {
			if strings.HasPrefix(l, "pid1") {
				nameLine = l
			}
			if strings.HasPrefix(l, "pid2") {
				exemptLine = l
			}
		}
		armedFound := false
		for _, suffix := range []string{"armed-4m", "armed-5m", "armed-6m"} {
			if strings.Contains(nameLine, "running "+suffix) {
				armedFound = true
				break
			}
		}
		require.True(t, armedFound, "listed service must render armed-Nm (got: %q)", res.Stdout)
		require.NotContains(t, exemptLine, "armed-")
		require.NotContains(t, res.Stdout, "at-cap")
	})
}

func fxShutdownStateFired() *structs.AppBudgetShutdownState {
	armed := time.Now().UTC().Add(-time.Hour)
	shut := armed.Add(30 * time.Minute)
	return &structs.AppBudgetShutdownState{
		SchemaVersion:        1,
		ArmedAt:              &armed,
		ShutdownAt:           &shut,
		NotifyBeforeMinutes:  30,
		RecoveryMode:         "auto-on-reset",
		ShutdownOrder:        "largest-cost",
		ShutdownTickId:       "tick-fired-status-test",
		EligibleServiceCount: 1,
		Services:             []structs.AppBudgetShutdownStateService{{Name: "name", ShutdownAt: &shut}},
	}
}
