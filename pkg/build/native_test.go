package build_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/build"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/exec"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const helperAddr = "tcp://build-build1-arm64.rack1-app1.svc.cluster.local:1234"

// commands stands in for buildctl on the per-architecture path.
type commands struct {
	mu         sync.Mutex
	builds     [][]string
	probes     int
	dockerfile string

	probe func(ctx context.Context, n int) bool
	build func(ctx context.Context, w io.Writer, remote bool, arch string) error
}

func (c *commands) run(ctx context.Context, w io.Writer, name string, args ...string) error {
	if name != "buildctl" {
		return fmt.Errorf("unexpected command: %s", name)
	}

	if args[len(args)-1] == "workers" {
		c.mu.Lock()
		c.probes++
		n := c.probes
		c.mu.Unlock()

		if len(args) != 12 || !helperTarget(args) || strings.Join(args[8:], " ") != "--timeout 5 debug workers" {
			return fmt.Errorf("probe did not target the helper")
		}

		if c.probe == nil || c.probe(ctx, n) {
			return nil
		}
		return fmt.Errorf("unreachable")
	}

	if args[0] != "build" && !helperTarget(args) {
		return fmt.Errorf("build did not target the helper")
	}

	c.mu.Lock()
	c.builds = append(c.builds, args)
	c.mu.Unlock()

	arch, metadata, dir := "", "", ""
	for i, a := range args {
		switch {
		case strings.HasPrefix(a, "platform=linux/"):
			arch = strings.TrimPrefix(a, "platform=linux/")
		case a == "--metadata-file":
			metadata = args[i+1]
		case strings.HasPrefix(a, "context="):
			dir = strings.TrimPrefix(a, "context=")
		}
	}

	if strings.Contains(arch, ",") {
		data, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.dockerfile = string(data)
		c.mu.Unlock()
		return nil
	}

	if c.build != nil {
		if err := c.build(ctx, w, args[0] == "--addr", arch); err != nil {
			return err
		}
	} else {
		for i := 0; i < 50; i++ {
			fmt.Fprintf(w, "#%d step ", i)
			fmt.Fprintf(w, "for %s\n", arch)
		}
		fmt.Fprintf(w, "#50 done %s", arch)
	}

	return os.WriteFile(metadata, []byte(fmt.Sprintf(`{"containerimage.digest":"sha256:%s"}`, arch)), 0600)
}

func helperTarget(args []string) bool {
	if len(args) < 8 || args[0] != "--addr" || args[1] != os.Getenv("BUILD_HELPER_ADDR") {
		return false
	}
	for i, f := range [][2]string{{"--tlscacert", "BUILD_HELPER_CA"}, {"--tlscert", "BUILD_HELPER_CERT"}, {"--tlskey", "BUILD_HELPER_KEY"}} {
		data, err := os.ReadFile(args[3+2*i])
		if args[2+2*i] != f[0] || err != nil || string(data) != os.Getenv(f[1]) {
			return false
		}
	}
	return true
}

func (c *commands) invocations() (local, remote map[string][]string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	local, remote = map[string][]string{}, map[string][]string{}
	for _, args := range c.builds {
		for _, a := range args {
			if strings.HasPrefix(a, "platform=") {
				arch := strings.ReplaceAll(strings.TrimPrefix(a, "platform="), "linux/", "")
				if args[0] == "--addr" {
					remote[arch] = args
				} else {
					local[arch] = args
				}
			}
		}
	}

	return local, remote
}

func helperEnv(t *testing.T) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	kder, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))

	t.Setenv("PROVIDER", "aws")
	t.Setenv("DISABLE_IMAGE_MANIFEST_CACHE", "false")
	t.Setenv("BUILD_ARCHS", "amd64,arm64")
	t.Setenv("BUILD_HELPER_ADDR", helperAddr)
	t.Setenv("BUILD_HELPER_ARCH", "arm64")
	t.Setenv("BUILD_HELPER_CA", cert)
	t.Setenv("BUILD_HELPER_CERT", cert)
	t.Setenv("BUILD_HELPER_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder})))
}

func nativeSource(t *testing.T, manifest string, dockerfiles map[string]string) build.Options {
	t.Helper()

	src := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(src, "convox.yml"), []byte(manifest), 0600))
	for name, data := range dockerfiles {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(src, name)), 0700))
		require.NoError(t, os.WriteFile(filepath.Join(src, name), []byte(data), 0600))
	}

	return build.Options{
		App:    "app1",
		Auth:   "{}",
		Cache:  true,
		Id:     "build1",
		Rack:   "rack1",
		Source: "dir://" + src,
		Push:   "registry.test.com",
	}
}

func nativeProvider(p *structs.MockProvider, ok bool) {
	p.On("BuildGet", "app1", "build1").Return(fxBuildStarted(), nil).Once()
	p.On("BuildUpdate", "app1", "build1", mock.Anything).Return(fxBuildStarted(), nil)
	p.On("ReleaseList", "app1", structs.ReleaseListOptions{Limit: options.Int(1)}).Return(structs.Releases{}, nil)
	p.On("ObjectStore", "app1", "build/build1/logs", mock.Anything, structs.ObjectStoreOptions{}).Return(fxObject(), nil)
	if ok {
		p.On("ReleaseCreate", "app1", structs.ReleaseCreateOptions{Build: options.String("build1")}).Return(fxRelease2(), nil)
	}
	p.On("EventSend", "build:create", mock.Anything).Return(nil)
}

const oneService = "services:\n  web:\n    build: .\n"

func TestBuildNative(t *testing.T) {
	helperEnv(t)
	t.Setenv("BUILDKIT_HOST_PATH_CACHE_ENABLE", "true")

	c := &commands{}
	build.StubCommands(t, c.run)

	opts := nativeSource(t, oneService, map[string]string{"Dockerfile": "FROM alpine\nARG FOO\n"})
	opts.BuildArgs = []string{"FOO=bar"}
	opts.Cache = false

	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		local, remote := c.invocations()
		require.Len(t, local, 2)
		require.Len(t, remote, 1)

		src := strings.TrimPrefix(opts.Source, "dir://")
		common := func(arch string) []string {
			return []string{
				"build", "--frontend", "dockerfile.v0", "--local", "context=" + src, "--local", "dockerfile=" + src, "--opt", "filename=Dockerfile",
				"--output", "type=image,name=registry.test.com,push-by-digest=true,name-canonical=true,push=true",
				"--opt", "platform=linux/" + arch,
			}
		}
		registry := func(arch string) []string {
			return []string{
				"--export-cache", "mode=max,image-manifest=true,oci-mediatypes=true,ignore-error=true,compression=estargz,type=registry,ref=registry.test.com:web.buildcache-" + arch,
				"--import-cache", "type=registry,ignore-error=true,ref=registry.test.com:web.buildcache-" + arch,
				"--opt", "build-arg:FOO=bar", "--no-cache",
			}
		}

		amd := local["amd64"]
		require.Equal(t, common("amd64"), amd[:13])
		require.Equal(t, "--metadata-file", amd[13])
		require.Equal(t, append([]string{
			"--import-cache", "type=local,src=/var/lib/buildkit",
			"--export-cache", "type=local,dest=/var/lib/buildkit",
		}, registry("amd64")...), amd[15:])

		arm := remote["arm64"]
		require.Equal(t, []string{"--addr", helperAddr, "--tlscacert", "--tlscert", "--tlskey"}, []string{arm[0], arm[1], arm[2], arm[4], arm[6]})
		require.Equal(t, common("arm64"), arm[8:21])
		require.Equal(t, registry("arm64"), arm[23:])

		index := local["amd64,arm64"]
		require.Equal(t, []string{"--opt", "filename=Dockerfile", "--opt", "platform=linux/amd64,linux/arm64", "--output", "type=image,name=registry.test.com:web.build1,push=true"}, index[7:])
		require.Equal(t, "FROM registry.test.com@sha256:amd64 AS amd64\nFROM registry.test.com@sha256:arm64 AS arm64\nFROM ${TARGETARCH}\n", c.dockerfile)

		for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
			if strings.Contains(line, "step for") {
				arch := line[strings.LastIndex(line, " ")+1:]
				require.True(t, strings.HasPrefix(line, "["+arch+"] #"), line)
			}
		}
		require.Equal(t, 100, strings.Count(out.String(), " step for "))
		require.Equal(t, 51, strings.Count(out.String(), "[amd64] #"))
		require.Equal(t, 51, strings.Count(out.String(), "[arm64] #"))
		require.Contains(t, out.String(), "[amd64] #50 done amd64\n")
		require.Contains(t, out.String(), "[arm64] #50 done arm64\n")
		require.NotContains(t, out.String(), "WARNING")
	})
}

func TestBuildNativeImmutableCacheRefs(t *testing.T) {
	helperEnv(t)
	t.Setenv("ECR_IMMUTABLE_TAGS_ENABLED", "true")

	c := &commands{}
	build.StubCommands(t, c.run)

	testBuild(t, nativeSource(t, oneService, map[string]string{"Dockerfile": "FROM alpine\n"}), bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		p.On("BuildList", "app1", structs.BuildListOptions{Limit: options.Int(50)}).Return(structs.Builds{{Id: "B3", Status: "complete"}}, nil).Once()
		e.On("Execute", "skopeo", "--command-timeout", "30s", "inspect", "--raw", "docker://registry.test.com:web.buildcache-amd64.B3").Return([]byte("{}"), nil).Once()
		e.On("Execute", "skopeo", "--command-timeout", "30s", "inspect", "--raw", "docker://registry.test.com:web.buildcache-arm64.B3").Return([]byte(nil), fmt.Errorf("manifest unknown")).Once()
		e.On("Execute", "skopeo", "--command-timeout", "30s", "inspect", "--raw", "docker://registry.test.com:web.buildcache-arm64").Return([]byte(nil), fmt.Errorf("manifest unknown")).Once()
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		local, remote := c.invocations()
		require.Contains(t, local, "amd64")
		require.Contains(t, remote, "arm64")
		export := "mode=max,image-manifest=true,oci-mediatypes=true,ignore-error=true,compression=estargz,type=registry,ref=registry.test.com:web.buildcache-"
		require.Equal(t, []string{"--export-cache", export + "amd64.build1", "--import-cache", "type=registry,ignore-error=true,ref=registry.test.com:web.buildcache-amd64.B3"}, local["amd64"][15:])
		require.Equal(t, []string{"--export-cache", export + "arm64.build1"}, remote["arm64"][23:])
	})
}

func TestBuildNativeHelperNeverReachable(t *testing.T) {
	helperEnv(t)

	fallback := 0

	c := &commands{probe: func(context.Context, int) bool { return false }}
	c.build = func(_ context.Context, _ io.Writer, _ bool, arch string) error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if arch == "arm64" && fallback == 0 {
			fallback = c.probes
		}
		return nil
	}
	build.StubCommands(t, c.run)

	manifest := "services:\n  web:\n    build: .\n  worker:\n    build: ./worker\n"
	opts := nativeSource(t, manifest, map[string]string{"Dockerfile": "FROM alpine\n", "worker/Dockerfile": "FROM alpine\n"})

	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:worker.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		c.mu.Lock()
		defer c.mu.Unlock()

		require.Len(t, c.builds, 6)
		for _, args := range c.builds {
			require.Equal(t, "build", args[0])
		}

		require.Equal(t, 2, strings.Count(out.String(), "WARNING: no arm64 build node is reachable, building arm64 on this node under emulation\n"))

		require.Positive(t, fallback)
		require.Equal(t, fallback, c.probes)
	})
}

func TestBuildNativeHelperFailures(t *testing.T) {
	tests := []struct {
		name    string
		probe   func(ctx context.Context, n int) bool
		remote  func(ctx context.Context, w io.Writer) error
		primary error
		err     string
		rebuilt bool
	}{
		{
			name:  "goes silent",
			probe: func(_ context.Context, n int) bool { return n == 1 },
			remote: func(ctx context.Context, _ io.Writer) error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			},
			rebuilt: true,
		},
		{
			name:  "silent then back",
			probe: func(_ context.Context, n int) bool { return n == 1 || n > 4 },
			remote: func(ctx context.Context, _ io.Writer) error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			},
			rebuilt: true,
		},
		{
			name:  "probe hangs",
			probe: func(_ context.Context, n int) bool { return n == 1 },
			remote: func(ctx context.Context, _ io.Writer) error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			},
			rebuilt: true,
		},
		{
			name:  "pod gone",
			probe: func(_ context.Context, n int) bool { return n == 1 },
			remote: func(context.Context, io.Writer) error {
				return fmt.Errorf("exit status 1")
			},
			rebuilt: true,
		},
		{
			name: "connection dropped",
			remote: func(_ context.Context, w io.Writer) error {
				fmt.Fprintf(w, "error: failed to receive status: rpc error: code = Unavailable desc = error reading from server: EOF\n")
				return fmt.Errorf("exit status 1")
			},
			rebuilt: true,
		},
		{
			name: "build fails",
			remote: func(_ context.Context, w io.Writer) error {
				fmt.Fprintf(w, "error: failed to solve: process \"/bin/sh -c false\" did not complete successfully: exit code: 1\n")
				return fmt.Errorf("exit status 1")
			},
			err: "exit status 1",
		},
		{
			name: "no match",
			remote: func(_ context.Context, w io.Writer) error {
				fmt.Fprintf(w, "error: failed to solve: alpine: no match for platform in manifest: not found\n")
				return fmt.Errorf("exit status 1")
			},
			err: "an image in this build is not published for the requested build architectures (amd64,arm64): set the BuildArch app parameter to an architecture the image supports (convox apps params set BuildArch=amd64 -a app1) or use multi-arch images",
		},
		{
			name: "primary fails",
			remote: func(ctx context.Context, _ io.Writer) error {
				<-ctx.Done()
				return fmt.Errorf("signal: killed")
			},
			primary: fmt.Errorf("exit status 2"),
			err:     "exit status 2",
		},
		{
			name:    "primary fails during the wait",
			probe:   func(context.Context, int) bool { return false },
			primary: fmt.Errorf("exit status 2"),
			err:     "exit status 2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			helperEnv(t)

			c := &commands{probe: test.probe}
			c.build = func(ctx context.Context, w io.Writer, remote bool, arch string) error {
				switch {
				case remote:
					return test.remote(ctx, w)
				case arch == "amd64":
					return test.primary
				}
				return nil
			}
			if test.name == "probe hangs" {
				c.probe = func(ctx context.Context, n int) bool {
					if n > 1 {
						<-ctx.Done()
					}
					return n == 1
				}
			}
			build.StubCommands(t, c.run)
			if test.name == "primary fails during the wait" {
				build.SetHelperReadyTimeout(time.Minute)
			}

			testBuild(t, nativeSource(t, oneService, map[string]string{"Dockerfile": "FROM alpine\n"}), bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
				nativeProvider(p, test.err == "")
				if test.err == "" {
					e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)
				}

				done := make(chan error, 1)
				go func() { done <- b.Execute() }()

				var err error
				select {
				case err = <-done:
				case <-time.After(10 * time.Second):
					t.Fatal("build did not return")
				}

				local, _ := c.invocations()
				_, rebuilt := local["arm64"]
				require.Equal(t, test.rebuilt, rebuilt)

				if test.err != "" {
					require.EqualError(t, err, test.err)
					require.NotContains(t, out.String(), "WARNING")
					require.NotContains(t, local, "amd64,arm64")
					return
				}

				require.NoError(t, err)
				require.Equal(t, 1, strings.Count(out.String(), "WARNING: no arm64 build node is reachable"))
				require.Contains(t, local, "amd64,arm64")
			})
		})
	}
}

func TestBuildNativeKeepsSingleInvocation(t *testing.T) {
	multi := []interface{}{"--opt", "platform=linux/amd64,linux/arm64"}

	tests := []struct {
		name        string
		manifest    string
		dockerfile  string
		development bool
		env         map[string]string
		platform    []interface{}
		tag         string
	}{
		{name: "image service", manifest: "services:\n  web:\n    image: httpd\n", platform: multi},
		{name: "development", manifest: oneService, dockerfile: "FROM alpine\n", development: true},
		{name: "onbuild", manifest: oneService, dockerfile: "FROM alpine\nonbuild RUN true\n", platform: multi},
		{name: "one arch", manifest: oneService, dockerfile: "FROM alpine\n", env: map[string]string{"BUILD_ARCHS": "arm64"}, platform: []interface{}{"--opt", "platform=linux/arm64"}},
		{name: "bad address", manifest: oneService, dockerfile: "FROM alpine\n", env: map[string]string{"BUILD_HELPER_ADDR": "build-helper"}, platform: multi},
		{name: "bad key", manifest: oneService, dockerfile: "FROM alpine\n", env: map[string]string{"BUILD_HELPER_KEY": "MHcCAQ"}, platform: multi},
		{name: "no ca", manifest: oneService, dockerfile: "FROM alpine\n", env: map[string]string{"BUILD_HELPER_CA": ""}, platform: multi},
		{name: "helper arch not built", manifest: oneService, dockerfile: "FROM alpine\n", env: map[string]string{"BUILD_HELPER_ARCH": "riscv64"}, platform: multi},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			helperEnv(t)
			for k, v := range test.env {
				t.Setenv(k, v)
			}

			build.StubCommands(t, func(context.Context, io.Writer, string, ...string) error {
				t.Error("per-architecture path used")
				return nil
			})

			opts := nativeSource(t, test.manifest, map[string]string{"Dockerfile": test.dockerfile})
			opts.Development = test.development

			testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
				nativeProvider(p, true)

				args := []interface{}{
					mock.Anything,
					"buildctl", "build", "--frontend", "dockerfile.v0", "--local", mock.MatchedBy(matchContext), "--local", mock.MatchedBy(matchDockerfile),
					"--opt", mock.MatchedBy(matchFilename), "--output", "type=image,name=registry.test.com:web.build1,push=true",
				}
				args = append(args, test.platform...)
				args = append(args,
					"--export-cache", "mode=max,image-manifest=true,oci-mediatypes=true,ignore-error=true,compression=estargz,type=registry,ref=registry.test.com:web.buildcache",
					"--import-cache", "type=registry,ignore-error=true,ref=registry.test.com:web.buildcache",
				)
				e.On("Run", args...).Return(nil).Once()
				e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)

				require.NoError(t, b.Execute())
			})
		})
	}
}

func TestBuildNativeHelperLostStaysLost(t *testing.T) {
	helperEnv(t)

	c := &commands{probe: func(_ context.Context, n int) bool { return n == 1 }}
	c.build = func(_ context.Context, _ io.Writer, remote bool, _ string) error {
		if remote {
			return fmt.Errorf("exit status 1")
		}
		return nil
	}
	build.StubCommands(t, c.run)

	manifest := "services:\n  web:\n    build: .\n  worker:\n    build: ./worker\n"
	opts := nativeSource(t, manifest, map[string]string{"Dockerfile": "FROM alpine\n", "worker/Dockerfile": "FROM alpine\n"})

	testBuild(t, opts, bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:worker.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		c.mu.Lock()
		defer c.mu.Unlock()

		remote := 0
		for _, args := range c.builds {
			if args[0] == "--addr" {
				remote++
			}
		}
		require.Equal(t, 1, remote)
		require.Equal(t, 2, c.probes)
		require.Equal(t, 2, strings.Count(out.String(), "WARNING: no arm64 build node is reachable"))
	})
}

func TestBuildNativeAmd64Helper(t *testing.T) {
	helperEnv(t)
	t.Setenv("BUILD_HELPER_ARCH", "amd64")

	c := &commands{}
	build.StubCommands(t, c.run)

	testBuild(t, nativeSource(t, oneService, map[string]string{"Dockerfile": "FROM alpine\n"}), bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		local, remote := c.invocations()
		require.Contains(t, local, "arm64")
		require.Contains(t, local, "amd64,arm64")
		require.NotContains(t, local, "amd64")
		require.Contains(t, remote, "amd64")
		require.Len(t, remote, 1)
		require.Contains(t, out.String(), "[arm64] #")
		require.Contains(t, out.String(), "[amd64] #")
	})
}

func TestBuildNativeProbeFlaps(t *testing.T) {
	helperEnv(t)

	c := &commands{probe: func(_ context.Context, n int) bool { return n%2 == 1 }}
	c.build = func(ctx context.Context, _ io.Writer, remote bool, _ string) error {
		for remote {
			c.mu.Lock()
			n := c.probes
			c.mu.Unlock()
			if n >= 9 {
				break
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("signal: killed")
			case <-time.After(time.Millisecond):
			}
		}
		return nil
	}
	build.StubCommands(t, c.run)

	testBuild(t, nativeSource(t, oneService, map[string]string{"Dockerfile": "FROM alpine\n"}), bkEngine, func(b *build.Build, p *structs.MockProvider, e *exec.MockInterface, out *bytes.Buffer) {
		nativeProvider(p, true)
		e.On("Execute", "skopeo", "inspect", "--config", "docker://registry.test.com:web.build1").Return(fxSkopeoInspect(), nil)

		require.NoError(t, b.Execute())

		local, _ := c.invocations()
		require.NotContains(t, local, "arm64")
		require.NotContains(t, out.String(), "WARNING")
	})
}
