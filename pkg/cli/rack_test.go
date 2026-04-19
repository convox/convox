package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdcli"
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

		tfdata, err := os.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := os.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(removeSettingsLine(string(tfdata)), "\n"), removeSettingsLine(strings.Trim(string(testdata), "\n")))

		// existing rack should not switch
		res, err = testExecute(e, "switch", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"rack1"})

		me.AssertExpectations(t)
	})
}

func removeSettingsLine(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "settings =") {
			lines = append(lines[:i], lines[i+1:]...)
			break
		}
	}
	return strings.Join(lines, "\n")
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

		tfdata, err := os.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := os.ReadFile("testdata/terraform/dev1.args.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(removeSettingsLine(string(tfdata)), "\n"), removeSettingsLine(strings.Trim(string(testdata), "\n")))

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

		tfdata, err := os.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := os.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(removeSettingsLine(string(tfdata)), "\n"), removeSettingsLine(strings.Trim(string(testdata), "\n")))

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

		tfdata, err := os.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := os.ReadFile("testdata/terraform/dev1.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(removeSettingsLine(string(tfdata)), "\n"), removeSettingsLine(strings.Trim(string(testdata), "\n")))

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

		tfdata, err := os.ReadFile(tf)
		require.NoError(t, err)

		testdata, err := os.ReadFile("testdata/terraform/dev1.version.tf")
		require.NoError(t, err)

		require.Equal(t, strings.Trim(removeSettingsLine(string(tfdata)), "\n"), removeSettingsLine(strings.Trim(string(testdata), "\n")))

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

func TestRackLogsMaxLogRequests(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemLogs", structs.LogsOptions{Prefix: options.Bool(true), MaxLogRequests: options.Int(50)}).Return(testLogs(fxLogs()), nil)

		res, err := testExecute(e, "rack logs --max-log-requests 50", nil)
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
		res.RequireStdout(t, []string{
			"Updating parameters... OK",
		})
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

func TestRackParamsSetTerraformUpdateTimeout(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.SystemUpdateOptions{
			Parameters: map[string]string{
				"terraform_update_timeout": "3h",
			},
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack params set terraform_update_timeout=3h", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Updating parameters... OK",
		})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutCompound(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.SystemUpdateOptions{
			Parameters: map[string]string{
				"terraform_update_timeout": "2h30m",
			},
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack params set terraform_update_timeout=2h30m", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"Updating parameters... OK",
		})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutMinutes(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.SystemUpdateOptions{
			Parameters: map[string]string{
				"terraform_update_timeout": "90m",
			},
		}
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack params set terraform_update_timeout=90m", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"Updating parameters... OK",
		})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutInvalid(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		res, err := testExecute(e, "rack params set terraform_update_timeout=abc", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: invalid value for terraform_update_timeout: must be a valid duration (e.g., '2h', '90m', '2h30m'): time: invalid duration \"abc\""})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutNegative(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		res, err := testExecute(e, "rack params set terraform_update_timeout=-1h", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: invalid value for terraform_update_timeout: must be a positive duration"})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutZero(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		res, err := testExecute(e, "rack params set terraform_update_timeout=0s", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: invalid value for terraform_update_timeout: must be a positive duration"})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutEmpty(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		res, err := testExecute(e, "rack params set terraform_update_timeout=", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: param 'terraform_update_timeout' requires an explicit value (omit to keep current)"})
	})
}

func TestRackParamsSetTerraformUpdateTimeoutSpecialChars(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		res, err := testExecute(e, "rack params set terraform_update_timeout=${var.foo}", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: invalid value for terraform_update_timeout: must be a valid duration (e.g., '2h', '90m', '2h30m'): time: invalid duration \"${var.foo}\""})
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
		me.On("Terminal", "terraform", "destroy", "-auto-approve", "-no-color", "-refresh=true").Return(nil)

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
			Force:   options.Bool(false),
		}
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack update", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackUpdateDowngradeMinorError(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(&structs.System{
			Count:      1,
			Domain:     "domain",
			Name:       "name",
			Outputs:    map[string]string{"k1": "v1", "k2": "v2"},
			Parameters: map[string]string{"Autoscale": "Yes", "ParamFoo": "value1", "ParamOther": "value2"},
			Provider:   "provider",
			Region:     "region",
			Status:     "running",
			Type:       "type",
			Version:    "3.3.0",
		}, nil)

		res, err := testExecute(e, "rack update 3.2.12", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
	})
}

func TestRackUpdateSpecific(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.SystemUpdateOptions{
			Version: options.String("ver1"),
			Force:   options.Bool(false),
		}
		i.On("SystemGet").Return(fxSystem(), nil)
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
			Force:   options.Bool(false),
		}
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemUpdate", opts).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack update", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackUpdateForce(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.SystemUpdateOptions{
			Version: options.String("3.10.12"),
			Force:   options.Bool(true),
		}

		i.On("SystemGet").Return(&structs.System{
			Count:      1,
			Domain:     "domain",
			Name:       "name",
			Outputs:    map[string]string{"k1": "v1", "k2": "v2"},
			Parameters: map[string]string{"Autoscale": "Yes", "ParamFoo": "value1", "ParamOther": "value2"},
			Provider:   "provider",
			Region:     "region",
			Status:     "running",
			Type:       "type",
			Version:    "3.3.0",
		}, nil)

		i.On("SystemUpdate", opts).Return(nil)

		res, err := testExecute(e, "rack update 3.10.12 --force", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestRackParamsDefaultPipe(t *testing.T) {
	// Default test harness: IsTerminalFn wraps c.Writer().IsTerminal() which
	// returns false for *bytes.Buffer. Result: shouldMask=false, pipe-bypass.
	// This test verifies sensitive values render RAW when stdout is not a TTY,
	// which is the intentional behavior revert from 3.24.4-always-mask.
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemSensitive(), nil)

		res, err := testExecute(e, "rack params", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^secret_key\s+KEY-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^docker_hub_password\s+HUB-PASS$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^token\s+TOK-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^access_id\s+ACCESS-ID$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_host\s+eks-host-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_user\s+eks-user-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_pass\s+eks-pass-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^region\s+us-east-1$`), res.Stdout)
		require.NotContains(t, res.Stdout, "**********")
	})
}

func TestRackParamsMaskedTTY(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemSensitive(), nil)

		res, err := testExecute(e, "rack params", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^secret_key\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^docker_hub_password\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^token\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^access_id\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_host\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_user\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_pass\s+\*{10}$`), res.Stdout)

		require.Regexp(t, regexp.MustCompile(`(?m)^region\s+us-east-1$`), res.Stdout)

		require.NotContains(t, res.Stdout, "KEY-SENSITIVE")
		require.NotContains(t, res.Stdout, "HUB-PASS")
		require.NotContains(t, res.Stdout, "TOK-SENSITIVE")
		require.NotContains(t, res.Stdout, "ACCESS-ID")
		require.NotContains(t, res.Stdout, "eks-host-1")
		require.NotContains(t, res.Stdout, "eks-user-1")
		require.NotContains(t, res.Stdout, "eks-pass-1")
	})
}

func TestRackParamsRevealTTY(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemSensitive(), nil)

		res, err := testExecute(e, "rack params --reveal", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^secret_key\s+KEY-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^docker_hub_password\s+HUB-PASS$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^token\s+TOK-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^access_id\s+ACCESS-ID$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_host\s+eks-host-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_user\s+eks-user-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_pass\s+eks-pass-1$`), res.Stdout)

		require.NotContains(t, res.Stdout, "**********")
	})
}

func TestRackParamsMaskedTTYWithGroupFilter(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemSensitive(), nil)

		res, err := testExecute(e, "rack params -g security", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^access_id\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^secret_key\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^docker_hub_password\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^token\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_host\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_user\s+\*{10}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_pass\s+\*{10}$`), res.Stdout)

		require.NotRegexp(t, regexp.MustCompile(`(?m)^region\s`), res.Stdout)
		require.NotRegexp(t, regexp.MustCompile(`(?m)^ParamOther\s`), res.Stdout)
		require.NotContains(t, res.Stdout, "us-east-1")
	})
}

func TestRackParamsRevealTTYWithGroupFilter(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemSensitive(), nil)

		res, err := testExecute(e, "rack params -g security --reveal", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^access_id\s+ACCESS-ID$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^secret_key\s+KEY-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^docker_hub_password\s+HUB-PASS$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^token\s+TOK-SENSITIVE$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_host\s+eks-host-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_user\s+eks-user-1$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^private_eks_pass\s+eks-pass-1$`), res.Stdout)

		require.NotContains(t, res.Stdout, "**********")
		require.NotRegexp(t, regexp.MustCompile(`(?m)^region\s`), res.Stdout)
		require.NotContains(t, res.Stdout, "us-east-1")
	})
}
