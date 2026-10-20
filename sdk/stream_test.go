package sdk

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type streamErr interface {
	Err() error
}

func websocketServer(t *testing.T, end func(*websocket.Conn), hungUp chan time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !websocket.IsWebSocketUpgrade(r) {
			http.NotFound(w, r)
			return
		}

		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer ws.Close()

		if err := ws.WriteMessage(websocket.TextMessage, []byte("hello\n")); err != nil {
			t.Error(err)
			return
		}

		end(ws)

		if hungUp != nil {
			start := time.Now()
			_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			for {
				if _, _, err := ws.NextReader(); err != nil {
					hungUp <- time.Since(start)
					return
				}
			}
		}
	}))
}

func TestStreamEnd(t *testing.T) {
	tests := []struct {
		name string
		end  func(*websocket.Conn)
		cut  bool
	}{
		{"end frame", func(ws *websocket.Conn) { _ = ws.WriteMessage(websocket.BinaryMessage, []byte{}) }, false},
		{"normal close", func(ws *websocket.Conn) {
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		}, false},
		{"internal error close", func(ws *websocket.Conn) {
			_ = ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, ""))
		}, true},
		{"connection closed", func(ws *websocket.Conn) { _ = ws.NetConn().Close() }, true},
		{"connection reset", func(ws *websocket.Conn) {
			tc, ok := ws.NetConn().(*net.TCPConn)
			if ok {
				_ = tc.SetLinger(0)
			}
			_ = ws.NetConn().Close()
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := websocketServer(t, tt.end, nil)
			defer ts.Close()

			cl, err := New(ts.URL)
			require.NoError(t, err)

			r, err := cl.SystemLogs(structs.LogsOptions{})
			require.NoError(t, err)

			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(data))

			s, ok := r.(streamErr)
			require.True(t, ok)
			require.Equal(t, tt.cut, s.Err() != nil, "Err() = %v", s.Err())
		})
	}
}

func TestStreamHangsUpAfterEndFrame(t *testing.T) {
	hungUp := make(chan time.Duration, 1)

	ts := websocketServer(t, func(ws *websocket.Conn) { _ = ws.WriteMessage(websocket.BinaryMessage, []byte{}) }, hungUp)
	defer ts.Close()

	cl, err := New(ts.URL)
	require.NoError(t, err)

	r, err := cl.SystemLogs(structs.LogsOptions{})
	require.NoError(t, err)

	_, err = io.ReadAll(r)
	require.NoError(t, err)

	select {
	case d := <-hungUp:
		require.Less(t, d, time.Second)
	case <-time.After(3 * time.Second):
		t.Fatal("client kept the socket open after the end frame")
	}
}

type closeSignalListener struct {
	net.Listener
	sendBuffer int
	closed     chan struct{}
}

func (l *closeSignalListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	if tc, ok := c.(*net.TCPConn); ok && l.sendBuffer > 0 {
		_ = tc.SetWriteBuffer(l.sendBuffer)
	}

	return &closeSignalConn{Conn: c, closed: l.closed}, nil
}

type closeSignalConn struct {
	net.Conn
	closed chan struct{}
}

func (c *closeSignalConn) Close() error {
	select {
	case c.closed <- struct{}{}:
	default:
	}
	return c.Conn.Close()
}

func TestAppLogsFromRack(t *testing.T) {
	tests := []struct {
		name string
		cut  bool
	}{
		{"complete", false},
		{"cut", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := stdapi.New("api", "api")
			s.Route("SOCKET", "/apps/{app}/logs", func(c *stdapi.Context) error {
				if _, err := c.Write([]byte("hello\n")); err != nil {
					return err
				}
				if tt.cut {
					return c.Websocket().NetConn().Close()
				}
				return nil
			})

			closed := make(chan struct{}, 1)

			ts := httptest.NewUnstartedServer(s)
			ts.Listener = &closeSignalListener{Listener: ts.Listener, closed: closed}
			ts.Start()
			defer ts.Close()

			cl, err := New(ts.URL)
			require.NoError(t, err)

			r, err := cl.AppLogs("app1", structs.LogsOptions{})
			require.NoError(t, err)

			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(data))

			se, ok := r.(streamErr)
			require.True(t, ok)
			require.Equal(t, tt.cut, se.Err() != nil)

			select {
			case <-closed:
			case <-time.After(3 * time.Second):
				t.Fatal("rack kept the socket open after the client finished")
			}
		})
	}
}

func TestRackStreamReachesSlowReader(t *testing.T) {
	const total = 192 << 10

	returned := make(chan struct{})

	s := stdapi.New("api", "api")
	s.Route("SOCKET", "/apps/{app}/logs", func(c *stdapi.Context) error {
		defer close(returned)

		line := strings.Repeat("x", 1023) + "\n"
		for sent := 0; sent < total; sent += len(line) {
			if _, err := io.WriteString(c, line); err != nil {
				return err
			}
		}
		return nil
	})

	closed := make(chan struct{}, 1)

	ts := httptest.NewUnstartedServer(s)
	ts.Listener = &closeSignalListener{Listener: ts.Listener, sendBuffer: 64 << 10, closed: closed}
	ts.Start()
	defer ts.Close()

	ws, res, err := websocket.DefaultDialer.Dial(strings.Replace(ts.URL, "http", "ws", 1)+"/apps/app1/logs", nil)
	require.NoError(t, err)
	defer res.Body.Close()

	tc, ok := ws.NetConn().(*net.TCPConn)
	require.True(t, ok)
	require.NoError(t, tc.SetReadBuffer(64<<10))

	stop := make(chan struct{})
	pinged := make(chan struct{})
	go func() {
		defer close(pinged)
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				_ = ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second))
			}
		}
	}()

	require.NoError(t, ws.SetReadDeadline(time.Now().Add(20*time.Second)))

	got, end := 0, false
	for !end {
		mt, r, err := ws.NextReader()
		require.NoError(t, err, "after %d of %d bytes", got, total)

		if mt == websocket.BinaryMessage {
			end = true
			continue
		}

		n, err := io.Copy(io.Discard, r)
		require.NoError(t, err)

		if got < 32<<10 && got+int(n) >= 32<<10 {
			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("handler still writing, socket buffers too small for this test")
			}

			time.Sleep(100 * time.Millisecond)
			runtime.GC()
			runtime.GC()
			time.Sleep(100 * time.Millisecond)
		}

		got += int(n)
	}

	close(stop)
	<-pinged
	require.NoError(t, ws.Close())
	require.Equal(t, total, got)

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("rack kept the socket open after the client hung up")
	}
}
