package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/stretchr/testify/require"
)

func TestPs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{*fxProcess(), *fxProcessPending()}, nil)
		i.On("AppBudgetGet", "app1").Return(nil, nil, nil).Maybe()

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID    SERVICE  STATUS   RELEASE   STARTED     COMMAND",
			"pid1  name     running  release1  2 days ago  command",
			"pid1  name     pending  release1  2 days ago  command",
		})
	})
}

func TestPsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestPsAppNotFound(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)
		i.On("AppGet", "app1").Return(nil, fmt.Errorf("app not found: app1"))

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: app not found: app1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestPsNoProcesses(t *testing.T) {
	tests := []struct {
		cmd  string
		opts structs.ProcessListOptions
	}{
		{"ps -a app1", structs.ProcessListOptions{}},
		{"ps -a app1 -s missing", structs.ProcessListOptions{Service: options.String("missing")}},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", tt.opts).Return(structs.Processes{}, nil)
				i.On("AppGet", "app1").Return(fxApp(), nil)
				i.On("AppBudgetGet", "app1").Return(nil, nil, nil)

				res, err := testExecute(e, tt.cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{""})
				res.RequireStdout(t, []string{"ID  SERVICE  STATUS  RELEASE  STARTED  COMMAND"})
			})
		})
	}
}

func TestPsAppGetOtherErrorKeepsTable(t *testing.T) {
	for _, msg := range []string{"response status 503", "you are unauthorized to access this", `namespaces "app1" not found`} {
		t.Run(msg, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ProcessList", "app1", structs.ProcessListOptions{}).Return(structs.Processes{}, nil)
				i.On("AppGet", "app1").Return(nil, fmt.Errorf("%s", msg))
				i.On("AppBudgetGet", "app1").Return(nil, nil, nil)

				res, err := testExecute(e, "ps -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{""})
				res.RequireStdout(t, []string{"ID  SERVICE  STATUS  RELEASE  STARTED  COMMAND"})
			})
		})
	}
}

func TestPs_MissingApp_RealClient(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		testRealRack(t, func(s *stdapi.Server) {
			s.Route("GET", "/apps/{app}/processes", func(c *stdapi.Context) error {
				return c.RenderJSON(structs.Processes{})
			})
			s.Route("GET", "/apps/{app}", func(c *stdapi.Context) error {
				return structs.ErrNotFound("app not found: %s", c.Var("app"))
			})
		})

		res, err := testExecute(e, "ps -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: app not found: app1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestPsInfo(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)

		res, err := testExecute(e, "ps info pid1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Id        pid1",
			"App       app1",
			"Command   command",
			"Instance  instance",
			"Release   release1",
			"Service   name",
			"Started   2 days ago",
			"Status    running",
		})
	})
}

func TestPsInfoError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessGet", "app1", "pid1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "ps info pid1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestPsStop(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessStop", "app1", "pid1").Return(nil)

		res, err := testExecute(e, "ps stop pid1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Stopping pid1... OK"})
	})
}

func TestPsStopError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessStop", "app1", "pid1").Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "ps stop pid1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Stopping pid1... "})
	})
}

func TestPsInfoExitCode(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", options.Int(3)), nil)

		res, err := testExecute(e, "ps info pid1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Id        pid1",
			"App       app1",
			"Command   command",
			"Instance  instance",
			"Release   release1",
			"Service   name",
			"Started   2 days ago",
			"Status    failed",
			"Exit      3",
		})
	})
}
