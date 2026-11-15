package rack

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/convox/stdcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryEnvRecorder struct {
	retryMax string
}

func (r *retryEnvRecorder) Execute(string, ...string) ([]byte, error) {
	r.retryMax = os.Getenv("TF_HTTP_RETRY_MAX")

	return []byte("{}"), nil
}

func (*retryEnvRecorder) Run(io.Writer, string, ...string) error { return nil }

func (*retryEnvRecorder) Terminal(string, ...string) error { return nil }

func TestTerraformBackendRetryEnv(t *testing.T) {
	t.Setenv("TF_HTTP_RETRY_MAX", "")
	t.Setenv("TF_HTTP_RETRY_WAIT_MIN", "")
	t.Setenv("TF_HTTP_RETRY_WAIT_MAX", "")

	terraformBackendRetryEnv()

	assert.Equal(t, "10", os.Getenv("TF_HTTP_RETRY_MAX"))
	assert.Equal(t, "2", os.Getenv("TF_HTTP_RETRY_WAIT_MIN"))
	assert.Equal(t, "15", os.Getenv("TF_HTTP_RETRY_WAIT_MAX"))

	t.Setenv("TF_HTTP_RETRY_MAX", "3")
	t.Setenv("TF_HTTP_RETRY_WAIT_MIN", "")

	terraformBackendRetryEnv()

	assert.Equal(t, "3", os.Getenv("TF_HTTP_RETRY_MAX"))
	assert.Equal(t, "2", os.Getenv("TF_HTTP_RETRY_WAIT_MIN"))
	assert.Equal(t, "15", os.Getenv("TF_HTTP_RETRY_WAIT_MAX"))
}

func TestLoadTerraformSetsBackendRetryEnv(t *testing.T) {
	t.Setenv("TF_HTTP_RETRY_MAX", "")
	t.Setenv("TF_HTTP_RETRY_WAIT_MIN", "")
	t.Setenv("TF_HTTP_RETRY_WAIT_MAX", "")

	root := t.TempDir()
	dir := filepath.Join(root, "racks", "myrack")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), nil, 0600))

	rec := &retryEnvRecorder{}

	e := stdcli.New("convox", "test")
	e.Settings = root
	e.Executor = rec
	e.Command("load", "load a terraform rack", func(c *stdcli.Context) error {
		_, err := LoadTerraform(c, "myrack")
		return err
	}, stdcli.CommandOptions{})

	require.Equal(t, 0, e.Execute([]string{"load"}))
	assert.Equal(t, "10", rec.retryMax)
}
