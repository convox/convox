package build_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/convox/convox/pkg/build"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/exec"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	bkEngine = &build.BuildKit{}
)

func TestBuild(t *testing.T) {
	opts := build.Options{
		App:      "app1",
		Auth:     "{}",
		Cache:    true,
		Id:       "build1",
		Rack:     "rack1",
		Source:   "object://app1/object.tgz",
		Push:     "registry.test.com",
		Manifest: "convox2.yml",
	}

	os.Setenv("PROVIDER", "do")
	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		p.On("BuildGet", "app1", "build1").Return(fxBuildStarted(), nil).Once()

		bdata, err := os.ReadFile("testdata/httpd.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(io.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := os.ReadFile("testdata/httpd/convox2.yml")
		require.NoError(t, err)

		p.On("BuildUpdate", "app1", "build1", mock.Anything).Return(fxBuildStarted(), nil).Run(func(args mock.Arguments) {
			opts := args.Get(2).(structs.BuildUpdateOptions)
			if opts.Ended != nil {
				require.False(t, opts.Ended.IsZero())
			}
			if opts.Logs != nil {
				require.NotNil(t, opts.Logs)
			}
			if opts.Manifest != nil {
				require.Equal(t, string(mdata), *opts.Manifest)
			}
			if opts.Entrypoint != nil {
				require.Equal(t, *opts.Entrypoint, "sh ./entry.sh")
			}
		})

		p.On("ReleaseList", "app1", structs.ReleaseListOptions{Limit: options.Int(1)}).Return(structs.Releases{*fxRelease()}, nil)
		p.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		e.On(
			"Run",
			mock.Anything,
			"buildctl", "build", "--frontend", "dockerfile.v0", "--local", mock.MatchedBy(matchContext), "--local", mock.MatchedBy(matchDockerfile),
			"--opt", mock.MatchedBy(matchFilename), "--output", mock.MatchedBy(matchTag),
			"--export-cache", "type=registry,ref=registry.test.com:buildcache",
			"--import-cache", "type=registry,ref=registry.test.com:buildcache",
			"--build-arg:", "FOO=bar",
		).Return(nil).Run(func(args mock.Arguments) {
			fmt.Fprintf(args.Get(0).(io.Writer), "build1\nbuild2\n")
		})

		e.On(
			"Execute",
			"skopeo",
			"inspect",
			"--config",
			"docker://registry.test.com:web.build1",
		).Return(fxSkopeoInspect(), nil)

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := io.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)
	})
}

func TestBuildDevelopment(t *testing.T) {
	opts := build.Options{
		App:         "app1",
		Auth:        "{}",
		Cache:       true,
		Development: true,
		Id:          "build1",
		Rack:        "rack1",
		Source:      "object://app1/object.tgz",
		Push:        "registry.test.com",
	}

	os.Setenv("PROVIDER", "do")
	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		p.On("BuildGet", "app1", "build1").Return(fxBuildStarted(), nil).Once()

		bdata, err := os.ReadFile("testdata/httpd-dev.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(io.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := os.ReadFile("testdata/httpd-dev/convox.yml")
		require.NoError(t, err)

		p.On("BuildUpdate", "app1", "build1", mock.Anything).Return(fxBuildStarted(), nil).Run(func(args mock.Arguments) {
			opts := args.Get(2).(structs.BuildUpdateOptions)
			if opts.Ended != nil {
				require.False(t, opts.Ended.IsZero())
			}
			if opts.Logs != nil {
				require.NotNil(t, opts.Logs)
			}
			if opts.Manifest != nil {
				require.Equal(t, string(mdata), *opts.Manifest)
			}
			if opts.Entrypoint != nil {
				require.Equal(t, *opts.Entrypoint, "sh ./entry.sh")
			}
		})

		p.On("ReleaseList", "app1", structs.ReleaseListOptions{Limit: options.Int(1)}).Return(structs.Releases{*fxRelease()}, nil)
		p.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		e.On(
			"Run",
			mock.Anything,
			"buildctl", "build", "--frontend", "dockerfile.v0", "--local", mock.MatchedBy(matchContext), "--local", mock.MatchedBy(matchDockerfile),
			"--opt", mock.MatchedBy(matchFilename), "--output", mock.MatchedBy(matchTag),
			"--export-cache", "type=registry,ref=registry.test.com:buildcache",
			"--import-cache", "type=registry,ref=registry.test.com:buildcache",
			"--target", "development",
		).Return(nil).Run(func(args mock.Arguments) {
			fmt.Fprintf(args.Get(0).(io.Writer), "build1\nbuild2\n")
		})

		e.On(
			"Execute",
			"skopeo",
			"inspect",
			"--config",
			"docker://registry.test.com:web.build1",
		).Return(fxSkopeoInspect(), nil)

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := io.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)
	})
}

func TestBuildOptions(t *testing.T) {
	opts := build.Options{
		App:      "app1",
		Auth:     "{}",
		Cache:    true,
		Id:       "build1",
		Rack:     "rack1",
		Source:   "object://app1/object.tgz",
		Push:     "registry.test.com",
		Manifest: "convox2.yml",
	}

	os.Setenv("PROVIDER", "do")
	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		p.On("BuildGet", "app1", "build1").Return(fxBuildStarted(), nil).Once()

		bdata, err := os.ReadFile("testdata/httpd.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(io.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := os.ReadFile("testdata/httpd/convox2.yml")
		require.NoError(t, err)

		p.On("BuildUpdate", "app1", "build1", mock.Anything).Return(fxBuildStarted(), nil).Run(func(args mock.Arguments) {
			opts := args.Get(2).(structs.BuildUpdateOptions)
			if opts.Ended != nil {
				require.False(t, opts.Ended.IsZero())
			}
			if opts.Logs != nil {
				require.NotNil(t, opts.Logs)
			}
			if opts.Manifest != nil {
				require.Equal(t, string(mdata), *opts.Manifest)
			}
			if opts.Entrypoint != nil {
				require.Equal(t, *opts.Entrypoint, "sh ./entry.sh")
			}
		})

		p.On("ReleaseList", "app1", structs.ReleaseListOptions{Limit: options.Int(1)}).Return(structs.Releases{*fxRelease()}, nil)
		p.On("ReleaseGet", "app1", "release1").Return(fxRelease(), nil)

		e.On(
			"Run",
			mock.Anything,
			"buildctl", "build", "--frontend", "dockerfile.v0", "--local", mock.MatchedBy(matchContext), "--local", mock.MatchedBy(matchDockerfile),
			"--opt", mock.MatchedBy(matchFilename), "--output", mock.MatchedBy(matchTag),
			"--export-cache", "type=registry,ref=registry.test.com:buildcache",
			"--import-cache", "type=registry,ref=registry.test.com:buildcache",
			"--build-arg:", "FOO=bar",
		).Return(nil).Run(func(args mock.Arguments) {
			fmt.Fprintf(args.Get(0).(io.Writer), "build1\nbuild2\n")
		})

		e.On(
			"Execute",
			"skopeo",
			"inspect",
			"--config",
			"docker://registry.test.com:web.build1",
		).Return([]byte("''"), nil)

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := io.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)
	})
}

func TestLogin(t *testing.T) {
	tmp, err := os.MkdirTemp("", "convox-tests")
	if err != nil {
		panic(err)
	}
	err = os.Mkdir(fmt.Sprintf("%s/.docker", tmp), 0600)
	if err != nil {
		panic(err)
	}

	defer os.Remove(tmp)
	os.Setenv("HOME", tmp)

	r, err := sdk.NewFromEnv()
	if err != nil {
		panic(err)
	}

	bk := build.BuildKit{}
	opts := build.Options{
		App:      "app1",
		Auth:     `{"host1":{"username":"user1","password":"pass1"}}`,
		Cache:    true,
		Id:       "build1",
		Manifest: "convox2.yml",
		Push:     "push1",
		Rack:     "rack1",
		Source:   "object://app1/object.tgz",
	}
	bb, err := build.New(r, opts, &bk)
	if err != nil {
		panic(err)
	}

	err = bk.Login(bb)
	if err != nil {
		panic(err)
	}

	f, err := os.Open(fmt.Sprintf("%s/.docker/config.json", tmp))
	if err != nil {
		panic(err)
	}

	data, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// base64 encoded user:password
	require.Equal(t, string(data), `{"Auths":{"host1":{"auth":"dXNlcjE6cGFzczE="}}}`)
}

func fxSkopeoInspect() []byte {
	return []byte(`{
		"config": {
			"Entrypoint": [
				"sh",
				"./entry.sh"
			]
		}
	}`)
}
