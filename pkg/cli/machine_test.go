package cli_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type machineClient struct {
	*mocksdk.Interface
}

func (machineClient) ClientType() string {
	return "machine"
}

func (machineClient) CloudMachine() string {
	return "m1"
}

func testMachine(t *testing.T, fn func(*cli.Engine, *mocksdk.Interface)) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestClient = machineClient{i}
		fn(e, i)
	})
}

var rackWord = regexp.MustCompile(`(?i)\brack`)

func requireCloudHints(t *testing.T, out string) {
	t.Helper()

	for _, line := range strings.Split(out, "\n") {
		require.NotRegexp(t, rackWord, line)
		require.Equal(t, strings.Count(line, "convox "), strings.Count(line, "convox cloud "), line)
	}
}

func TestMachineAppsParams(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxApp(), nil)

		res, err := testExecute(e, "apps params -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ParamFoo       value1",
			"ParamOther     value2",
			"ParamPassword  ****",
		})
	})
}

func TestMachineAppsParamsSet(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.AppUpdateOptions{
			Parameters: map[string]string{
				"Foo": "bar",
				"Baz": "qux",
			},
		}
		i.On("AppUpdate", "app1", opts).Return(nil)
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxAppParameters(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "apps params set Foo=bar Baz=qux -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Updating parameters... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
			"OK",
		})
	})
}

func TestMachineScaleMinMax(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(5)}
		i.On("ServiceList", "app1").Return(structs.Services{{Name: "web", TriggersOverrideActive: options.Bool(true)}}, nil)
		i.On("ServiceUpdate", "app1", "web", opts).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --min 1 --max 5 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
	})
}

func TestMachineScaleMinOnly(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ServiceUpdate", "app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2)}).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "scale web --min 2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
	})
}

func TestMachineRangeNeedsTrigger(t *testing.T) {
	for _, cmd := range []string{"scale web --min 1 --max 3 -a app1", "services update web --min 1 --max 3 -a app1"} {
		t.Run(cmd, func(t *testing.T) {
			testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ServiceList", "app1").Return(structs.Services{{Name: "web", TriggersOverrideActive: options.Bool(false)}, {Name: "worker"}}, nil)

				res, err := testExecute(e, cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, []string{"ERROR: --max on Cloud machines needs a cpu or memory trigger; set the range with convox cloud services triggers enable web --min <count> --max <count> --cpu <percent> -a app1 -i m1"})
			})
		})
	}
}

func TestMachineServicesTriggers(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		i.On("ServiceTriggersEnable", "app1", "web", opts, "").Return(nil)
		i.On("ServiceTriggersThresholdSet", "app1", "web", "cpu", 60.0, "").Return(nil)
		i.On("ServiceTriggersDisable", "app1", "web", "").Return(nil)

		res, err := testExecute(e, "services triggers enable web --min 1 --max 5 --cpu 70 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Enabling triggers override on web... OK"})

		res, err = testExecute(e, "services triggers threshold-set web --type cpu --threshold 60 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Setting web cpu threshold to 60... OK"})

		res, err = testExecute(e, "services triggers disable web -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Disabling triggers override on web... OK"})
	})
}

func TestMachineRefusals(t *testing.T) {
	cases := map[string]string{
		"build --external -a app1":                                               "ERROR: --external is not available on Cloud machines; run the command without it",
		"deploy --external -a app1":                                              "ERROR: --external is not available on Cloud machines; run the command without it",
		"resources proxy postgres1 -a app1":                                      "ERROR: resources proxy is not available on Cloud machines",
		"services triggers enable web --min 1 --max 3 --gpu 50 -a app1":          "ERROR: gpu and queue triggers are not available on Cloud machines",
		"services triggers enable web --min 1 --max 3 --queue 5 -a app1":         "ERROR: gpu and queue triggers are not available on Cloud machines",
		"services triggers threshold-set web --type gpu --threshold 50 -a app1":  "ERROR: gpu and queue triggers are not available on Cloud machines",
		"services triggers threshold-set web --type queue --threshold 5 -a app1": "ERROR: gpu and queue triggers are not available on Cloud machines",
	}

	for cmd, msg := range cases {
		t.Run(cmd, func(t *testing.T) {
			testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
				res, err := testExecute(e, cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, []string{msg})
				res.RequireStdout(t, []string{""})
			})
		})
	}
}

func TestMachineCommandsStayOnMachinePaths(t *testing.T) {
	extra := map[string][]string{
		"build":                           {"--external"},
		"deploy":                          {"--external"},
		"scale":                           {"web --min 1 --max 2"},
		"services triggers enable":        {"web --min 1 --max 2 --cpu 50"},
		"services triggers threshold-set": {"web --type cpu --threshold 50"},
	}

	total := 0

	for _, c := range cli.New("convox", "test").Commands {
		name, ok := strings.CutPrefix(strings.Join(c.Command, " "), "cloud ")
		if !ok || name == "machines" {
			continue
		}

		for _, args := range append([]string{"x1 x2"}, extra[name]...) {
			t.Run(name+" "+args, func(t *testing.T) {
				testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
					var mu sync.Mutex
					paths := []string{}

					ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						mu.Lock()
						paths = append(paths, r.URL.Path)
						mu.Unlock()
						http.Error(w, `{"error":"x"}`, http.StatusNotFound)
					}))
					defer ts.Close()

					mc, err := sdk.New(ts.URL)
					require.NoError(t, err)
					mc.MachineID = "m1"
					rack.TestClient = mc

					for k := range e.Commands {
						if strings.Join(e.Commands[k].Command, " ") == name {
							e.Commands[k].Validate = nil
						}
					}

					_, _ = testExecute(e, name+" "+args, nil)

					mu.Lock()
					defer mu.Unlock()
					total += len(paths)
					for _, p := range paths {
						require.Truef(t, p == "/machines" || strings.HasPrefix(p, "/machines/m1/"), "request outside the machine: %s", p)
					}
				})
			})
		}
	}

	require.NotZero(t, total)
}

func TestMachinePromoteHints(t *testing.T) {
	cases := []struct {
		name   string
		cmd    string
		mocks  func(*mocksdk.Interface)
		stderr []string
	}{
		{
			name: "rolled back",
			cmd:  "releases promote release2 -a app1",
			mocks: func(i *mocksdk.Interface) {
				i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
				i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{Force: options.Bool(false)}).Return(nil)
				i.On("AppGet", "app1").Return(fxAppAt("release2", "updating"), nil).Once()
				i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil)
			},
			stderr: []string{
				"ERROR: rollout failed for app1, the previous release was restored",
				"  convox cloud ps -a app1 -i m1",
			},
		},
		{
			name: "superseded",
			cmd:  "releases promote release2 -a app1",
			mocks: func(i *mocksdk.Interface) {
				i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
				i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{Force: options.Bool(false)}).Return(nil)
				i.On("AppGet", "app1").Return(fxAppAt("release3", "updating"), nil).Once()
				i.On("AppGet", "app1").Return(fxAppAt("release3", "running"), nil)
			},
			stderr: []string{
				"ERROR: release release2 for app1 was superseded by release3",
				"  convox cloud releases info release3 -a app1 -i m1",
			},
		},
		{
			name: "earlier rollout failed",
			cmd:  "releases promote release2 -a app1",
			mocks: func(i *mocksdk.Interface) {
				i.On("AppGet", "app1").Return(fxAppAt("release1", "updating"), nil).Once()
				i.On("AppGet", "app1").Return(fxAppAt("release1", "rollback"), nil)
			},
			stderr: []string{
				"ERROR: release release2 for app1 was not promoted, another rollout failed while it waited",
				"  convox cloud ps -a app1 -i m1",
			},
		},
		{
			name: "rollback superseded",
			cmd:  "releases rollback release2 -a app1",
			mocks: func(i *mocksdk.Interface) {
				i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
				i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(fxRelease3(), nil)
				i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
				i.On("ReleasePromote", "app1", "release3", structs.ReleasePromoteOptions{Force: options.Bool(false)}).Return(nil)
				i.On("AppGet", "app1").Return(fxAppAt("release4", "updating"), nil).Once()
				i.On("AppGet", "app1").Return(fxAppAt("release4", "running"), nil)
			},
			stderr: []string{
				"ERROR: release release3 for app1 was superseded by release4",
				"  convox cloud releases info release4 -a app1 -i m1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
				tc.mocks(i)
				i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil).Maybe()

				res, err := testExecute(e, tc.cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, tc.stderr)
				requireCloudHints(t, res.Stderr)
			})
		})
	}
}

func TestMachineBuildEnvDropHints(t *testing.T) {
	for _, guard := range []string{"strict", ""} {
		t.Run("guard="+guard, func(t *testing.T) {
			t.Setenv("CONVOX_ENV_DROP_GUARD", guard)
			testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("AppGet", "app1").Return(&structs.App{Name: "app1", Release: "release1", Status: "running", Generation: "2"}, nil)
				i.On("ReleaseList", "app1", structs.ReleaseListOptions{Limit: options.Int(1)}).Return(structs.Releases{{Id: "release2", App: "app1", Env: "FOO=bar", Created: fxStarted.Add(time.Hour)}}, nil)
				i.On("ReleaseGet", "app1", "release1").Return(&structs.Release{Id: "release1", App: "app1", Env: "SECRET_KEY=abc\nFOO=bar", Created: fxStarted}, nil)
				i.On("SystemGet").Return(fxSystem(), nil).Maybe()
				i.On("ObjectStore", "app1", mock.AnythingOfType("string"), mock.Anything, structs.ObjectStoreOptions{}).Return(&fxObject, nil).Maybe()
				i.On("BuildCreate", "app1", "object://test", structs.BuildCreateOptions{Description: options.String("foo")}).Return(fxBuild(), nil).Maybe()
				i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogs()), nil).Maybe()
				i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil).Maybe()
				i.On("BuildGet", "app1", "build4").Return(fxBuild(), nil).Maybe()

				res, err := testExecute(e, "build ./testdata/httpd -a app1 -d foo", nil)
				require.NoError(t, err)
				require.Contains(t, res.Stderr, "This usually happens after `convox cloud env set` or `convox cloud env unset` without --promote.")
				require.Contains(t, res.Stderr, "\n    convox cloud env set SECRET_KEY=... --promote -a app1 -i m1\n")
				requireCloudHints(t, res.Stderr)
			})
		})
	}
}

func TestMachineEnvMaskSetHint(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "env mask set API_TOKEN=secret -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: use `convox cloud env set KEY=VALUE -a app1 -i m1` to set env values. `convox cloud env mask set` takes key names only"})
	})
}

func TestMachineRunDetachedHint(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("bash")}).Return(fxProcess(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{"  convox cloud ps stop pid1 -a app1 -i m1"})
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
	})
}

func TestMachineRunDetachedWaitUnconfirmed(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("bash"), Retain: options.Int(60)}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(nil, fmt.Errorf("err1"))

		pr, pw := io.Pipe()
		t.Cleanup(func() { pw.Close() })
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(pr, nil).Maybe()

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "ERROR: could not confirm the outcome of process pid1: err1\n       convox cloud ps info pid1 -a app1 -i m1\n")
		requireCloudHints(t, res.Stderr)
	})
}

func TestMachineRunDetachedWaitWithoutExitStatus(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("bash"), Retain: options.Int(60)}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", nil), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: process pid1 ended with status failed without an exit status.",
			"       It may have been stopped before its command ran",
		})
	})
}

func TestMachineExecStreamIncomplete(t *testing.T) {
	cases := map[string]func(*mocksdk.Interface){
		"run web bash -a app1 -t 7200": func(i *mocksdk.Interface) {
			i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("sleep 7200")}).Return(fxProcess(), nil)
			i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)
			i.On("ProcessExec", "app1", "pid1", "bash", mock.Anything, structs.ProcessExecOptions{Entrypoint: options.Bool(true), Tty: options.Bool(false)}).Return(0, sdk.ErrExecIncomplete)
			i.On("ProcessStop", "app1", "pid1").Return(nil)
		},
		"exec 0123456789 bash -a app1": func(i *mocksdk.Interface) {
			i.On("ProcessExec", "app1", "0123456789", "bash", mock.Anything, structs.ProcessExecOptions{Tty: options.Bool(false)}).Return(0, sdk.ErrExecIncomplete)
		},
	}

	for cmd, mocks := range cases {
		t.Run(cmd, func(t *testing.T) {
			testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
				mocks(i)

				res, err := testExecute(e, cmd, strings.NewReader("in"))
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				require.Contains(t, res.Stderr, "ERROR: no exit status was reported for this command, so it may not have finished.")
				requireCloudHints(t, res.Stderr)
			})
		})
	}
}

func TestMachineCostTrackingDisabled(t *testing.T) {
	testMachine(t, func(e *cli.Engine, i *mocksdk.Interface) {
		fx := fxAppCost()
		fx.TrackingEnabled = false
		i.On("AppCost", "app1").Return(fx, nil)

		res, err := testExecute(e, "cost -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.True(t, strings.HasPrefix(res.Stdout, "Cost tracking is disabled. Values shown are the most-recent persisted snapshot and may be empty or stale.\n"), res.Stdout)
		requireCloudHints(t, res.Stdout)
	})
}
