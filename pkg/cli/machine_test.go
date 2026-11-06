package cli_test

import (
	"net/http"
	"net/http/httptest"
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

func testMachine(t *testing.T, fn func(*cli.Engine, *mocksdk.Interface)) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		rack.TestClient = machineClient{i}
		fn(e, i)
	})
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
				res.RequireStderr(t, []string{"ERROR: --max on Cloud machines needs a cpu or memory trigger; set the range with convox cloud services triggers enable web --min <count> --max <count> --cpu <percent>"})
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
