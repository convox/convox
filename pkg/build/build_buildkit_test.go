package build_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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

		bdata, err := ioutil.ReadFile("testdata/httpd.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(ioutil.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := ioutil.ReadFile("testdata/httpd/convox2.yml")
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

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := ioutil.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
			// require.Equal(t, "Building: .\nbuild1\nbuild2\nRunning: docker pull httpd\nRunning: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1\nRunning: docker tag httpd rack1/app1:web.build1\n", string(data))
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)

		// require.Equal(t,
		// 	[]string{
		// 		"Building: .",
		// 		"build1",
		// 		"build2",
		// 		"Running: docker pull httpd",
		// 		"Running: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1",
		// 		"Running: docker tag httpd rack1/app1:web.build1",
		// 	},
		// 	strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n"),
		// )
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

		bdata, err := ioutil.ReadFile("testdata/httpd-dev.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(ioutil.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := ioutil.ReadFile("testdata/httpd-dev/convox.yml")
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

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := ioutil.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
			// require.Equal(t, "Building: .\nbuild1\nbuild2\nRunning: docker pull httpd\nRunning: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1\nRunning: docker tag httpd rack1/app1:web.build1\n", string(data))
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)

		// require.Equal(t,
		// 	[]string{
		// 		"Building: .",
		// 		"build1",
		// 		"build2",
		// 		"Running: docker pull httpd",
		// 		"Running: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1",
		// 		"Running: docker tag httpd rack1/app1:web.build1",
		// 	},
		// 	strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n"),
		// )
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

		bdata, err := ioutil.ReadFile("testdata/httpd.tgz")
		require.NoError(t, err)
		p.On("ObjectFetch", "app1", "/object.tgz").Return(ioutil.NopCloser(bytes.NewReader(bdata)), nil)

		mdata, err := ioutil.ReadFile("testdata/httpd/convox2.yml")
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

		p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil).Run(func(args mock.Arguments) {
			_, err := ioutil.ReadAll(args.Get(2).(io.Reader))
			require.NoError(t, err)
			// require.Equal(t, "Building: .\nbuild1\nbuild2\nRunning: docker pull httpd\nRunning: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1\nRunning: docker tag httpd rack1/app1:web.build1\n", string(data))
		})
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
		p.On("EventSend", "build:create", structs.EventSendOptions{Data: map[string]string{"app": "app1", "id": "build1", "release_id": "release2"}}).Return(nil)

		err = b.Execute()
		require.NoError(t, err)

		// require.Equal(t,
		// 	[]string{
		// 		"Building: .",
		// 		"build1",
		// 		"build2",
		// 		"Running: docker pull httpd",
		// 		"Running: docker tag e00bc968ebe3f5b4c934a1f3c00fcfba74384f944f6f9fa2ba819445 rack1/app1:web2.build1",
		// 		"Running: docker tag httpd rack1/app1:web.build1",
		// 	},
		// 	strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n"),
		// )
	})
}

func TestLogin(t *testing.T) {
	tmp, err := os.MkdirTemp("", "convox-tests")
	if err != nil {
		panic(err)
	}
	err = os.Mkdir(fmt.Sprintf("%s/.docker", tmp), 0777)
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

	data, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	// base64 encoded user:password
	require.Equal(t, string(data), `{"Auths":{"host1":{"auth":"dXNlcjE6cGFzczE="}}}`)
}
