package sdk

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

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
	require.Equal(t, -1, code)
	require.Equal(t, 0, *calls)
	require.Empty(t, out.String())
}

// methods.go is generated; ProcessExec is hand-maintained to relay the ECS Exec
// protocol via execStream. A regen would revert it to a plain WebsocketExit call
// and silently break ECS Exec (the unused linter would not notice, because these
// tests reference execStream directly). This guard fails loudly if that happens.
func TestProcessExecWiredToExecStream(t *testing.T) {
	src, err := os.ReadFile("methods.go")
	require.NoError(t, err)
	require.Contains(t, string(src), "return execStream(ws, rw)",
		"sdk/methods.go ProcessExec must relay via execStream; if methods.go was regenerated, reapply the ECS Exec wrapper")
}
