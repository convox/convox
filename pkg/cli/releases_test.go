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

func TestReleases(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("ReleaseList", "app1", structs.ReleaseListOptions{}).Return(structs.Releases{*fxRelease(), *fxRelease2()}, nil)

		res, err := testExecute(e, "releases -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID        STATUS  BUILD   CREATED     DESCRIPTION",
			"release1  active  build1  2 days ago  description1",
			"release2          build1  2 days ago  ",
		})
	})
}

func TestReleasesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("ReleaseList", "app1", structs.ReleaseListOptions{}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "releases -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestReleasesInfo(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "releases info release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Id           release1",
			"Build        build1",
			fmt.Sprintf("Created      %s", fxRelease().Created.Format(time.RFC3339)),
			"Description  description1",
			"Env          FOO=bar",
			"             BAZ=quux",
		})
	})
}

func TestReleasesInfoError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "releases info release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestReleasesManifest(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)
		i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil)

		res, err := testExecute(e, "releases manifest release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"manifest1",
			"manifest2",
		})
	})
}

func TestReleasesManifestError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "releases manifest release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestReleasesPromote(t *testing.T) {
	testClientWait(t, 100*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxApp(), nil).Once()
		i.On("ReleasePromote", "app1", "release1", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Promoting release1... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
			"OK",
		})
	})
}

func TestReleasesPromoteError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("ReleasePromote", "app1", "release1", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "releases promote release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Promoting release1... "})
	})
}

func TestReleasesPromoteAlreadyUpdating(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxApp(), nil).Once()
		i.On("ReleasePromote", "app1", "release1", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxApp(), nil).Once()
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxApp(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Waiting for app to be ready... OK",
			"Promoting release1... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
			"OK",
		})
	})
}

func TestReleasesRollback(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(fxRelease3(), nil)
		i.On("ReleasePromote", "app1", "release3", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppUpdating(), nil).Twice()
		i.On("AppGet", "app1").Return(fxAppRelease3(), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases rollback release2 -a app1", nil)
		require.NoError(t, err)
		// require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Rolling back to release2... OK, release3",
			"Promoting release3... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
			"OK",
		})
	})
}

func TestReleasesRollbackErrorCreate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "releases rollback release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Rolling back to release2... "})
	})
}

func TestReleasesRollbackErrorPromote(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(fxRelease3(), nil)
		i.On("AppGet", "app1").Return(fxApp(), nil).Once()
		i.On("ReleasePromote", "app1", "release3", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "releases rollback release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{
			"Rolling back to release2... OK, release3",
			"Promoting release3... ",
		})
	})
}

func TestReleasesRollbackSuperseded(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(fxRelease3(), nil)
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
		i.On("ReleasePromote", "app1", "release3", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release4", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release4", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases rollback release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: release release3 for app1 was superseded by release4",
			"  convox releases info release4 -a app1",
		})
	})
}

func TestReleasesRollbackRolledBack(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release2").Return(fxRelease2(), nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String(fxRelease2().Build), Env: options.String(fxRelease2().Env)}).Return(fxRelease3(), nil)
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
		i.On("ReleasePromote", "app1", "release3", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release3", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases rollback release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: rollout failed for app1, the previous release was restored",
			"  convox deploy-debug -a app1",
		})
	})
}

func TestReleasesPromoteSuperseded(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
		i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release3", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release3", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: release release2 for app1 was superseded by release3",
			"  convox releases info release3 -a app1",
		})
		res.RequireStdout(t, []string{
			"Promoting release2... ",
			"TIME system/aws/component log1",
			"TIME system/aws/component log2",
		})
	})
}

func TestReleasesPromoteRolledBack(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
		i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release2", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: rollout failed for app1, the previous release was restored",
			"  convox deploy-debug -a app1",
		})
	})
}

func TestReleasesPromoteSettledEmpty(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Once()
		i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release2", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: rollout failed for app1, the previous release was restored",
			"  convox deploy-debug -a app1",
		})
	})
}

func TestReleasesPromoteWaitRolloutFailed(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppAt("release1", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release1", "rollback"), nil)

		res, err := testExecute(e, "releases promote release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: release release2 for app1 was not promoted, another rollout failed while it waited",
			"  convox deploy-debug -a app1",
		})
		res.RequireStdout(t, []string{"Waiting for app to be ready... "})
	})
}

func TestReleasesPromoteWaitRereadsPrevious(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("AppGet", "app1").Return(fxAppAt("release0", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release1", "running"), nil).Times(3)
		i.On("ReleasePromote", "app1", "release2", structs.ReleasePromoteOptions{
			Force: options.Bool(false),
		}).Return(nil)
		i.On("AppGet", "app1").Return(fxAppAt("release2", "updating"), nil).Once()
		i.On("AppGet", "app1").Return(fxAppAt("release0", "running"), nil)
		i.On("AppLogs", "app1", mock.Anything).Return(testLogs(fxLogsSystem()), nil)

		res, err := testExecute(e, "releases promote release2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"ERROR: release release2 for app1 was superseded by release0",
			"  convox releases info release0 -a app1",
		})
	})
}

func TestReleasesInfoRevealFlag(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		res, err := testExecute(e, "releases info release1 --reveal -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Id           release1",
			"Build        build1",
			fmt.Sprintf("Created      %s", fxRelease().Created.Format(time.RFC3339)),
			"Description  description1",
			"Env          FOO=bar",
			"             BAZ=quux",
		})
	})
}
