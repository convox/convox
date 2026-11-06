package cli_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProxy(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		i.On("WithContext", ctx).Return(i)
		opts := structs.ProxyOptions{
			TLS: options.Bool(false),
		}
		i.On("Proxy", "test.example.org", 5000, mock.Anything, opts).Return(nil).Run(func(args mock.Arguments) {
			buf := make([]byte, 2)
			rwc := args.Get(2).(io.ReadWriteCloser)
			n, err := rwc.Read(buf)
			require.NoError(t, err)
			require.Equal(t, 2, n)
			require.Equal(t, "in", string(buf))
			n, err = rwc.Write([]byte("out"))
			require.NoError(t, err)
			require.Equal(t, 3, n)
			rwc.Close()
		})

		port := freePort(t)

		ch := make(chan *result)

		go func() {
			res, err := testExecuteContext(ctx, e, fmt.Sprintf("proxy %d:test.example.org:5000", port), nil)
			require.NoError(t, err)
			ch <- res
		}()

		cn := dialEventually(t, port)

		cn.Write([]byte("in"))

		data, err := io.ReadAll(cn)
		require.NoError(t, err)
		require.Equal(t, "out", string(data))

		cancel()

		res := <-ch

		require.NotNil(t, res)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			fmt.Sprintf("proxying localhost:%d to test.example.org:5000", port),
			fmt.Sprintf("connect: %d", port),
		})
	})
}

func TestProxyError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		i.On("WithContext", ctx).Return(i)
		opts := structs.ProxyOptions{
			TLS: options.Bool(false),
		}
		i.On("Proxy", "test.example.org", 5000, mock.Anything, opts).Return(fmt.Errorf("err1"))

		port := freePort(t)

		ch := make(chan *result)

		go func() {
			res, err := testExecuteContext(ctx, e, fmt.Sprintf("proxy %d:test.example.org:5000", port), nil)
			require.NoError(t, err)
			ch <- res
		}()

		cn := dialEventually(t, port)

		cn.Write([]byte("in"))

		data, _ := io.ReadAll(cn)
		require.Len(t, data, 0)

		cancel()
		// cli.ProxyCloser <- nil

		res := <-ch

		require.NotNil(t, res)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{
			fmt.Sprintf("proxying localhost:%d to test.example.org:5000", port),
			fmt.Sprintf("connect: %d", port),
		})
	})
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.NoError(t, ln.Close())
	return addr.Port
}

func dialEventually(t *testing.T, port int) net.Conn {
	t.Helper()
	var cn net.Conn
	require.Eventually(t, func() bool {
		var err error
		cn, err = net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		return err == nil
	}, 5*time.Second, 20*time.Millisecond)
	return cn
}
