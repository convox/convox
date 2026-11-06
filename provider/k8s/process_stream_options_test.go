package k8s

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
)

func TestExecStreamOptionsTty(t *testing.T) {
	for _, c := range []struct {
		name  string
		stdin bool
		tty   bool
	}{
		{"interactive", true, true},
		{"piped", true, false},
		{"no stdin no tty", false, false},
		{"no stdin with tty", false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			rw := &bytes.Buffer{}

			sopts := execStreamOptions(
				&ac.PodExecOptions{Stdin: c.stdin, TTY: c.tty},
				rw,
				structs.ProcessExecOptions{Height: options.Int(24), Width: options.Int(80)},
			)

			require.Equal(t, c.tty, sopts.Tty)
			require.Equal(t, c.stdin, sopts.Stdin != nil)
			require.Same(t, rw, sopts.Stdout)
			require.Same(t, rw, sopts.Stderr)
			require.NotNil(t, sopts.TerminalSizeQueue)
		})
	}
}

func TestExecStreamOptionsTerminalSize(t *testing.T) {
	eo := &ac.PodExecOptions{Stdin: true, TTY: true}

	sopts := execStreamOptions(eo, &bytes.Buffer{}, structs.ProcessExecOptions{Height: options.Int(24), Width: options.Int(80)})
	require.NotNil(t, sopts.TerminalSizeQueue)

	sopts = execStreamOptions(eo, &bytes.Buffer{}, structs.ProcessExecOptions{Height: options.Int(24)})
	require.Nil(t, sopts.TerminalSizeQueue)

	sopts = execStreamOptions(eo, &bytes.Buffer{}, structs.ProcessExecOptions{})
	require.Nil(t, sopts.TerminalSizeQueue)
}

// The stdin fix and the stderr fix landed as separate PRs that both rewrote this
// construction; a merge that keeps only the stderr side compiles, passes both
// PRs' own tests, and hangs here.
func TestExecStreamOptionsStdinEndsOnCleanEnd(t *testing.T) {
	sopts := execStreamOptions(&ac.PodExecOptions{Stdin: true}, &bytes.Buffer{}, structs.ProcessExecOptions{})

	done := make(chan struct{})

	go func() {
		_, _ = io.ReadAll(sopts.Stdin)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "exec stdin never reached end of input")
	}
}
