package cli_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestRack(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)

		res, err := testExecute(e, "rack", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Name      name",
			"Provider  provider",
			"Region    region",
			"Router    domain",
			"Status    running",
			"Version   21000101000000",
		})
	})
}

func TestRackError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackInternal(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemInternal(), nil)

		res, err := testExecute(e, "rack", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Name      name",
			"Provider  provider",
			"Region    region",
			"Router    domain (external)",
			"          domain-internal (internal)",
			"Status    running",
			"Version   20180901000000",
		})
	})
}

func TestRackLogs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemLogs", structs.LogsOptions{Prefix: options.Bool(true)}).Return(testLogs(fxLogs()), nil)

		res, err := testExecute(e, "rack logs", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			fxLogs()[0],
			fxLogs()[1],
		})
	})
}

func TestRackLogsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemLogs", structs.LogsOptions{Prefix: options.Bool(true)}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack logs", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackParams(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)

		res, err := testExecute(e, "rack params", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Autoscale   Yes",
			"ParamFoo    value1",
			"ParamOther  value2",
		})
	})
}

func TestRackParamsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack params", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackParamsSet(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.SystemUpdateOptions{
			Parameters: map[string]string{
				"Foo": "bar",
				"Baz": "qux",
			},
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack params set Foo=bar Baz=qux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Updating parameters... OK"})
	})
}

func TestRackParamsSetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.SystemUpdateOptions{
			Parameters: map[string]string{
				"Foo": "bar",
				"Baz": "qux",
			},
		}
		i.On("SystemUpdate", opts).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack params set Foo=bar Baz=qux", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Updating parameters... "})
	})
}

func TestRackParamsSetClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		i.On("AppParametersSet", "name", map[string]string{"Foo": "bar", "Baz": "qux"}).Return(nil)

		res, err := testExecute(e, "rack params set Foo=bar Baz=qux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Updating parameters... OK"})
	})
}

func TestRackParamsSetClassicError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		i.On("AppParametersSet", "name", map[string]string{"Foo": "bar", "Baz": "qux"}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack params set Foo=bar Baz=qux", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Updating parameters... "})
	})
}

func TestRackPs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemProcesses", structs.SystemProcessesOptions{}).Return(structs.Processes{*fxProcess(), *fxProcessPending()}, nil)

		res, err := testExecute(e, "rack ps", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID    APP   SERVICE  STATUS   RELEASE   STARTED     COMMAND",
			"pid1  app1  name     running  release1  2 days ago  command",
			"pid1  app1  name     pending  release1  2 days ago  command",
		})
	})
}

func TestRackPsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemProcesses", structs.SystemProcessesOptions{}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack ps", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackPsAll(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemProcesses", structs.SystemProcessesOptions{All: options.Bool(true)}).Return(structs.Processes{*fxProcess(), *fxProcessPending()}, nil)

		res, err := testExecute(e, "rack ps -a", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID    APP   SERVICE  STATUS   RELEASE   STARTED     COMMAND",
			"pid1  app1  name     running  release1  2 days ago  command",
			"pid1  app1  name     pending  release1  2 days ago  command",
		})
	})
}

func TestRackReleases(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemReleases").Return(structs.Releases{*fxRelease(), *fxRelease()}, nil)

		res, err := testExecute(e, "rack releases", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"VERSION   UPDATED   ",
			"release1  2 days ago",
			"release1  2 days ago",
		})
	})
}

func TestRackReleasesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemReleases").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack releases", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackScale(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)

		res, err := testExecute(e, "rack scale", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Autoscale  Yes",
			"Count      1",
			"Status     running",
			"Type       type",
		})
	})
}

func TestRackScaleError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack scale", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackScaleUpdate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemUpdate", structs.SystemUpdateOptions{Count: options.Int(5), Type: options.String("type1")}).Return(nil)

		res, err := testExecute(e, "rack scale -c 5 -t type1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Scaling rack... OK"})
	})
}

func TestRackScaleUpdateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemUpdate", structs.SystemUpdateOptions{Count: options.Int(5), Type: options.String("type1")}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack scale -c 5 -t type1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Scaling rack... "})
	})
}

func TestRackUpdate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemUpdate", structs.SystemUpdateOptions{Version: options.String("version1")}).Return(nil)

		res, err := testExecute(e, "rack update version1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Updating to version1... OK"})
	})
}

func TestRackUpdateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemUpdate", structs.SystemUpdateOptions{Version: options.String("version1")}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack update version1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Updating to version1... "})
	})
}

func TestRackWait(t *testing.T) {
	testClientWait(t, 100*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.LogsOptions{
			Prefix: options.Bool(true),
			Since:  options.Duration(5 * time.Second),
		}
		i.On("SystemLogs", opts).Return(testLogs(fxLogsSystem()), nil).Once()
		i.On("SystemGet").Return(&structs.System{Status: "updating"}, nil).Twice()
		i.On("SystemGet").Return(fxSystem(), nil)

		res, err := testExecute(e, "rack wait", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Waiting for rack... ",
			fxLogsSystem()[0],
			fxLogsSystem()[1],
			"OK",
		})
	})
}

func TestRackWaitError(t *testing.T) {
	testClientWait(t, 100*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.LogsOptions{
			Prefix: options.Bool(true),
			Since:  options.Duration(5 * time.Second),
		}
		i.On("SystemLogs", opts).Return(testLogs(fxLogsSystem()), nil).Once()
		i.On("SystemGet").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack wait", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{
			"Waiting for rack... ",
			fxLogsSystem()[0],
			fxLogsSystem()[1],
		})
	})
}
