package cli_test

import (
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func pausedScaleService() structs.Service {
	s := *fxService()
	mn, mx := 0, 10
	s.Count = 0
	s.Min = &mn
	s.Max = &mx
	s.ColdStart = options.Bool(true)
	s.Paused = options.Bool(true)
	return s
}

func TestScaleShowsPaused(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{pausedScaleService()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"SERVICE   DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS",
			"service1  0        0        2    3       -    0    10   paused",
		})
	})
}

func TestScaleShowsPausedBeforeBudgetToken(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceList", "app1").Return(structs.Services{pausedScaleService()}, nil)
		i.On("AppBudgetGet", "app1").Return(fxAppBudgetCap(structs.BudgetAtCapActionBlockNewDeploys), fxAppBudgetStateTripped(), nil)
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)

		res, err := testExecute(e, "scale -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "service1  0        0        2    3       -    0    10   paused at-cap")
	})
}
