package start

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/convox/convox/pkg/prefix"
	"github.com/convox/exec"
)

var (
	Exec exec.Interface = &exec.Exec{}
)

type Interface interface {
	Start2(context.Context, io.Writer, Options2) error
}

type Start struct{}

func New() Interface {
	return &Start{}
}

func prefixHash(prefix string) int {
	sum := 0

	for c := range prefix {
		sum += int(c)
	}

	return sum % 18
}

func prefixWriter(w io.Writer, services map[string]bool) prefix.Writer {
	prefixes := map[string]string{
		"build":  "system",
		"convox": "system",
	}

	for s := range services {
		prefixes[s] = fmt.Sprintf("color%d", prefixHash(s))
	}

	return prefix.NewWriter(w, prefixes)
}

func handleInterrupt(fn func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	<-ch
	fmt.Println("")
	fn()
}
