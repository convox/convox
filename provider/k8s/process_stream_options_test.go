package k8s

import (
	"bytes"
	"testing"

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
