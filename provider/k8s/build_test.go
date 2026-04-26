package k8s_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildList(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    structs.Builds
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    structs.Builds{structs.Build{Id: "BUILD1", App: "app1", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)}},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    structs.Builds(structs.Builds{}),
			Err:         errors.New("app not found: app2-not-found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Once()
				}

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				bs, err := p.BuildList(test.AppNameList, structs.BuildListOptions{})

				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, bs, test.Response)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildGet(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "app1", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("builds.convox.com \"build2\" not found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				bs, err := p.BuildGet(test.AppName, test.BuildName)
				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, bs, test.Response)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildUpdate(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("builds.convox.com \"build2\" not found"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				err := buildCreate(p.Convox, test.Namespace, test.BuildName, "basic")
				require.NoError(t, err)

				status := "Running"
				release := "v1"

				_, err = p.BuildUpdate(test.AppName, test.BuildName, structs.BuildUpdateOptions{
					Status:  &status,
					Release: &release,
				})

				if err == nil {
					require.NoError(t, err)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestBuildCreate(t *testing.T) {
	tests := []struct {
		Name        string
		RackName    string
		AppName     string
		AppNameList string
		Namespace   string
		BuildName   string
		Response    *structs.Build
		Err         error
	}{
		{
			Name:        "Success",
			RackName:    "rack1",
			AppName:     "app1",
			AppNameList: "app1",
			Namespace:   "rack1-app1",
			BuildName:   "build1",
			Response:    &structs.Build{Id: "BUILD1", App: "", Description: "foo", Entrypoint: "", Logs: "", Manifest: "services:\n  web:\n    build: .\n    port: 5000\n", Process: "", Release: "", Reason: "", Repository: "", Status: "", Started: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Ended: time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC), Tags: map[string]string(nil)},
			Err:         nil,
		},
		{
			Name:        "app not found",
			RackName:    "rack2",
			AppName:     "app2",
			AppNameList: "app2-not-found",
			Namespace:   "rack2-app2",
			BuildName:   "build2",
			Response:    nil,
			Err:         errors.New("app not found: app2"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Creating", "R1234567", nil).Times(3)
				}

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				_, err := p.BuildCreate(test.AppName, test.BuildName, structs.BuildCreateOptions{})
				if err == nil {
					require.NoError(t, err)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func buildCreate(kc cv.Interface, ns, id, fixture string) error {
	spec, err := buildFixture(fixture)
	if err != nil {
		return errors.WithStack(err)
	}

	app := strings.Split(ns, "-")

	b := &ca.Build{
		ObjectMeta: am.ObjectMeta{
			Name: id,
			Labels: map[string]string{
				"app": app[1],
			},
		},
		Spec: *spec,
	}

	if _, err := kc.ConvoxV1().Builds(ns).Create(b); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func buildFixture(name string) (*ca.BuildSpec, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/build-%s.yml", name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var fixture struct {
		Description string
		Ended       string
		Manifest    interface{}
		Started     string
	}

	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, errors.WithStack(err)
	}

	mdata, err := yaml.Marshal(fixture.Manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := &ca.BuildSpec{
		Description: fixture.Description,
		Ended:       fixture.Ended,
		Manifest:    string(mdata),
		Started:     fixture.Started,
	}

	return s, nil
}

func waitForBuildStatus(t *testing.T, p *k8s.Provider, app, id string, want string) *structs.Build {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		b, err := p.BuildGet(app, id)
		if err == nil && b.Status == want {
			return b
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, err := p.BuildGet(app, id)
	require.NoError(t, err)
	t.Fatalf("build %s never reached status %q (last status: %q, reason: %q)", id, want, b.Status, b.Reason)
	return nil
}

// readAuthfile reads the file path stored after the `--authfile` flag in a
// captured skopeo args slice and returns its contents. Returns "" if the flag
// isn't present.
func readAuthfileFromArgs(t *testing.T, args []string) string {
	t.Helper()
	for i, a := range args {
		if a == "--authfile" && i+1 < len(args) {
			data, err := os.ReadFile(args[i+1])
			require.NoError(t, err)
			return string(data)
		}
	}
	return ""
}

// assertArgsShape checks the canonical skopeo args order: copy [...flags] -- docker://<src> docker://<dst>.
// Returns the (src, dst) positional pair.
func assertArgsShape(t *testing.T, args []string) (string, string) {
	t.Helper()
	require.NotEmpty(t, args)
	require.Equal(t, "copy", args[0])
	require.GreaterOrEqual(t, len(args), 4, "args must have at least: copy -- src dst")
	// positional args are the last two; the token right before them is `--`
	require.Equal(t, "--", args[len(args)-3])
	src := args[len(args)-2]
	dst := args[len(args)-1]
	require.Truef(t, strings.HasPrefix(src, "docker://"), "src missing docker:// prefix: %q", src)
	require.Truef(t, strings.HasPrefix(dst, "docker://"), "dst missing docker:// prefix: %q", dst)
	return src, dst
}

func TestBuildImportImage(t *testing.T) {
	origSkopeo := *k8s.SkopeoExecForTest
	defer func() { *k8s.SkopeoExecForTest = origSkopeo }()

	t.Run("Success", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "build1", "basic"))

			var capturedArgs []string
			var authContents string
			var ctxNonNil bool
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				capturedArgs = append([]string(nil), args...)
				authContents = readAuthfileFromArgs(t, args)
				ctxNonNil = ctx != nil
				return []byte("ok"), nil
			}

			err := p.BuildImportImage("app1", "build1", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "build1", "complete")
			assert.Equal(t, "", b.Reason)
			assert.True(t, ctxNonNil, "skopeoExec must be called with non-nil ctx")

			src, dst := assertArgsShape(t, capturedArgs)
			assert.Equal(t, "docker://vllm/vllm-openai:v0.6.3", src)
			assert.Equal(t, "docker://repo1:web.BUILD1", dst)
			assert.Contains(t, capturedArgs, "--authfile", "unified authfile flag expected")
			assert.NotContains(t, capturedArgs, "--dest-creds", "dest creds must NOT be on argv")
			assert.NotContains(t, capturedArgs, "--src-creds", "src creds must NOT be on argv")
			assert.NotContains(t, capturedArgs, "--src-authfile", "legacy split-authfile flag must not be emitted")
			for _, a := range capturedArgs {
				assert.NotContains(t, a, "un1:pw1", "dest cred colon-pair must not appear on argv")
			}
			// authfile must carry the dest creds (base64 of "un1:pw1") under the rack registry host.
			require.NotEmpty(t, authContents, "authfile must exist during skopeo invocation")
			assert.Contains(t, authContents, "repo1", "authfile must scope to destination host")
			assert.Contains(t, authContents, "dW4xOnB3MQ==", "authfile must carry base64(un1:pw1) for dest")
		})
	})

	t.Run("SrcAndDestCredsInAuthfile", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "build2", "basic"))

			var capturedArgs []string
			var authContentsAtRuntime string
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				capturedArgs = append([]string(nil), args...)
				authContentsAtRuntime = readAuthfileFromArgs(t, args)
				return nil, nil
			}

			user := "$oauthtoken"
			pass := "nvapi-secret"
			err := p.BuildImportImage("app1", "build2", "nvcr.io/nim/meta/llama3-8b:1.0.0", structs.BuildImportImageOptions{
				SrcCredsUser: &user,
				SrcCredsPass: &pass,
			})
			require.NoError(t, err)

			waitForBuildStatus(t, p, "app1", "build2", "complete")
			assert.Contains(t, capturedArgs, "--authfile")
			assert.NotContains(t, capturedArgs, "--src-creds", "src creds must NOT be on argv")
			assert.NotContains(t, capturedArgs, "--dest-creds", "dest creds must NOT be on argv")
			for _, a := range capturedArgs {
				assert.NotContains(t, a, "nvapi-secret", "src cred must not appear anywhere on argv")
				assert.NotContains(t, a, "$oauthtoken:nvapi-secret", "src colon-pair must not be on argv")
				assert.NotContains(t, a, "un1:pw1", "dest cred colon-pair must not be on argv")
			}
			// Authfile contains BOTH source and destination host entries.
			assert.Contains(t, authContentsAtRuntime, "nvcr.io", "authfile must include source registry host")
			assert.Contains(t, authContentsAtRuntime, "repo1", "authfile must include destination registry host")
			assert.Contains(t, authContentsAtRuntime, "JG9hdXRodG9rZW46bnZhcGktc2VjcmV0", "base64 of '$oauthtoken:nvapi-secret'")
			assert.Contains(t, authContentsAtRuntime, "dW4xOnB3MQ==", "base64 of 'un1:pw1' (dest)")
		})
	})

	t.Run("EmptyStringSrcCredsSkipped", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildec", "basic"))

			var capturedArgs []string
			var authContents string
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				capturedArgs = append([]string(nil), args...)
				authContents = readAuthfileFromArgs(t, args)
				return nil, nil
			}

			empty := ""
			err := p.BuildImportImage("app1", "buildec", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{
				SrcCredsUser: &empty,
				SrcCredsPass: &empty,
			})
			require.NoError(t, err)

			waitForBuildStatus(t, p, "app1", "buildec", "complete")
			// Dest creds still present (required); source entry must NOT be written.
			assert.Contains(t, capturedArgs, "--authfile")
			assert.NotContains(t, authContents, "docker.io", "empty src creds must not produce a Docker Hub entry")
			assert.Contains(t, authContents, "repo1", "dest entry must still be present")
		})
	})

	t.Run("SkopeoFailureScrubbed", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "build3", "basic"))

			// stderr includes a Bearer token and an inline cred URL; both must be scrubbed.
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("auth failed: Bearer abcdef1234567890TOKEN at https://user:sup3rs3cret@host/x"), fmt.Errorf("exit status 1")
			}

			err := p.BuildImportImage("app1", "build3", "nvcr.io/private/gated:1.0", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "build3", "failed")
			assert.Contains(t, b.Reason, "image relay failed for service web")
			assert.NotContains(t, b.Reason, "sup3rs3cret", "inline password must be scrubbed")
			assert.NotContains(t, b.Reason, "abcdef1234567890TOKEN", "bearer token must be scrubbed")
			assert.LessOrEqual(t, len(b.Reason), 500, "Reason must be truncated")
		})
	})

	t.Run("ScrubEmptyUsernameToken", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildeu", "basic"))

			// URL with empty username and a bare bearer-style token after the colon.
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("auth: fetch https://:eyJhbGciOiJIUzI1NiJ9-veryLongToken@registry.example.com/v2/"), fmt.Errorf("exit status 1")
			}

			err := p.BuildImportImage("app1", "buildeu", "registry.example.com/gated:1.0", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "buildeu", "failed")
			assert.NotContains(t, b.Reason, "eyJhbGciOiJIUzI1NiJ9-veryLongToken", "empty-username token must still be scrubbed")
			assert.Contains(t, b.Reason, "[REDACTED]@", "redaction marker must be present")
		})
	})

	t.Run("ScrubPreservesBenignColons", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildsb", "basic"))

			// error text with benign `host:port` and no `://` prefix — must NOT be scrubbed.
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("dial tcp registry.internal:5000@pod-abc: connection refused"), fmt.Errorf("exit status 1")
			}

			err := p.BuildImportImage("app1", "buildsb", "nvcr.io/private/x:1.0", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "buildsb", "failed")
			assert.Contains(t, b.Reason, "registry.internal:5000@pod-abc", "benign host:port@host must pass through unscrubbed")
		})
	})

	t.Run("MultiService", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildm", "multisvc"))

			var calls [][]string
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				calls = append(calls, append([]string(nil), args...))
				return nil, nil
			}

			err := p.BuildImportImage("app1", "buildm", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			waitForBuildStatus(t, p, "app1", "buildm", "complete")
			require.Len(t, calls, 2, "one skopeo call per service")

			dstSeen := map[string]bool{}
			for _, call := range calls {
				_, dst := assertArgsShape(t, call)
				dstSeen[dst] = true
			}
			assert.True(t, dstSeen["docker://repo1:web.BUILDM"])
			assert.True(t, dstSeen["docker://repo1:worker.BUILDM"])
		})
	})

	t.Run("MultiServicePartialFailure", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildpf", "multisvc"))

			var call int
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				call++
				if call == 2 {
					return []byte("upstream 502"), fmt.Errorf("exit status 1")
				}
				return nil, nil
			}

			err := p.BuildImportImage("app1", "buildpf", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "buildpf", "failed")
			// Final Reason should name whichever service was iterated second (map order is not guaranteed).
			assert.Regexp(t, `image relay failed for service (web|worker)`, b.Reason)
			assert.Contains(t, b.Reason, "upstream 502")
		})
	})

	t.Run("SvcImageOverride", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildso", "svcimage"))

			var capturedSrc string
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				src, _ := assertArgsShape(t, args)
				capturedSrc = src
				return nil, nil
			}

			err := p.BuildImportImage("app1", "buildso", "caller-provided/default:v1", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			waitForBuildStatus(t, p, "app1", "buildso", "complete")
			assert.Equal(t, "docker://override.example.com/baked:v1", capturedSrc, "svc.Image must override caller-provided imageRef")
		})
	})

	t.Run("PanicInGoroutineFinalizesFailed", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildp", "basic"))

			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				panic("synthetic panic inside skopeoExec")
			}

			err := p.BuildImportImage("app1", "buildp", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			b := waitForBuildStatus(t, p, "app1", "buildp", "failed")
			assert.Contains(t, b.Reason, "panic during image import")
		})
	})

	t.Run("EmptyServicesRejected", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))

			// Manifest that parses but has no services. `services: {}` is accepted by the loader.
			emptySvc := &ca.Build{
				ObjectMeta: am.ObjectMeta{Name: "buildes", Labels: map[string]string{"app": "app1"}},
				Spec: ca.BuildSpec{
					Manifest: "services: {}\n",
					Started:  "20200101.000000.000000000",
					Ended:    "20200101.000000.000000000",
				},
			}
			_, err := p.Convox.ConvoxV1().Builds("rack1-app1").Create(emptySvc)
			require.NoError(t, err)

			err = p.BuildImportImage("app1", "buildes", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "no services to relay")
		})
	})

	t.Run("RejectsReinvocationWhileRunning", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildrr", "basic"))

			// First call transitions status to running synchronously before the
			// goroutine launches; block the goroutine's skopeo until we've made
			// the second call.
			release := make(chan struct{})
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				<-release
				return nil, nil
			}

			err := p.BuildImportImage("app1", "buildrr", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.NoError(t, err)

			// Second call hits the running precondition and is rejected immediately.
			err = p.BuildImportImage("app1", "buildrr", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "already importing")

			// Let the first run finish so the test doesn't leak a blocked goroutine.
			close(release)
			waitForBuildStatus(t, p, "app1", "buildrr", "complete")
		})
	})

	t.Run("InvalidImageRefRejected", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildiv", "basic"))

			hostile := []string{
				"--help",         // leading flag
				"img;rm -rf /",   // shell metacharacters
				"img with space", // whitespace
				"a//b:1",         // double-slash
				"a@-flag:1",      // @- smuggle
				"a/b:-flag",      // :- smuggle
				"a/-flag/b:1",    // /- smuggle
				"img:",           // trailing colon
				"img@",           // trailing at
				"img/",           // trailing slash
				"img::tag",       // double colon
				"img@@digest",    // double at
				"img:@bar",       // :@ junction
				"img/@bar",       // /@ junction
			}
			for _, bad := range hostile {
				err := p.BuildImportImage("app1", "buildiv", bad, structs.BuildImportImageOptions{})
				require.Errorf(t, err, "expected rejection for %q", bad)
				assert.Containsf(t, err.Error(), "invalid image ref", "wrong error for %q", bad)
			}
		})
	})

	t.Run("MissingImageParam", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "build4", "basic"))

			err := p.BuildImportImage("app1", "build4", "", structs.BuildImportImageOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "image ref required")
		})
	})

	t.Run("MissingManifest", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))

			empty := &ca.Build{
				ObjectMeta: am.ObjectMeta{
					Name:   "build5",
					Labels: map[string]string{"app": "app1"},
				},
				Spec: ca.BuildSpec{
					Started: "20200101.000000.000000000",
					Ended:   "20200101.000000.000000000",
				},
			}
			_, err := p.Convox.ConvoxV1().Builds("rack1-app1").Create(empty)
			require.NoError(t, err)

			err = p.BuildImportImage("app1", "build5", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "has no manifest")
		})
	})

	t.Run("BuildNotFound", func(t *testing.T) {
		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))

			err := p.BuildImportImage("app1", "nonexistent", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.Error(t, err)
		})
	})

	t.Run("ConcurrentImportSlotAcquiredAndReleased", func(t *testing.T) {
		// Cap of 2 — run two sequential imports, then a third, all succeed.
		// Proves slots are released on the happy path so steady-state callers
		// are unaffected by the limiter.
		origCap := *k8s.MaxConcurrentImportsForTest
		*k8s.MaxConcurrentImportsForTest = 2
		k8s.ResetImportSlotsForTest()
		defer func() {
			*k8s.MaxConcurrentImportsForTest = origCap
			k8s.ResetImportSlotsForTest()
		}()

		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildc1", "basic"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildc2", "basic"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildc3", "basic"))

			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("ok"), nil
			}

			require.NoError(t, p.BuildImportImage("app1", "buildc1", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			waitForBuildStatus(t, p, "app1", "buildc1", "complete")

			require.NoError(t, p.BuildImportImage("app1", "buildc2", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			waitForBuildStatus(t, p, "app1", "buildc2", "complete")

			// Third call after both above release proves slots are recycled,
			// not leaked one-shot.
			require.NoError(t, p.BuildImportImage("app1", "buildc3", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			waitForBuildStatus(t, p, "app1", "buildc3", "complete")
		})
	})

	t.Run("ConcurrentImportCap_Returns409", func(t *testing.T) {
		// Cap of 1 — block the first import in skopeo, prove the second is
		// rejected with HTTP 409 and a "concurrent-import cap" message.
		origCap := *k8s.MaxConcurrentImportsForTest
		*k8s.MaxConcurrentImportsForTest = 1
		k8s.ResetImportSlotsForTest()
		defer func() {
			*k8s.MaxConcurrentImportsForTest = origCap
			k8s.ResetImportSlotsForTest()
		}()

		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildcap1", "basic"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildcap2", "basic"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildcap3", "basic"))

			release := make(chan struct{})
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				<-release
				return []byte("ok"), nil
			}

			// Build A: succeeds synchronously (returns nil); goroutine blocks
			// in skopeo, holding the only slot.
			require.NoError(t, p.BuildImportImage("app1", "buildcap1", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			// Wait for build A to enter running state (informer-side visibility).
			waitForBuildStatus(t, p, "app1", "buildcap1", "running")

			// Build B: rack at cap → 409.
			err := p.BuildImportImage("app1", "buildcap2", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "concurrent-import cap")
			assert.Contains(t, err.Error(), "wait and retry")

			// Verify the error wraps structs.HttpError with code 409.
			var httpErr *structs.HttpError
			require.True(t, stderrors.As(err, &httpErr), "error must wrap structs.HttpError")
			assert.Equal(t, http.StatusConflict, httpErr.Code())

			// Release build A; once it completes the slot is freed and a
			// fresh call succeeds, proving release happened.
			close(release)
			waitForBuildStatus(t, p, "app1", "buildcap1", "complete")

			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("ok"), nil
			}
			require.NoError(t, p.BuildImportImage("app1", "buildcap3", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			waitForBuildStatus(t, p, "app1", "buildcap3", "complete")
		})
	})

	t.Run("ConcurrentImportCap_PanicReleasesSlot", func(t *testing.T) {
		// Cap of 1 — first build's skopeo panics; verify the goroutine's
		// recover defer + the outermost releaseImportSlot still free the
		// slot so a second call succeeds.
		origCap := *k8s.MaxConcurrentImportsForTest
		*k8s.MaxConcurrentImportsForTest = 1
		k8s.ResetImportSlotsForTest()
		defer func() {
			*k8s.MaxConcurrentImportsForTest = origCap
			k8s.ResetImportSlotsForTest()
		}()

		testProvider(t, func(p *k8s.Provider) {
			kk, ok := p.Cluster.(*fake.Clientset)
			require.True(t, ok)
			require.NoError(t, appCreate(kk, "rack1", "app1"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildp1", "basic"))
			require.NoError(t, buildCreate(p.Convox, "rack1-app1", "buildp2", "basic"))

			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				panic("synthetic panic to verify slot release")
			}

			// Synchronous return is nil; the panic is in the goroutine.
			require.NoError(t, p.BuildImportImage("app1", "buildp1", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			// Wait for failure path to settle (recover defer flips status to failed).
			waitForBuildStatus(t, p, "app1", "buildp1", "failed")

			// Restore success stub; second call must succeed, proving the
			// panic-recover path released the slot via the outermost defer.
			*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
				return []byte("ok"), nil
			}
			require.NoError(t, p.BuildImportImage("app1", "buildp2", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))
			waitForBuildStatus(t, p, "app1", "buildp2", "complete")
		})
	})
}

// TestBuildImportImage_DefaultCapIsFour locks the production cap at 4. A
// future PR raising the cap must update this test deliberately so the
// change appears in the diff log alongside the source bump.
func TestBuildImportImage_DefaultCapIsFour(t *testing.T) {
	assert.Equal(t, 4, *k8s.MaxConcurrentImportsForTest)
}
