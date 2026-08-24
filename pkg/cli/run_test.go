package cli_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
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
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Running detached process... OK, pid1"})
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
		res.RequireStderr(t, []string{""})
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

		res, err := testExecute(e, "run web bash -a app1 -d -w", nil)
		require.NoError(t, err)
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
		res.RequireStderr(t, []string{"Running detached process... OK, pid1"})
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
