package cli_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
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

func TestRackInstall(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestLatest = "foo"

		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return([]byte{}, nil)
		me.On("Terminal", "terraform", "init", "-force-copy", "-no-color", "-upgrade").Return(nil)
		me.On("Terminal", "terraform", "apply", "-auto-approve", "-no-color").Return(nil)
		e.Executor = me

		res, err := testExecute(e, "rack install local dev1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "racks", "dev1")
		tf := filepath.Join(dir, "main.tf")

		_, err = os.Stat(dir)
		require.NoError(t, err)
		_, err = os.Stat(tf)
		require.NoError(t, err)

		tfdata, err := ioutil.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := ioutil.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(string(tfdata), "\n"), strings.Trim(string(testdata), "\n"))

		// existing rack should not switch
		res, err = testExecute(e, "switch", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"rack1"})

		me.AssertExpectations(t)
	})
}

func TestRackInstallArgs(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestLatest = "foo"

		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return([]byte{}, nil)
		me.On("Terminal", "terraform", "init", "-force-copy", "-no-color", "-upgrade").Return(nil)
		me.On("Terminal", "terraform", "apply", "-auto-approve", "-no-color").Return(nil)
		e.Executor = me

		res, err := testExecute(e, "rack install local dev1 foo=bar baz=qux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "racks", "dev1")
		tf := filepath.Join(dir, "main.tf")

		_, err = os.Stat(dir)
		require.NoError(t, err)
		_, err = os.Stat(tf)
		require.NoError(t, err)

		tfdata, err := ioutil.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := ioutil.ReadFile("testdata/terraform/dev1.args.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(string(tfdata), "\n"), strings.Trim(string(testdata), "\n"))

		me.AssertExpectations(t)
	})
}

func TestRackInstallPrepare(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestLatest = "foo"

		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return([]byte{}, nil)
		me.On("Terminal", "terraform", "init", "-force-copy", "-no-color", "-upgrade").Return(nil)
		e.Executor = me

		res, err := testExecute(e, "rack install local dev1 --prepare", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "racks", "dev1")
		tf := filepath.Join(dir, "main.tf")

		_, err = os.Stat(dir)
		require.NoError(t, err)
		_, err = os.Stat(tf)
		require.NoError(t, err)

		tfdata, err := ioutil.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := ioutil.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(string(tfdata), "\n"), strings.Trim(string(testdata), "\n"))

		me.AssertExpectations(t)
	})
}

func TestRackInstallSwitch(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestLatest = "foo"

		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return([]byte{}, nil)
		me.On("Terminal", "terraform", "init", "-force-copy", "-no-color", "-upgrade").Return(nil)
		me.On("Terminal", "terraform", "apply", "-auto-approve", "-no-color").Return(nil)
		me.On("Execute", "terraform", "output", "-json").Return([]byte(`{}`), nil)
		e.Executor = me

		// remove current rack
		os.Remove(filepath.Join(e.Settings, "current"))

		res, err := testExecute(e, "rack install local dev1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "racks", "dev1")
		tf := filepath.Join(dir, "main.tf")

		_, err = os.Stat(dir)
		require.NoError(t, err)
		_, err = os.Stat(tf)
		require.NoError(t, err)

		tfdata, err := ioutil.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := ioutil.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(string(tfdata), "\n"), strings.Trim(string(testdata), "\n"))

		// no existing rack should switch
		res, err = testExecute(e, "switch", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"dev1"})

		me.AssertExpectations(t)
	})
}

func TestRackInstallVersion(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return([]byte{}, nil)
		me.On("Terminal", "terraform", "init", "-force-copy", "-no-color", "-upgrade").Return(nil)
		me.On("Terminal", "terraform", "apply", "-auto-approve", "-no-color").Return(nil)
		e.Executor = me

		res, err := testExecute(e, "rack install local dev1 -v otherver", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "racks", "dev1")
		tf := filepath.Join(dir, "main.tf")

		_, err = os.Stat(dir)
		require.NoError(t, err)
		_, err = os.Stat(tf)
		require.NoError(t, err)

		tfdata, err := ioutil.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := ioutil.ReadFile("testdata/terraform/dev1.version.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(string(tfdata), "\n"), strings.Trim(string(testdata), "\n"))

		me.AssertExpectations(t)
	})
}

func TestRackInstallNoTerraform(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "terraform", "version").Return(nil, fmt.Errorf("exit 1"))
		e.Executor = me

		res, err := testExecute(e, "rack install local dev1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: terraform required"})
		res.RequireStdout(t, []string{""})

		me.AssertExpectations(t)
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
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
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
		res.RequireStdout(t, []string{
			"Updating parameters... OK",
		})
	})
}

func TestRackParamsSetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
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

func TestRackPs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemProcesses", structs.SystemProcessesOptions{}).Return(structs.Processes{*fxProcess(), *fxProcessPending()}, nil)

		res, err := testExecute(e, "rack ps", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID    APP   SERVICE  STATUS   RELEASE   CPU    MEM    STARTED     COMMAND",
			"pid1  app1  name     running  release1  89.0%  387MB  2 days ago  command",
			"pid2  app1  name     pending  release1  --     --     2 days ago  command",
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
			"ID    APP   SERVICE  STATUS   RELEASE   CPU    MEM    STARTED     COMMAND",
			"pid1  app1  name     running  release1  89.0%  387MB  2 days ago  command",
			"pid2  app1  name     pending  release1  --     --     2 days ago  command",
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
			"VERSION   UPDATED",
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

func TestRackUninstall(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		me := e.Executor.(*mockstdcli.Executor)
		me.On("Terminal", "terraform", "init", "-no-color", "-upgrade").Return(nil)
		me.On("Terminal", "terraform", "destroy", "-auto-approve", "-no-color").Return(nil)

		res, err := testExecute(e, "rack uninstall dev1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})

		dir := filepath.Join(e.Settings, "convox", "racks", "dev1")

		_, err = os.Stat(dir)
		require.True(t, os.IsNotExist(err))

		me.AssertExpectations(t)
	})
}

func TestRackUninstallUnknown(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		res, err := testExecute(e, "rack uninstall dev2", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: could not find rack: dev2"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackUpdate(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.SystemUpdateOptions{
			Version: options.String("latest"),
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack update", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackUpdateSpecific(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.SystemUpdateOptions{
			Version: options.String("ver1"),
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack update ver1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackUpdateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.SystemUpdateOptions{
			Version: options.String("latest"),
		}
		i.On("SystemUpdate", opts).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack update", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}
