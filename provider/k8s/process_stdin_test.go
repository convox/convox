package k8s

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStdinPipeEndsOnCleanEnd(t *testing.T) {
	read := make(chan []byte, 1)

	r := stdinPipe(strings.NewReader("hello\n"))

	go func() {
		b, _ := io.ReadAll(r)
		read <- b
	}()

	select {
	case b := <-read:
		require.Equal(t, "hello\n", string(b))
	case <-time.After(5 * time.Second):
		t.Fatal("stdin never reached end of input")
	}
}

func TestStdinPipeStaysOpenOnReadError(t *testing.T) {
	r := stdinPipe(io.MultiReader(strings.NewReader("partial"), iotest.ErrReader(errors.New("boom"))))

	b := make([]byte, len("partial"))
	_, err := io.ReadFull(r, b)
	require.NoError(t, err)
	require.Equal(t, "partial", string(b))

	returned := make(chan struct{})

	go func() {
		var next [1]byte
		_, _ = r.Read(next[:])
		close(returned)
	}()

	select {
	case <-returned:
		t.Fatal("stdin reached end of input after a failed read")
	case <-time.After(500 * time.Millisecond):
	}
}
