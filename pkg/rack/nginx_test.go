package rack

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/convox/stdcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type terraformStub struct{}

func (terraformStub) Execute(string, ...string) ([]byte, error) { return nil, nil }

func (terraformStub) Run(io.Writer, string, ...string) error { return nil }

func (terraformStub) Terminal(cmd string, args ...string) error {
	fmt.Fprintf(os.Stderr, "%s %s\n", cmd, args[0])
	return nil
}

func staleNginxMessage(version string) string {
	return "WARNING: nginx_image pins ingress-nginx controller " + version + ", which is older than the patched v1.13.9, v1.14.5 and v1.15.1 releases. Rack updates keep this image until nginx_image is changed or cleared with 'convox rack params set nginx_image= --force'.\n"
}

func TestStaleNginxImageWarning(t *testing.T) {
	for _, tc := range []struct {
		image   string
		version string
	}{
		{image: ""},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.15.1@sha256:594ceea76b01c592858f803f9ff4d2cb40542cae2060410b2c95f75907d659e1"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.12.0@sha256:e6b8de175acda6ca913891f0f727bca4527e797d52688cbe9fec9040d6f6b6fa", version: "v1.12.0"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.12.6@sha256:c371fbf42b4f23584ce879d99303463131f4f31612f0875482b983354eeca7e6", version: "v1.12.6"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.13.8", version: "v1.13.8"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.13.9"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.14.4", version: "v1.14.4"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.14.5"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.15.0", version: "v1.15.0"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.16.0"},
		{image: "registry.k8s.io/ingress-nginx/controller:v1.12.0-beta.0", version: "v1.12.0"},
		{image: "k8s.gcr.io/ingress-nginx/controller:v0.49.3", version: "v0.49.3"},
		{image: "123456789012.dkr.ecr.us-east-1.amazonaws.com/ingress-nginx/controller:v1.11.3", version: "v1.11.3"},
		{image: "registry.example.com:5000/ingress-nginx/controller:v1.12.6", version: "v1.12.6"},
		{image: "  registry.k8s.io/ingress-nginx/controller:v1.12.6", version: "v1.12.6"},
		{image: "registry.k8s.io/ingress-nginx/controller@sha256:594ceea76b01c592858f803f9ff4d2cb40542cae2060410b2c95f75907d659e1"},
		{image: "convox/ingress-nginx:1.15.1-1"},
		{image: "nginx/nginx-ingress:3.0.0"},
	} {
		t.Run(tc.image, func(t *testing.T) {
			want := ""
			if tc.version != "" {
				want = staleNginxMessage(tc.version)
			}
			assert.Equal(t, want, staleNginxImageWarning(tc.image))
		})
	}
}

func TestNginxImageWarningReadsStoredParam(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
		want string
	}{
		{name: "parameter absent", vars: map[string]string{"region": "us-east-1"}},
		{name: "current pin", vars: map[string]string{"nginx_image": "registry.k8s.io/ingress-nginx/controller:v1.15.1"}},
		{name: "stale pin", vars: map[string]string{"nginx_image": "registry.k8s.io/ingress-nginx/controller:v1.12.6"}, want: staleNginxMessage("v1.12.6")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			f := &reconcileFixture{rackName: "myrack", vars: tc.vars}
			f.setup(t, root)

			var got string
			withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
				got = tf.nginxImageWarning()
			})

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestApplyPrintsNginxWarningAfterTerraform(t *testing.T) {
	root := t.TempDir()
	f := &reconcileFixture{rackName: "myrack", vars: map[string]string{"nginx_image": "registry.k8s.io/ingress-nginx/controller:v1.12.6"}}
	f.setup(t, root)

	e := stdcli.New("convox", "test")
	e.Settings = root
	e.Executor = terraformStub{}
	e.Command("rt", "apply-test handler", func(c *stdcli.Context) error {
		return Terraform{ctx: c, name: "myrack", provider: "azure"}.apply()
	}, stdcli.CommandOptions{})

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()

	code := e.Execute([]string{"rt"})
	require.NoError(t, w.Close())
	stderr := <-done
	os.Stderr = origStderr

	assert.Equal(t, 0, code)
	assert.Equal(t, "terraform apply\n"+staleNginxMessage("v1.12.6"), stderr)
}
