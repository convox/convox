//go:build !windows

package rack

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeCommand(t *testing.T, name, script string) {
	t.Helper()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0700)) //nolint:gosec // the script has to be executable
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeTerraform(t *testing.T, script string) {
	t.Helper()

	fakeCommand(t, "terraform", script)
}

func TestRunTerraformKeepsStreamsApart(t *testing.T) {
	fakeTerraform(t, "echo \"out $1 $2\"\necho err-line >&2\nexit 3\n")

	var stdout, stderr bytes.Buffer

	start := time.Now()
	err := runTerraform(&stdout, &stderr, "apply", "-no-color")

	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.ExitCode())
	assert.Equal(t, "out apply -no-color\n", stdout.String())
	assert.Equal(t, "err-line\n", stderr.String())
	assert.Less(t, time.Since(start), terraformDrainIdle/2)
}

type startedWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	started chan struct{}
	once    sync.Once
}

func (w *startedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)
	if strings.Contains(w.buf.String(), "started") {
		w.once.Do(func() { close(w.started) })
	}

	return len(p), nil
}

func (w *startedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

func TestRunTerraformSurvivesSigterm(t *testing.T) {
	gate := filepath.Join(t.TempDir(), "release")
	t.Setenv("TERRAFORM_TEST_GATE", gate)

	fakeTerraform(t, "trap 'echo got-term' TERM\necho started\nwhile [ ! -e \"$TERRAFORM_TEST_GATE\" ]; do sleep 0.05; done\necho finished\nexit 0\n")

	// keeps the test binary alive if runTerraform does not handle the signal
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	stdout := &startedWriter{started: make(chan struct{})}

	done := make(chan error, 1)
	go func() { done <- runTerraform(stdout, io.Discard, "apply") }()

	select {
	case <-stdout.started:
	case <-time.After(30 * time.Second):
		t.Fatal("fake terraform did not start")
	}

	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGTERM))

	select {
	case <-guard:
	case <-time.After(30 * time.Second):
		t.Fatal("SIGTERM was not delivered")
	}

	require.NoError(t, os.WriteFile(gate, nil, 0600))

	select {
	case err := <-done:
		assert.ErrorIs(t, err, errTerraformInterrupted)
	case <-time.After(30 * time.Second):
		t.Fatal("runTerraform did not return")
	}

	assert.Contains(t, stdout.String(), "finished")
	assert.NotContains(t, stdout.String(), "got-term")
}

func shortDrainIdle(t *testing.T, idle time.Duration) {
	t.Helper()

	orig := terraformDrainIdle
	terraformDrainIdle = idle
	t.Cleanup(func() { terraformDrainIdle = orig })
}

func TestRunTerraformDoesNotWaitForInheritedPipe(t *testing.T) {
	shortDrainIdle(t, 200*time.Millisecond)

	fakeTerraform(t, "sleep 5 &\necho done\nexit 3\n")

	var stdout bytes.Buffer

	start := time.Now()
	err := runTerraform(&stdout, io.Discard, "apply")

	var exit *exec.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 3, exit.ExitCode())
	assert.Less(t, time.Since(start), 3*time.Second)
	assert.Equal(t, "done\n", stdout.String())
}

type pausingWriter struct {
	pause time.Duration
	once  sync.Once
	buf   bytes.Buffer
}

func (w *pausingWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { time.Sleep(w.pause) })

	return w.buf.Write(p)
}

func TestRunTerraformKeepsOutputForPausedConsumer(t *testing.T) {
	shortDrainIdle(t, 100*time.Millisecond)

	fakeTerraform(t, "i=0\nwhile [ $i -lt 600 ]; do echo \"module.system.null_resource.r[$i]: Creation complete after 0s\"; i=$((i+1)); done\necho 'Apply complete!'\n")

	stdout := &pausingWriter{pause: time.Second}

	require.NoError(t, runTerraform(stdout, io.Discard, "apply"))

	assert.Equal(t, 600, strings.Count(stdout.buf.String(), "Creation complete"))
	assert.True(t, strings.HasSuffix(stdout.buf.String(), "Apply complete!\n"))
}

func TestRunTerraformOutlivesWriterPanic(t *testing.T) {
	fakeTerraform(t, "echo one\nsleep 0.2\necho two\nsleep 0.2\necho three\nexit 0\n")

	broken := &panicOnceWriter{}
	stdout := &redactWriter{red: &redactor{}, w: broken}

	require.NoError(t, runTerraform(stdout, io.Discard, "apply"))
	stdout.Flush()

	assert.Contains(t, broken.String(), "three\n")
}

func TestApplyHelmCleanupPublicRackGivesUpWhenAwsIsSlow(t *testing.T) {
	shortHelmClear(t, 20*time.Millisecond, 300*time.Millisecond)

	fakeCommand(t, "aws", "exec sleep 1\n")

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{"region": "us-east-1"})

	stubTerraform(t, func(_, w io.Writer, _ ...string) error {
		fmt.Fprint(w, helmErrorTerraform15)
		return errApplyFailed
	})

	var err error
	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		err = tf.apply()
	})

	assert.Equal(t, errApplyFailed, err)
	assert.Equal(t, helmErrorTerraform15+helmGaveUp, out)
}

func TestTerraformCaptureIgnoresInterrupt(t *testing.T) {
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, os.Interrupt)
	defer signal.Stop(guard)

	stubTerraform(t, func(_, _ io.Writer, _ ...string) error {
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGINT))
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	require.NoError(t, terraformCapture(t.TempDir(), nil, "apply"))

	select {
	case <-guard:
		t.Fatal("interrupt was delivered while terraform ran")
	default:
	}
}
