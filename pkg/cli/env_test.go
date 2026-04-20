package cli_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdcli"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "env -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"BAZ=quux",
			"FOO=bar",
		})
	})
}

func TestEnvError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "env -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestEnvGet(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "env get FOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"bar"})
	})
}

func TestEnvGetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "env get FOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestEnvGetMissing(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "env get FOOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: env not found: FOOO"})
		res.RequireStdout(t, []string{""})
	})
}

func TestEnvSet(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("AAA=bbb\nBAZ=quux\nCCC=ddd\nFOO=bar")}
		i.On("ReleaseCreate", "app1", ropts).Return(fxRelease(), nil)

		res, err := testExecute(e, "env set AAA=bbb CCC=ddd -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Setting AAA, CCC... OK",
			"Release: release1",
		})
	})
}

func TestEnvSetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("AAA=bbb\nBAZ=quux\nCCC=ddd\nFOO=bar")}
		i.On("ReleaseCreate", "app1", ropts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "env set AAA=bbb CCC=ddd -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Setting AAA, CCC... "})
	})
}

func TestEnvSetClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		i.On("EnvironmentSet", "app1", []byte("AAA=bbb\nBAZ=quux\nCCC=ddd\nFOO=bar")).Return(fxRelease(), nil)

		res, err := testExecute(e, "env set AAA=bbb CCC=ddd -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Setting AAA, CCC... OK",
			"Release: release1",
		})
	})
}

func TestEnvSetReplace(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("AAA=bbb\nCCC=ddd")}
		i.On("ReleaseCreate", "app1", ropts).Return(fxRelease(), nil)

		res, err := testExecute(e, "env set AAA=bbb CCC=ddd -a app1 --replace", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Setting AAA, CCC... OK",
			"Release: release1",
		})
	})
}

func TestEnvSetReplaceError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("AAA=bbb\nCCC=ddd")}
		i.On("ReleaseCreate", "app1", ropts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "env set AAA=bbb CCC=ddd -a app1 --replace", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Setting AAA, CCC... "})
	})
}

func TestEnvUnset(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("BAZ=quux")}
		i.On("ReleaseCreate", "app1", ropts).Return(fxRelease(), nil)

		res, err := testExecute(e, "env unset FOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Unsetting FOO... OK",
			"Release: release1",
		})
	})
}

func TestEnvUnsetError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		ropts := structs.ReleaseCreateOptions{Env: options.String("BAZ=quux")}
		i.On("ReleaseCreate", "app1", ropts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "env unset FOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Unsetting FOO... "})
	})
}

func TestEnvUnsetClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		i.On("EnvironmentUnset", "app1", "FOO").Return(fxRelease(), nil)

		res, err := testExecute(e, "env unset FOO -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Unsetting FOO... OK",
			"Release: release1",
		})
	})
}

func TestEnvMask(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN,DB_URL",
		}, nil)

		res, err := testExecute(e, "env mask -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"API_TOKEN",
			"DB_URL",
		})
	})
}

func TestEnvMaskEmpty(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(nil, fmt.Errorf("response status 404"))

		res, err := testExecute(e, "env mask -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

func TestEnvMaskSet(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(nil, fmt.Errorf("response status 404")).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "QVBJX1RPS0VO").Return(nil)
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN",
		}, nil).Once()

		res, err := testExecute(e, "env mask set API_TOKEN -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "Setting masked env keys")
		require.Contains(t, res.Stdout, "OK")
	})
}

func TestEnvMaskSetMerge(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN",
		}, nil).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "QVBJX1RPS0VOLERCX1VSTA==").Return(nil)
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN,DB_URL",
		}, nil).Once()

		res, err := testExecute(e, "env mask set DB_URL -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestEnvMaskSetOldRack(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(nil, fmt.Errorf("response status 404")).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "QVBJX1RPS0VO").Return(fmt.Errorf("response status 404"))

		res, err := testExecute(e, "env mask set API_TOKEN -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "rack version may not support env masking")
	})
}

func TestEnvMaskSetMissingApp(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "missing-app", "cli-env-mask").Return(nil, fmt.Errorf("app missing-app not found")).Once()
		i.On("AppConfigSet", "missing-app", "cli-env-mask", "QVBJX1RPS0VO").Return(fmt.Errorf("failed to set config: namespaces \"convox-missing-app\" not found"))

		res, err := testExecute(e, "env mask set API_TOKEN -a missing-app", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "namespaces")
		require.NotContains(t, res.Stdout, "rack version may not support")
	})
}

func TestEnvMaskSetInvalidFormat(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "env mask set API_TOKEN=secret -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "convox env set KEY=VALUE")
	})
}

func TestEnvMaskSetInvalidKeyName(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "env mask set 'BAD KEY' -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "invalid env key name")
	})
}

func TestEnvMaskSetRejectsControlChar(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "env mask set \"BAD\x1bKEY\" -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "control characters")
	})
}

func TestEnvMaskSetAcceptsDottedKey(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(nil, fmt.Errorf("app config not found")).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "Rk9PLkJBUg==").Return(nil)
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "FOO.BAR",
		}, nil).Once()

		res, err := testExecute(e, "env mask set FOO.BAR -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestEnvMaskUnset(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN,DB_URL",
		}, nil).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "QVBJX1RPS0VO").Return(nil)
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "API_TOKEN",
		}, nil).Once()

		res, err := testExecute(e, "env mask unset DB_URL -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "Unsetting masked env keys")
		require.Contains(t, res.Stdout, "OK")
	})
}

func TestEnvMaskUnsetOldRack(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(nil, fmt.Errorf("response status 404")).Once()
		i.On("AppConfigSet", "app1", "cli-env-mask", "").Return(fmt.Errorf("response status 404"))

		res, err := testExecute(e, "env mask unset DB_URL -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Contains(t, res.Stdout, "rack version may not support env masking")
	})
}

func TestEnvRevealFlag(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "env --reveal -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"BAZ=quux",
			"FOO=bar",
		})
	})
}

func TestEnvMaskedTTY(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		i.On("AppConfigGet", "app1", "cli-env-mask").Return(&structs.AppConfig{
			Name:  "cli-env-mask",
			Value: "BAZ",
		}, nil)

		res, err := testExecute(e, "env -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^BAZ=\*{4}$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^FOO=bar$`), res.Stdout)
		require.NotContains(t, res.Stdout, "BAZ=quux")
	})
}

func TestEnvRevealTTY(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.ReleaseListOptions{Limit: options.Int(1)}
		i.On("ReleaseList", "app1", opts).Return(structs.Releases{*fxRelease()}, nil)
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "env --reveal -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})

		require.Regexp(t, regexp.MustCompile(`(?m)^BAZ=quux$`), res.Stdout)
		require.Regexp(t, regexp.MustCompile(`(?m)^FOO=bar$`), res.Stdout)
		require.NotContains(t, res.Stdout, "****")
	})
}
