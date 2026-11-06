package api_test

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type logsRoute struct {
	name string
	path string
	args []interface{}
}

var logsRoutes = []logsRoute{
	{"AppLogs", "/apps/app1/logs", []interface{}{"app1", mock.Anything}},
	{"BuildLogs", "/apps/app1/builds/build1/logs", []interface{}{"app1", "build1", mock.Anything}},
	{"ProcessLogs", "/apps/app1/processes/pid1/logs", []interface{}{"app1", "pid1", mock.Anything}},
	{"ServiceLogs", "/apps/app1/services/web/logs", []interface{}{"app1", "web", mock.Anything}},
	{"SystemLogs", "/system/logs", []interface{}{mock.Anything}},
}

type watchedReader struct {
	*io.PipeReader
	once   sync.Once
	closed chan struct{}
}

func (r *watchedReader) Close() error {
	err := r.PipeReader.Close()
	r.once.Do(func() { close(r.closed) })
	return err
}

func logsStream(t *testing.T, c *stdsdk.Client, p *structs.MockProvider, rt logsRoute, r io.ReadCloser) *websocket.Conn {
	p.On(rt.name, rt.args...).Return(r, nil)

	conn, res, err := websocket.DefaultDialer.Dial("ws://"+c.Endpoint.Host+rt.path, nil)
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	t.Cleanup(func() { conn.Close() })

	return conn
}

func pipeStream(t *testing.T) (*watchedReader, *io.PipeWriter) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { pw.Close() })
	return &watchedReader{PipeReader: pr, closed: make(chan struct{})}, pw
}

func readText(t *testing.T, conn *websocket.Conn) string {
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	mt, data, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, mt)
	return string(data)
}

func TestLogsClientGone(t *testing.T) {
	leave := map[string]func(*websocket.Conn) error{
		"tcp": func(conn *websocket.Conn) error {
			return conn.NetConn().Close()
		},
		"close": func(conn *websocket.Conn) error {
			return conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		},
	}

	for _, rt := range logsRoutes {
		for how, fn := range leave {
			t.Run(rt.name+"/"+how, func(t *testing.T) {
				testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
					r, pw := pipeStream(t)
					conn := logsStream(t, c, p, rt, r)

					go func() { _, _ = pw.Write([]byte("line\n")) }()
					require.Equal(t, "line\n", readText(t, conn))

					require.NoError(t, fn(conn))

					select {
					case <-r.closed:
					case <-time.After(2 * time.Second):
						t.Fatal("provider reader not closed after the client left")
					}

					_, err := pw.Write([]byte{})
					require.ErrorIs(t, err, io.ErrClosedPipe)
				})
			})
		}
	}
}

func TestLogsClientFramesIgnored(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r, pw := pipeStream(t)
		conn := logsStream(t, c, p, logsRoutes[0], r)

		pongs := 0
		conn.SetPongHandler(func(string) error { pongs++; return nil })

		require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, []byte{}))
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("hello")))
		require.NoError(t, conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)))

		time.Sleep(200 * time.Millisecond)

		go func() { _, _ = pw.Write([]byte("line\n")) }()
		require.Equal(t, "line\n", readText(t, conn))
		require.Equal(t, 0, pongs)

		select {
		case <-r.closed:
			t.Fatal("provider reader closed while the client is connected")
		default:
		}
	})
}

func TestLogsFiniteStreamDrains(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		conn := logsStream(t, c, p, logsRoutes[0], io.NopCloser(strings.NewReader("test")))

		require.Equal(t, "test", readText(t, conn))

		mt, data, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, mt)
		require.Empty(t, data)

		require.NoError(t, conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))

		require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
		_, _, err = conn.ReadMessage()
		require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure), "got %v", err)
	})
}

func TestLogsCopyErrorReachesClient(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r, pw := pipeStream(t)
		conn := logsStream(t, c, p, logsRoutes[0], r)

		require.NoError(t, pw.CloseWithError(errors.New("err2")))

		require.Equal(t, "ERROR: err2\n", readText(t, conn))

		mt, data, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.BinaryMessage, mt)
		require.Empty(t, data)
	})
}
