package cli_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("ObjectStore", "app1", mock.AnythingOfType("string"), mock.Anything, structs.ObjectStoreOptions{}).Return(&fxObject, nil).Run(func(args mock.Arguments) {
			require.Regexp(t, `tmp/[0-9a-f]{30}\.tgz`, args.Get(1).(string))
		})
		i.On("BuildCreate", "app1", "object://test", structs.BuildCreateOptions{Description: options.String("foo")}).Return(fxBuild(), nil)
		i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogs()), nil).Once()
		i.On("BuildGet", "app1", "build1").Return(fxBuildRunning(), nil).Once()
		i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil)
		i.On("BuildGet", "app1", "build4").Return(fxBuild(), nil)
		i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogs()), nil)

		res, err := testExecute(e, "build ./testdata/httpd -a app1 -d foo", nil)
		require.NoError(t, err)
		// require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Packaging source... OK",
			"Uploading source... OK",
			"Starting build... OK",
			"log1",
			"log2",
			"Build:   build1",
			"Release: release1",
		})
	})
}

func TestBuildFinalizeLogs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// i.On("ClientType").Return("standard")
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("ObjectStore", "app1", mock.AnythingOfType("string"), mock.Anything, structs.ObjectStoreOptions{}).Return(&fxObject, nil).Run(func(args mock.Arguments) {
			require.Regexp(t, `tmp/[0-9a-f]{30}\.tgz`, args.Get(1).(string))
		})
		i.On("BuildCreate", "app1", "object://test", structs.BuildCreateOptions{Description: options.String("foo")}).Return(fxBuild(), nil)
		i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogs()), nil).Once()
		i.On("BuildGet", "app1", "build1").Return(fxBuildRunning(), nil).Once()
		i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil)
		i.On("BuildGet", "app1", "build4").Return(fxBuild(), nil)
		i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogsLonger()), nil)

		res, err := testExecute(e, "build ./testdata/httpd -a app1 -d foo", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Packaging source... OK",
			"Uploading source... OK",
			"Starting build... OK",
			"log1",
			"log2",
			"log3",
			"Build:   build1",
			"Release: release1",
		})
	})
}

func TestBuildError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// i.On("ClientType").Return("standard")
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("ObjectStore", "app1", mock.AnythingOfType("string"), mock.Anything, structs.ObjectStoreOptions{}).Return(&fxObject, nil).Run(func(args mock.Arguments) {
			require.Regexp(t, `tmp/[0-9a-f]{30}\.tgz`, args.Get(1).(string))
		})
		i.On("BuildCreate", "app1", "object://test", structs.BuildCreateOptions{Description: options.String("foo")}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "build ./testdata/httpd -a app1 -d foo", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{
			"Packaging source... OK",
			"Uploading source... OK",
			"Starting build... ",
		})
	})
}

func TestBuildClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		// i.On("ClientType").Return("standard")
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		i.On("BuildCreateUpload", "app1", mock.Anything, structs.BuildCreateOptions{Description: options.String("foo")}).Return(fxBuild(), nil)
		i.On("BuildLogs", "app1", "build1", structs.LogsOptions{}).Return(testLogs(fxLogs()), nil)
		i.On("BuildGet", "app1", "build1").Return(fxBuildRunning(), nil).Once()
		i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil)
		i.On("BuildGet", "app1", "build4").Return(fxBuild(), nil)

		res, err := testExecute(e, "build ./testdata/httpd -a app1 -d foo", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Packaging source... OK",
			"Starting build... OK",
			"log1",
			"log2",
			"Build:   build1",
			"Release: release1",
		})
	})
}

func TestBuilds(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		b1 := structs.Builds{
			*fxBuild(),
			*fxBuildRunning(),
			*fxBuildFailed(),
		}
		i.On("BuildList", "app1", structs.BuildListOptions{}).Return(b1, nil)

		res, err := testExecute(e, "builds -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID      STATUS    RELEASE   STARTED     ELAPSED  DESCRIPTION",
			"build1  complete  release1  2 days ago  2m0s     desc",
			"build4  running             2 days ago           ",
			"build3  failed              2 days ago           ",
		})
	})
}

func TestBuildsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BuildList", "app1", structs.BuildListOptions{}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "builds -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestBuildsExport(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		data, err := os.ReadFile("testdata/build.tgz")
		require.NoError(t, err)
		i.On("BuildExport", "app1", "build1", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			args.Get(2).(io.Writer).Write(data) //nolint:errcheck // mock type assertion
		})
		tmpd, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		tmpf := filepath.Join(tmpd, "export.tgz")

		res, err := testExecute(e, fmt.Sprintf("builds export build1 -a app1 -f %s", tmpf), nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Exporting build... OK"})
		tdata, err := os.ReadFile(tmpf)
		require.NoError(t, err)
		require.Equal(t, data, tdata)
	})
}

func TestBuildsExportStdout(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		data, err := os.ReadFile("testdata/build.tgz")
		require.NoError(t, err)
		i.On("BuildExport", "app1", "build1", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			args.Get(2).(io.Writer).Write(data) //nolint:errcheck // mock type assertion
		})

		res, err := testExecute(e, "builds export build1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{"Exporting build... OK"})
		require.Equal(t, data, []byte(res.Stdout))
	})
}

func TestBuildsExportError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BuildExport", "app1", "build1", mock.Anything).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "builds export build1 -a app1 -f /dev/null", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Exporting build... "})
	})
}

func TestBuildsImport(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		data, err := os.ReadFile("testdata/build.tgz")
		require.NoError(t, err)
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("BuildImport", "app1", mock.Anything).Return(fxBuild(), nil).Run(func(args mock.Arguments) {
			rdata, err := io.ReadAll(args.Get(1).(io.Reader)) //nolint:errcheck // mock type assertion
			require.NoError(t, err)
			require.Equal(t, data, rdata)
		})

		res, err := testExecute(e, "builds import -a app1 -f testdata/build.tgz", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Importing build... OK, release1"})
	})
}

func TestBuildsImportError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("BuildImport", "app1", mock.Anything).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "builds import -a app1 -f testdata/build.tgz", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Importing build... "})
	})
}

func TestBuildsImportImage(t *testing.T) {
	testClientWait(t, 10*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		manifestData, err := os.ReadFile("testdata/import-manifest/convox.yml")
		require.NoError(t, err)

		bExternal := &structs.Build{Id: "build1", App: "app1", Status: "created"}
		bRunning := &structs.Build{Id: "build1", App: "app1", Status: "running"}
		bComplete := &structs.Build{Id: "build1", App: "app1", Status: "complete"}

		i.On("BuildCreate", "app1", "", structs.BuildCreateOptions{External: options.Bool(true)}).Return(bExternal, nil)
		i.On("BuildUpdate", "app1", "build1", structs.BuildUpdateOptions{Manifest: options.String(string(manifestData))}).Return(bExternal, nil).Once()
		i.On("BuildImportImage", "app1", "build1", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}).Return(nil)
		i.On("BuildGet", "app1", "build1").Return(bRunning, nil).Once()
		i.On("BuildGet", "app1", "build1").Return(bComplete, nil)
		i.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(&structs.Release{Id: "release1"}, nil)
		i.On("BuildUpdate", "app1", "build1", structs.BuildUpdateOptions{Release: options.String("release1")}).Return(bComplete, nil).Once()

		res, err := testExecute(e, "builds import-image vllm/vllm-openai:v0.6.3 -a app1 -m testdata/import-manifest/convox.yml", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{
			"Creating build... OK, build1",
			"Relaying image vllm/vllm-openai:v0.6.3... OK",
			"Waiting for import to complete... OK",
			"Creating release... OK, release1",
			"Build:   build1",
			"Release: release1",
		})
	})
}

func TestBuildsImportImageFailure(t *testing.T) {
	testClientWait(t, 10*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		bExternal := &structs.Build{Id: "build1", App: "app1", Status: "created"}
		bFailed := &structs.Build{Id: "build1", App: "app1", Status: "failed", Reason: "manifest unknown"}

		i.On("BuildCreate", "app1", "", structs.BuildCreateOptions{External: options.Bool(true)}).Return(bExternal, nil)
		i.On("BuildUpdate", "app1", "build1", mock.Anything).Return(bExternal, nil).Once()
		i.On("BuildImportImage", "app1", "build1", "bad/image:1", structs.BuildImportImageOptions{}).Return(nil)
		i.On("BuildGet", "app1", "build1").Return(bFailed, nil)

		res, err := testExecute(e, "builds import-image bad/image:1 -a app1 -m testdata/import-manifest/convox.yml", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: import failed: manifest unknown"})
	})
}

func TestBuildsImportImageWithCreds(t *testing.T) {
	testClientWait(t, 10*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		bExternal := &structs.Build{Id: "build1", App: "app1", Status: "created"}
		bComplete := &structs.Build{Id: "build1", App: "app1", Status: "complete"}

		opts := structs.BuildImportImageOptions{
			SrcCredsUser: options.String("$oauthtoken"),
			SrcCredsPass: options.String("nvapi-key"),
		}

		i.On("BuildCreate", "app1", "", structs.BuildCreateOptions{External: options.Bool(true)}).Return(bExternal, nil)
		i.On("BuildUpdate", "app1", "build1", mock.Anything).Return(bExternal, nil).Once()
		i.On("BuildImportImage", "app1", "build1", "nvcr.io/nim/x:1.0", opts).Return(nil)
		i.On("BuildGet", "app1", "build1").Return(bComplete, nil)
		i.On("ReleaseCreate", "app1", mock.Anything).Return(&structs.Release{Id: "release1"}, nil)
		i.On("BuildUpdate", "app1", "build1", mock.MatchedBy(func(o structs.BuildUpdateOptions) bool {
			return o.Release != nil && *o.Release == "release1"
		})).Return(bComplete, nil).Once()

		res, err := testExecute(e, "builds import-image nvcr.io/nim/x:1.0 -a app1 -m testdata/import-manifest/convox.yml --src-creds-user $oauthtoken --src-creds-pass nvapi-key", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestBuildsImportClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		data, err := os.ReadFile("testdata/build.tgz")
		require.NoError(t, err)
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		i.On("BuildImportMultipart", "app1", mock.Anything).Return(fxBuild(), nil).Run(func(args mock.Arguments) {
			rdata, err := io.ReadAll(args.Get(1).(io.Reader)) //nolint:errcheck // mock type assertion
			require.NoError(t, err)
			require.Equal(t, data, rdata)
		})

		res, err := testExecute(e, "builds import -a app1 -f testdata/build.tgz", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Importing build... OK, release1"})
	})
}

func TestBuildsInfo(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BuildGet", "app1", "build1").Return(fxBuild(), nil)

		res, err := testExecute(e, "builds info build1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Id           build1",
			"Status       complete",
			"Release      release1",
			"Description  desc",
			"Started      2 days ago",
			"Elapsed      2m0s",
		})
	})
}

func TestBuildsInfoError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BuildGet", "app1", "build1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "builds info build1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestBuildsLogs(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.LogsOptions{}
		i.On("BuildLogs", "app1", "build1", opts).Return(testLogs(fxLogs()), nil)

		res, err := testExecute(e, "builds logs build1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			fxLogs()[0],
			fxLogs()[1],
		})
	})
}

func TestBuildsLogsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		opts := structs.LogsOptions{}
		i.On("BuildLogs", "app1", "build1", opts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "builds logs build1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}
