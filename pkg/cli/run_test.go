package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdapi"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("sleep 7200")}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessPending(), nil).Twice()
		i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)
		opts := structs.ProcessExecOptions{Entrypoint: options.Bool(true), Tty: options.Bool(false)}
		i.On("ProcessExec", "app1", "pid1", "bash", mock.Anything, opts).Return(4, nil).Run(func(args mock.Arguments) {
			data, err := io.ReadAll(args.Get(3).(io.Reader)) //nolint:errcheck // mock type assertion
			require.NoError(t, err)
			require.Equal(t, "in", string(data))
			args.Get(3).(io.Writer).Write([]byte("out")) //nolint:errcheck // mock type assertion
		})
		i.On("ProcessStop", "app1", "pid1").Return(nil)

		res, err := testExecute(e, "run web bash -a app1 -t 7200", strings.NewReader("in"))
		require.NoError(t, err)
		require.Equal(t, 4, res.Code)
		res.RequireStderr(t, []string{""})
		require.Equal(t, "out", res.Stdout)
	})
}

func TestRunError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("sleep 7200")}).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "run web bash -a app1 -t 7200", strings.NewReader("in"))
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRunDetached(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("bash")}).Return(fxProcess(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{"  convox logs -a app1", "  convox ps stop pid1 -a app1"})
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
	})
}

func TestRunStreamIncomplete(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("sleep 7200")}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)
		opts := structs.ProcessExecOptions{Entrypoint: options.Bool(true), Tty: options.Bool(false)}
		i.On("ProcessExec", "app1", "pid1", "bash", mock.Anything, opts).Return(0, sdk.ErrExecIncomplete)
		i.On("ProcessStop", "app1", "pid1").Return(nil)

		res, err := testExecute(e, "run web bash -a app1 -t 7200", strings.NewReader("in"))
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "the rack did not report an exit status for this command")
		require.Contains(t, res.Stderr, "This run's process has been stopped.")
		require.Contains(t, res.Stderr, "write its own success marker")
	})
}

func TestRunDetachedRetain(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
			Command: options.String("bash"),
			Retain:  options.Int(120),
		}).Return(fxProcess(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d --retain 120", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{"  convox logs -a app1", "  convox ps stop pid1 -a app1"})
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
	})
}

func TestRunRetainRequiresDetach(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "run web bash -a app1 --retain 120", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: --retain is only valid with --detach"})
		res.RequireStdout(t, []string{""})
		i.AssertNotCalled(t, "ProcessRun", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestRunDetachedWait(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
			Command: options.String("bash"),
			Retain:  options.Int(60),
		}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", options.Int(3)), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 3, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
	})
}

func TestRunDetachedWaitSuccess(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
			Command: options.String("bash"),
			Retain:  options.Int(300),
		}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("complete", options.Int(0)), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w --retain 300", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
	})
}

func TestRunDetachedWaitWithoutExitStatus(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
			Command: options.String("bash"),
			Retain:  options.Int(60),
		}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", nil), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "did not report an exit status")
	})
}

func TestRunDetachedWaitUnconfirmed(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
			Command: options.String("bash"),
			Retain:  options.Int(60),
		}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(nil, fmt.Errorf("err1"))

		pr, pw := io.Pipe()
		t.Cleanup(func() { pw.Close() })
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(pr, nil).Maybe()

		start := time.Now()
		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Less(t, time.Since(start), time.Second)
		require.Equal(t, 1, res.Code)
		require.Contains(t, res.Stderr, "could not confirm the outcome of process pid1")
	})
}

func TestRunDetachedId(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("bash")}).Return(fxProcess(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d --id", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"pid1"})
		res.RequireStderr(t, []string{"Running detached process... OK, pid1", "  convox logs -a app1", "  convox ps stop pid1 -a app1"})
	})
}

func TestRunWaitWithoutDetachIsStillANoOp(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{Command: options.String("sleep 7200")}).Return(fxProcess(), nil)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)
		opts := structs.ProcessExecOptions{Entrypoint: options.Bool(true), Tty: options.Bool(false)}
		i.On("ProcessExec", "app1", "pid1", "bash", mock.Anything, opts).Return(4, nil)
		i.On("ProcessStop", "app1", "pid1").Return(nil)

		res, err := testExecute(e, "run web bash -a app1 -t 7200 -w", strings.NewReader("in"))
		require.NoError(t, err)
		require.Equal(t, 4, res.Code)
	})
}

const deployKeyNotice = "WARNING: process output is not available to this deploy key; assign it a role with read access to the app to see output here\n"

type logsRack struct {
	*mocksdk.Interface
	dials   atomic.Int32
	streams func(n int) (io.ReadCloser, error)
}

func (r *logsRack) ProcessLogs(app, pid string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return r.streams(int(r.dials.Add(1)))
}

type testStream struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func newTestStream(lines ...string) *testStream {
	return &testStream{Reader: strings.NewReader(strings.Join(lines, "")), closed: make(chan struct{})}
}

func (s *testStream) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func logLine(text string) string {
	return "2026-10-11T12:00:00Z service/web/pid1 " + text + "\n"
}

func logStream(lines ...string) io.ReadCloser {
	return newTestStream(lines...)
}

func waitRun(i *mocksdk.Interface) {
	i.On("ProcessRun", "app1", "web", structs.ProcessRunOptions{
		Command: options.String("bash"),
		Retain:  options.Int(60),
	}).Return(fxProcess(), nil)
}

// runningUntil reports running until ch is closed and then for polls more reads, then exited with code.
func runningUntil(ch chan struct{}, polls, code int) func(string, string) *structs.Process {
	after := 0

	return func(string, string) *structs.Process {
		if isClosed(ch) {
			if after >= polls {
				return fxProcessExited("complete", options.Int(code))
			}
			after++
		}

		return fxProcess()
	}
}

func TestRunDetachedWaitLogsOptions(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("complete", options.Int(0)), nil)
		i.On("ProcessLogs", "app1", "pid1", structs.LogsOptions{Prefix: options.Bool(true), Since: options.Duration(0)}).Return(logStream(logLine("hello")), nil).Once()

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1", "hello"})
		res.RequireStderr(t, []string{""})
	})
}

func TestRunDetachedWaitStreamsOutput(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", options.Int(3)), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) {
			return logStream(
				"waiting for build pod...\n",
				logLine("<error>x</error>"),
				logLine("100%"),
				logLine(""),
				logLine("   "),
				logLine("\tindented"),
				logLine("ERROR: operation not permitted for deploy keys"),
				"build log stream interrupted, retrying...\n",
				"2026-10-11T14:00:00+02:00 service/web/pid1 offset\n",
				"2026-10-11T12:00:00Z service/web/pid2 other\n",
				logLine("last"),
			), nil
		}}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 3, res.Code)
		require.Equal(t, "Running detached process... OK, pid1\n<error>x</error>\n100%\n\n   \n\tindented\nERROR: operation not permitted for deploy keys\noffset\nlast\n", res.Stdout)
		require.Equal(t, "", res.Stderr)
		require.Equal(t, int32(1), lr.dials.Load())
	})
}

func TestRunDetachedWaitId(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("complete", options.Int(0)), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(logLine("one"), logLine("two")), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w --id", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		require.Equal(t, "pid1", res.Stdout)
		res.RequireStderr(t, []string{"Running detached process... OK, pid1", "one", "two"})
	})
}

func TestRunDetachedWaitDeployKey(t *testing.T) {
	testClientWait(t, 20*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		s := newTestStream("ERROR: operation not permitted for deploy keys\n")
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(s.closed, 5, 3), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) { return s, nil }}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 5", nil)
		require.NoError(t, err)
		require.Equal(t, 3, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
		require.Equal(t, deployKeyNotice, res.Stderr)
		require.Equal(t, int32(1), lr.dials.Load())
	})
}

func TestRunDetachedWaitReconnects(t *testing.T) {
	testClientWait(t, time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		second := newTestStream(logLine("after"), logLine("reconnect"))
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(second.closed, 0, 0), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) {
			if n == 1 {
				return logStream("ERROR: connection failed\n"), nil
			}
			return second, nil
		}}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 5", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1", "after", "reconnect"})
		require.Equal(t, "", res.Stderr)
		require.Equal(t, int32(2), lr.dials.Load())
	})
}

func TestRunDetachedWaitNoReconnectAfterExit(t *testing.T) {
	testClientWait(t, 20*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		s := newTestStream(logLine("done"))
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(s.closed, 1, 0), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) { return s, nil }}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 5", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1", "done"})
		require.Equal(t, int32(1), lr.dials.Load())
	})
}

func TestRunDetachedWaitPendingKeepsStreaming(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		last := newTestStream(logLine("finally"))
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(last.closed, 0, 0), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) {
			if n < 8 {
				return logStream("waiting for build pod to be scheduled...\n", "build pod did not start within 10 minutes -- check cluster resources and node availability\n"), nil
			}
			return last, nil
		}}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1", "finally"})
		require.Equal(t, "", res.Stderr)
		require.Equal(t, int32(8), lr.dials.Load())
	})
}

func TestRunDetachedWaitPrintedResetsRetries(t *testing.T) {
	testClientWait(t, 20*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		last := newTestStream(logLine("line 8"), logLine("line 9"))
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(last.closed, 0, 0), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) {
			if n < 8 {
				return logStream(logLine(fmt.Sprintf("line %d", n))), nil
			}
			return last, nil
		}}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 5", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1", "line 1", "line 2", "line 3", "line 4", "line 5", "line 6", "line 7", "line 8", "line 9"})
		require.Equal(t, "", res.Stderr)
		require.Equal(t, int32(8), lr.dials.Load())
	})
}

func TestRunDetachedWaitOutputStopped(t *testing.T) {
	testClientWait(t, 20*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcess(), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) {
			return nil, fmt.Errorf("dial failed")
		}}
		rack.TestClient = lr

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{
			"WARNING: process output stopped (dial failed); still waiting for the exit status",
			"ERROR: could not confirm the outcome of process pid1: timeout",
			"       convox ps info pid1 -a app1",
		})
		require.Equal(t, int32(6), lr.dials.Load())
	})
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestRunDetachedWaitBrokenOutput(t *testing.T) {
	testClientWait(t, 20*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)

		s := newTestStream(logLine("one"), logLine("two"))
		i.On("ProcessGet", "app1", "pid1").Return(runningUntil(s.closed, 5, 0), nil)

		lr := &logsRack{Interface: i, streams: func(n int) (io.ReadCloser, error) { return s, nil }}
		rack.TestClient = lr

		stderr := bytes.Buffer{}
		e.Reader.Reader = &bytes.Buffer{}
		e.Writer.Color = false
		e.Writer.Stdout = failWriter{}
		e.Writer.Stderr = &stderr

		code := e.Execute([]string{"run", "web", "bash", "-a", "app1", "-d", "-w", "-t", "5"})
		require.Equal(t, 0, code)
		require.Equal(t, "", stderr.String())
		require.Equal(t, int32(1), lr.dials.Load())
	})
}

func TestRunDetachedWaitExitAtTimeout(t *testing.T) {
	testClientWait(t, 667*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
		waitRun(i)
		i.On("ProcessGet", "app1", "pid1").Return(fxProcessExited("failed", options.Int(5)), nil)
		i.On("ProcessLogs", "app1", "pid1", mock.Anything).Return(logStream(), nil)

		res, err := testExecute(e, "run web bash -a app1 -d -w -t 1", nil)
		require.NoError(t, err)
		require.Equal(t, 5, res.Code)
		res.RequireStderr(t, []string{""})
	})
}

func waitRackRoutes(s *stdapi.Server, logs stdapi.HandlerFunc) {
	s.Route("POST", "/apps/{app}/services/{service}/processes", func(c *stdapi.Context) error {
		return c.RenderJSON(fxProcess())
	})
	s.Route("GET", "/apps/{app}/processes/{pid}", func(c *stdapi.Context) error {
		return c.RenderJSON(fxProcessExited("failed", options.Int(3)))
	})
	s.Route("SOCKET", "/apps/{app}/processes/{pid}/logs", logs)
}

func TestRunDetachedWaitStreamsOutput_RealClient(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		testRealRack(t, func(s *stdapi.Server) {
			waitRackRoutes(s, func(c *stdapi.Context) error {
				if c.Header("Since") != "0s" || c.Header("Prefix") != "true" {
					return fmt.Errorf("since %q prefix %q", c.Header("Since"), c.Header("Prefix"))
				}
				_, err := c.Write([]byte("waiting for build pod...\n" + logLine("one") + logLine("two")))
				return err
			})
		})

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 3, res.Code)
		require.Equal(t, "Running detached process... OK, pid1\none\ntwo\n", res.Stdout)
		require.Equal(t, "", res.Stderr)
	})
}

func TestRunDetachedWaitDeployKey_RealClient(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		testRealRack(t, func(s *stdapi.Server) {
			waitRackRoutes(s, func(c *stdapi.Context) error {
				return stdapi.Errorf(401, "operation not permitted for deploy keys")
			})
		})

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
		require.Equal(t, 3, res.Code)
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
		require.Equal(t, deployKeyNotice, res.Stderr)
	})
}
