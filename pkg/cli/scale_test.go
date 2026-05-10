package cli_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestScale(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{*fxService(), *fxService()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), *fxProcess()}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"service1  1        0        2    3       -    -    -    ",
			"service1  1        0        2    3       -    -    -    ",
		})
	})
}

func TestScaleShowsAutoscaleAndCold(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		mn, mx := 0, 10
		cold := true
		gpuTh := 70
		svc := *fxService()
		svc.Name = "vllm"
		svc.Min = &mn
		svc.Max = &mx
		svc.Count = 0
		svc.ColdStart = &cold
		svc.Autoscale = &structs.ServiceAutoscaleState{Enabled: true, GpuThreshold: &gpuTh}

		warm := *fxService()
		warm.Name = "web"
		two := 2
		warm.Min = &two
		warm.Max = &two
		warm.Count = 2

		i.On("ServiceList", "app1").Return(structs.Services{svc, warm}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  AUTOSCALE    STATUS",
			"vllm     0        0        2    3       -    0    10   gpu-util>70  COLD (~2-5m first req)",
			"web      2        0        2    3       -    2    2    -            ",
		})
	})
}

func TestScaleError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestScaleUpdate(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceUpdate", "app1", "web", structs.ServiceUpdateOptions{Count: options.Int(3), Cpu: options.Int(5), Memory: options.Int(10)}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --cpu 5 --memory 10 --count 3 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Scaling web... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
			"OK",
		})
	})
}

func TestScaleUpdateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceUpdate", "app1", "web", structs.ServiceUpdateOptions{Count: options.Int(3), Cpu: options.Int(5), Memory: options.Int(10)}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "scale web --cpu 5 --memory 10 --count 3 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Scaling web... "})
	})
}

func TestScaleMinMax(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(5)}
		i.On("SystemGet").Return(&structs.System{Version: "3.24.6"}, nil)
		i.On("ServiceUpdate", "app1", "web", opts).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --min 1 --max 5 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestScaleMinMaxRefusedOnPre3246Rack(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(&structs.System{Version: "3.24.5"}, nil)

		res, err := testExecute(e, "scale web --min 1 --max 5 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "rack version 3.24.6 or later")
	})
}

func TestScaleMinCountMutuallyExclusive(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "scale web --min 0 --count 3 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: --min/--max and --count are mutually exclusive"})
	})
}

func TestScaleMinZeroDeadPodsFastFail(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		svc := *fxService()
		svc.Name = "web"
		i.On("SystemGet").Return(&structs.System{Version: "3.24.6"}, nil)
		i.On("ServiceList", "app1").Return(structs.Services{svc}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()

		res, err := testExecute(e, "scale web --min 0 --max 5 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "no autoscale configured")
	})
}

func TestScaleMaxZero(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "scale web --max 0 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: --max must be >= 1"})
	})
}

func TestScaleMinGreaterThanMax(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "scale web --min 5 --max 2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: --max must be >= --min"})
	})
}

func TestScaleMinZeroServiceNotFound(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(&structs.System{Version: "3.24.6"}, nil)
		i.On("ServiceList", "app1").Return(structs.Services{*fxService()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()

		res, err := testExecute(e, "scale missing --min 0 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "not found in app")
	})
}

func TestScaleMinZeroWithAutoscale(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		gpu := 70
		svc := *fxService()
		svc.Name = "web"
		svc.Autoscale = &structs.ServiceAutoscaleState{Enabled: true, GpuThreshold: &gpu}
		i.On("SystemGet").Return(&structs.System{Version: "3.24.6"}, nil)
		i.On("ServiceList", "app1").Return(structs.Services{svc}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()

		opts := structs.ServiceUpdateOptions{Min: options.Int(0), Max: options.Int(10)}
		i.On("ServiceUpdate", "app1", "web", opts).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --min 0 --max 10 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestScaleDaemonsetRow(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// A daemonset-backed service has Min/Max populated but Autoscale and
		// ColdStart remain nil because daemonsets cannot scale to zero.
		two := 2
		agent := *fxService()
		agent.Name = "fluentd"
		agent.Min = &two
		agent.Max = &two
		agent.Count = 2

		i.On("ServiceList", "app1").Return(structs.Services{agent}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// No AUTOSCALE column (no service has autoscale enabled).
		res.RequireStdout(t, []string{
			"SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"fluentd  2        0        2    3       -    2    2    ",
		})
	})
}

func TestScaleReadModePositionalFilter(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		web := *fxService()
		web.Name = "web"
		api := *fxService()
		api.Name = "api"
		// pre-watch existence check + watch-loop ServiceList both call ServiceList
		i.On("ServiceList", "app1").Return(structs.Services{web, api}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale web -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// Filtered output: only the row for "web", header preserved.
		res.RequireStdout(t, []string{
			"SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"web      1        0        2    3       -    -    -    ",
		})
	})
}

func TestScaleReadModeNoPositionalShowsAll(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		web := *fxService()
		web.Name = "web"
		api := *fxService()
		api.Name = "api"
		i.On("ServiceList", "app1").Return(structs.Services{web, api}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// Both services present, alphabetically sorted.
		res.RequireStdout(t, []string{
			"SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"api      1        0        2    3       -    -    -    ",
			"web      1        0        2    3       -    -    -    ",
		})
	})
}

func TestScaleReadModeServiceNotFound(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		web := *fxService()
		web.Name = "web"
		i.On("ServiceList", "app1").Return(structs.Services{web}, nil)

		res, err := testExecute(e, "scale notfound -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{`ERROR: service "notfound" not found in app app1`})
	})
}

func TestScaleReadModeCaseSensitive(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		web := *fxService()
		web.Name = "web"
		i.On("ServiceList", "app1").Return(structs.Services{web}, nil)

		// Case-mismatch ('Web' vs 'web') is rejected at pre-watch validation.
		res, err := testExecute(e, "scale Web -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{`ERROR: service "Web" not found in app app1`})
	})
}

func TestScaleImperativeWithoutServiceStillErrors(t *testing.T) {
	// Imperative mode (--count) without a positional service must keep
	// erroring "service name required". F7's positional-in-read-mode
	// loosening must not weaken the imperative contract.
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "scale --count=3 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "service name required")
	})
}

// TestScaleColumnPositionContract pins the column-position contract against
// the public 3.24.5 baseline (`SERVICE | DESIRED | RUNNING | CPU | MEMORY |
// GPU`). User scripts that parse `convox scale` output positionally
// (`awk '{print $2}'`, `cut -f3`) MUST keep working unchanged across the
// 3.24.5 → 3.24.6 upgrade. New columns introduced in 3.24.6 (MIN, MAX,
// AUTOSCALE, STATUS) must append at positions 7+, never shift the legacy
// six.
//
// If this test fails, the column-position contract for 3.24.5 users is
// broken — DO NOT update the assertions to match new output. The fix is in
// pkg/cli/scale.go header construction.
func TestScaleColumnPositionContract(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		two := 2
		web := *fxService()
		web.Name = "web"
		web.Count = 2
		web.Min = &two
		web.Max = &two

		i.On("ServiceList", "app1").Return(structs.Services{web}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)

		// Parse the header row by whitespace-splitting the first stdout line.
		stdout := strings.TrimSuffix(res.Stdout, "\n")
		lines := strings.Split(stdout, "\n")
		require.NotEmpty(t, lines, "scale output must include at least a header line")
		header := strings.Fields(lines[0])

		// Positions 1-6 (0-indexed 0-5) must match the 3.24.5 column
		// names exactly. Anything at position 7+ is new and additive.
		require.GreaterOrEqual(t, len(header), 6,
			"header must have at least the 6 columns from 3.24.5; got %d", len(header))
		require.Equal(t, "SERVICE", header[0], "position 1 must be SERVICE (3.24.5 baseline)")
		require.Equal(t, "DESIRED", header[1], "position 2 must be DESIRED (3.24.5 baseline)")
		require.Equal(t, "RUNNING", header[2], "position 3 must be RUNNING (3.24.5 baseline)")
		require.Equal(t, "CPU", header[3], "position 4 must be CPU (3.24.5 baseline)")
		require.Equal(t, "MEMORY", header[4], "position 5 must be MEMORY (3.24.5 baseline)")
		require.Equal(t, "GPU", header[5], "position 6 must be GPU (3.24.5 baseline)")

		// New 3.24.6 columns are MIN, MAX (positions 7-8) and a trailing
		// STATUS. AUTOSCALE optionally appears between MAX and STATUS.
		require.Greater(t, len(header), 6,
			"3.24.6 must add at least one trailing column (MIN/MAX/STATUS)")
		require.Equal(t, "MIN", header[6], "position 7 must be MIN (3.24.6 additive)")
		require.Equal(t, "MAX", header[7], "position 8 must be MAX (3.24.6 additive)")
		require.Equal(t, "STATUS", header[len(header)-1],
			"trailing column must be STATUS (3.24.6 additive)")

		// Data row positions 1-6 must be parseable as integers (DESIRED,
		// RUNNING, CPU, MEMORY) and a string ("-" or numeric) for GPU —
		// the same shape 3.24.5 emitted. This catches any future regression
		// where the row construction order drifts from the header order.
		require.Len(t, lines, 2, "expected one header + one data row")
		row := strings.Fields(lines[1])
		require.GreaterOrEqual(t, len(row), 6, "data row must have at least 6 fields")
		require.Equal(t, "web", row[0], "data row position 1 must be SERVICE name")
		require.Equal(t, "2", row[1], "data row position 2 must be DESIRED count (s.Count)")
		require.Equal(t, "0", row[2], "data row position 3 must be RUNNING count (ProcessList)")
		require.Equal(t, "2", row[3], "data row position 4 must be CPU millicores")
		require.Equal(t, "3", row[4], "data row position 5 must be MEMORY MB")
		require.Equal(t, "-", row[5], "data row position 6 must be GPU (- when zero)")
	})
}
