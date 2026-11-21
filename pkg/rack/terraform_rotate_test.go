package rack

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/convox/stdcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const staleNotice = "NOTICE: removing parameters not supported by version 3.26.1: stale\n"

const rotateNotice = "NOTICE: rotated the rack API password. RACK_URL values that embed the old password no longer work; run 'convox rack access key rotate' to also revoke access tokens.\n"

var (
	initArgs  = []string{"init", "-force-copy", "-no-color", "-upgrade"}
	taintArgs = []string{"taint", "-no-color", "-allow-missing", "module.system.module.rack.module.api.module.k8s.random_string.password"}
	applyArgs = []string{"apply", "-auto-approve", "-no-color"}
)

type recordedTerraform struct {
	calls  [][]string
	failOn string
}

func (*recordedTerraform) Execute(string, ...string) ([]byte, error) { return nil, nil }

func (*recordedTerraform) Run(io.Writer, string, ...string) error { return nil }

func (r *recordedTerraform) Terminal(_ string, args ...string) error {
	r.calls = append(r.calls, args)
	fmt.Fprintf(os.Stderr, "terraform %s\n", args[0])
	if args[0] == r.failOn {
		return fmt.Errorf("%s failed", args[0])
	}
	return nil
}

func runWithTerraform(t *testing.T, root string, tf *recordedTerraform, fn func(c *stdcli.Context) error) (string, error) {
	t.Helper()

	e := stdcli.New("convox", "test")
	e.Settings = root
	e.Executor = tf
	stubTerraform(t, func(_, _ io.Writer, args ...string) error { return tf.Terminal("terraform", args...) })

	var ferr error
	e.Command("rt", "rotate-test handler", func(c *stdcli.Context) error {
		ferr = fn(c)
		return nil
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

	e.Execute([]string{"rt"})
	os.Stderr = origStderr
	require.NoError(t, w.Close())

	return <-done, ferr
}

func TestUpdateParamsRotateAPIPassword(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]string
		failOn string
		calls  [][]string
		stderr string
		err    string
	}{
		{
			name:   "rotate",
			params: map[string]string{"rotate_api_password": "true", "telemetry": "false"},
			calls:  [][]string{initArgs, taintArgs, applyArgs},
			stderr: "terraform init\n" + staleNotice + "terraform taint\nterraform apply\n" + rotateNotice,
		},
		{
			name:   "no key",
			params: map[string]string{"telemetry": "false"},
			calls:  [][]string{initArgs, applyArgs},
			stderr: "terraform init\n" + staleNotice + "terraform apply\n",
		},
		{
			name:   "key not true",
			params: map[string]string{"rotate_api_password": "false", "telemetry": "false"},
			calls:  [][]string{initArgs, applyArgs},
			stderr: "terraform init\n" + staleNotice + "terraform apply\n",
		},
		{
			name:   "taint fails",
			params: map[string]string{"rotate_api_password": "true", "telemetry": "false"},
			failOn: "taint",
			calls:  [][]string{initArgs, taintArgs},
			stderr: "terraform init\n" + staleNotice + "terraform taint\n",
			err:    "taint failed",
		},
		{
			name:   "apply fails",
			params: map[string]string{"rotate_api_password": "true", "telemetry": "false"},
			failOn: "apply",
			calls:  [][]string{initArgs, taintArgs, applyArgs},
			stderr: "terraform init\n" + staleNotice + "terraform taint\nterraform apply\n",
			err:    "apply failed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			f := &reconcileFixture{
				rackName:   "myrack",
				moduleVars: []string{"name", "release", "telemetry"},
				vars:       map[string]string{"release": "3.26.1", "stale": "x"},
			}
			f.setup(t, root)

			tf := &recordedTerraform{failOn: tc.failOn}
			stderr, err := runWithTerraform(t, root, tf, func(c *stdcli.Context) error {
				return Terraform{ctx: c, name: "myrack", provider: "azure"}.UpdateParams(tc.params)
			})

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tc.calls, tf.calls)
			assert.Equal(t, tc.stderr, stderr)

			vars := f.readVars(t)
			assert.NotContains(t, vars, "rotate_api_password")
			assert.Equal(t, "false", vars["telemetry"])

			main, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
			require.NoError(t, err)
			assert.NotContains(t, string(main), "rotate_api_password")
		})
	}
}

func TestInstallTerraformDropsRotateAPIPassword(t *testing.T) {
	root := t.TempDir()

	tf := &recordedTerraform{}
	stderr, err := runWithTerraform(t, root, tf, func(c *stdcli.Context) error {
		return InstallTerraform(c, "local", "myrack", "3.26.1", map[string]string{"rotate_api_password": "true"})
	})

	require.NoError(t, err)
	assert.Equal(t, [][]string{initArgs, applyArgs}, tf.calls)
	assert.Equal(t, "terraform init\nterraform apply\n", stderr)

	for _, name := range []string{"vars.json", "main.tf"} {
		data, err := os.ReadFile(filepath.Join(root, "racks", "myrack", name))
		require.NoError(t, err)
		assert.NotContains(t, string(data), "rotate_api_password", name)
	}
}
