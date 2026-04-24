package cli_test

import (
	"fmt"
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
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), *fxProcess()}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"SERVICE   MIN  MAX  CURRENT  CPU  MEMORY  GPU  STATUS",
			"service1  -    -    1        2    3       -    ",
			"service1  -    -    1        2    3       -    ",
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
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE  MIN  MAX  CURRENT  CPU  MEMORY  GPU  AUTOSCALE    STATUS",
			"vllm     0    10   0        2    3       -    gpu-util>70  COLD (~2-5m first req)",
			"web      2    2    2        2    3       -    -            ",
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
		i.On("ServiceUpdate", "app1", "web", opts).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --min 1 --max 5 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
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
		i.On("ServiceList", "app1").Return(structs.Services{svc}, nil)

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
		i.On("ServiceList", "app1").Return(structs.Services{*fxService()}, nil)

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
		i.On("ServiceList", "app1").Return(structs.Services{svc}, nil)

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
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		// No AUTOSCALE column (no service has autoscale enabled).
		res.RequireStdout(t, []string{
			"SERVICE  MIN  MAX  CURRENT  CPU  MEMORY  GPU  STATUS",
			"fluentd  2    2    2        2    3       -    ",
		})
	})
}
