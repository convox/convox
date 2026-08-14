package sdk

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// framesReader yields each frame on a successive Read, mirroring how the
// websocket transport delivers an exec stream as discrete chunks (a real SSM
// session blob spans several frames because the token is multi-KB).
type framesReader struct {
	frames [][]byte
	i      int
}

func (f *framesReader) Read(p []byte) (int, error) {
	if f.i >= len(f.frames) {
		return 0, io.EOF
	}
	n := copy(p, f.frames[f.i])
	f.i++
	return n, nil
}

func stubPlugin(t *testing.T) (*int, *ecsExecSession) {
	t.Helper()
	orig := runSessionManagerPlugin
	t.Cleanup(func() { runSessionManagerPlugin = orig })
	calls := 0
	got := &ecsExecSession{}
	runSessionManagerPlugin = func(s ecsExecSession) (int, error) {
		calls++
		*got = s
		return 0, nil
	}
	return &calls, got
}

func TestExecStreamDockerStream(t *testing.T) {
	calls, _ := stubPlugin(t)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader([]byte("hello world"+statusCodePrefix+"7\n")), out)
	require.NoError(t, err)
	require.Equal(t, 7, code)
	require.Equal(t, "hello world", out.String())
	require.Equal(t, 0, *calls)
}

func TestExecStreamECSSessionSingleFrame(t *testing.T) {
	calls, got := stubPlugin(t)
	blob := append([]byte{ecsExecSessionByte}, []byte(`{"sessionId":"sid-1","streamUrl":"wss://x/sid-1","tokenValue":"tok-1","region":"us-east-2"}`)...)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader(blob), out)
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, 1, *calls)
	require.Equal(t, "sid-1", got.SessionID)
	require.Equal(t, "us-east-2", got.Region)
	require.Empty(t, out.String(), "session blob must not leak to the terminal")
}

func TestExecStreamECSSessionMultiFrame(t *testing.T) {
	// The real production path: a multi-KB SSM token splits the blob across frames.
	calls, got := stubPlugin(t)
	body := `{"sessionId":"sid-2","streamUrl":"wss://x/sid-2","tokenValue":"` + strings.Repeat("t", 4096) + `","region":"us-west-2"}`
	blob := append([]byte{ecsExecSessionByte}, []byte(body)...)

	var frames [][]byte
	for i := 0; i < len(blob); i += 1024 {
		end := i + 1024
		if end > len(blob) {
			end = len(blob)
		}
		frames = append(frames, blob[i:end])
	}
	out := &bytes.Buffer{}

	code, err := execStream(&framesReader{frames: frames}, out)
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, 1, *calls, "plugin must be launched exactly once")
	require.Equal(t, "sid-2", got.SessionID)
	require.Equal(t, "us-west-2", got.Region)
	require.Empty(t, out.String(), "multi-frame session blob must not leak to the terminal")
}

func TestExecStreamNulInLaterFrameNotTriggered(t *testing.T) {
	// A 0x00 at the start of a non-first frame of a normal stream must NOT be
	// treated as the ECS discriminator (first-frame anchoring).
	calls, _ := stubPlugin(t)
	frames := [][]byte{
		[]byte("hello "),
		append([]byte{ecsExecSessionByte}, []byte("world"+statusCodePrefix+"0\n")...),
	}
	out := &bytes.Buffer{}

	code, err := execStream(&framesReader{frames: frames}, out)
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, 0, *calls, "a later-frame 0x00 must not launch the plugin")
	require.Equal(t, "hello \x00world", out.String())
}

func TestExecStreamECSSessionTruncated(t *testing.T) {
	// Connection drops mid-handshake: discriminator seen but JSON never completes
	// -> error, not a silent exit 0.
	calls, _ := stubPlugin(t)
	frames := [][]byte{
		append([]byte{ecsExecSessionByte}, []byte(`{"sessionId":"sid-3"`)...),
	}
	out := &bytes.Buffer{}

	code, err := execStream(&framesReader{frames: frames}, out)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrExecIncomplete), "the ECS branch keeps its own error")
	require.Equal(t, -1, code)
	require.Equal(t, 0, *calls)
	require.Empty(t, out.String())
}

func TestExecStreamNoMarker(t *testing.T) {
	// The defect: a stream that ends before the marker was indistinguishable from
	// a clean exit 0.
	stubPlugin(t)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader([]byte("hello world")), out)
	require.ErrorIs(t, err, ErrExecIncomplete)
	require.Equal(t, 0, code)
	require.Equal(t, "hello world", out.String())
}

// requireOutput asserts that every byte of real output arrived. A marker that
// straddles a read is recognized only once its last byte lands, so the bytes of
// it that were already written can trail the output; nothing else may.
func requireOutput(t *testing.T, want, got string, msg ...interface{}) {
	t.Helper()
	require.True(t, strings.HasPrefix(got, want), msg...)
	require.True(t, strings.HasPrefix(statusCodePrefix, strings.TrimPrefix(got, want)),
		"only marker bytes may trail the output, got %q", got)
}

func TestExecStreamMarkerSplitAtEveryOffset(t *testing.T) {
	// The transport delivers at most 1024 bytes per read and a short read can land
	// anywhere inside the marker. A split must never cost the code or invent a
	// failure on a run that succeeded.
	for _, code := range []int{0, 7, 137, -1} {
		stream := []byte(fmt.Sprintf("hello world%s%d\n", statusCodePrefix, code))

		for split := 1; split < len(stream); split++ {
			stubPlugin(t)
			out := &bytes.Buffer{}
			frames := [][]byte{stream[:split], stream[split:]}

			got, err := execStream(&framesReader{frames: frames}, out)
			require.NoError(t, err, "code %d split at %d", code, split)
			require.Equal(t, code, got, "code %d split at %d", code, split)
			requireOutput(t, "hello world", out.String(), "code %d split at %d", code, split)
		}
	}
}

func TestExecStreamMarkerOneBytePerRead(t *testing.T) {
	stubPlugin(t)
	stream := []byte("hi" + statusCodePrefix + "42\n")

	var frames [][]byte
	for i := range stream {
		frames = append(frames, stream[i:i+1])
	}
	out := &bytes.Buffer{}

	code, err := execStream(&framesReader{frames: frames}, out)
	require.NoError(t, err)
	require.Equal(t, 42, code)
	requireOutput(t, "hi", out.String())
}

func TestExecStreamMarkerCutBeforeNewline(t *testing.T) {
	// The code arrived, the newline did not. Pre-existing behavior is to trust it.
	stubPlugin(t)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader([]byte("out"+statusCodePrefix+"3")), out)
	require.NoError(t, err)
	require.Equal(t, 3, code)
	require.Equal(t, "out", out.String())
}

func TestExecStreamMarkerCutBeforeCode(t *testing.T) {
	stubPlugin(t)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader([]byte("out"+statusCodePrefix)), out)
	require.ErrorIs(t, err, ErrExecIncomplete)
	require.Equal(t, 0, code)
	require.Equal(t, "out", out.String())
}

func TestExecStreamPrefixInOutput(t *testing.T) {
	// Output that happens to carry the sentinel must not be swallowed, and must not
	// blackhole the rest of the stream waiting for a code that never comes.
	stubPlugin(t)
	noise := statusCodePrefix + strings.Repeat("x", 64)
	out := &bytes.Buffer{}

	code, err := execStream(bytes.NewReader([]byte(noise+statusCodePrefix+"9\n")), out)
	require.NoError(t, err)
	require.Equal(t, 9, code)
	require.Equal(t, noise, out.String())
}

func TestWebsocketExitStreamNoMarker(t *testing.T) {
	// Deliberately unchanged: the endpoints behind WebsocketExit are interactive
	// shells, not deploy gates, and two of them have no readable server source.
	out := &bytes.Buffer{}

	code, err := websocketExitStream(bytes.NewReader([]byte("hello world")), out)
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "hello world", out.String())
}

func TestExecBodyHoldsEOFUntilClose(t *testing.T) {
	// stdsdk sends an empty binary frame when the request body ends, and a
	// Console-proxied rack reads that as a closed stream and kills the command.
	b := newExecBody(strings.NewReader("in"))
	p := make([]byte, 8)

	n, err := b.Read(p)
	require.NoError(t, err)
	require.Equal(t, "in", string(p[:n]))

	blocked := make(chan error, 1)
	go func() {
		_, rerr := b.Read(p)
		blocked <- rerr
	}()

	select {
	case <-blocked:
		require.Fail(t, "the body reported EOF before the exec finished")
	case <-time.After(100 * time.Millisecond):
	}

	b.close()
	require.Equal(t, io.EOF, <-blocked)
}

// methods.go is generated; ProcessExec is hand-maintained to relay the ECS Exec
// protocol via execStream and to hold the request body open via newExecBody. A
// regen would revert it to a plain WebsocketExit call and silently break ECS Exec
// (the unused linter would not notice, because these tests reference execStream
// directly). This guard fails loudly if that happens.
func TestProcessExecWiredToExecStream(t *testing.T) {
	src, err := os.ReadFile("methods.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "return execStream(ws, rw)",
		"sdk/methods.go ProcessExec must relay via execStream; if methods.go was regenerated, reapply the ECS Exec wrapper")
	require.Contains(t, string(src), "newExecBody(rw)",
		"sdk/methods.go ProcessExec must wrap the request body with newExecBody; if methods.go was regenerated, reapply it")
}
