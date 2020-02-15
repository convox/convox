package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/router"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}

func run() error {
	common.InitializeKlog()

	r, err := router.New()
	if err != nil {
		return err
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go handleSignals(r, ch)

	return r.Serve()
}

func handleSignals(r *router.Router, ch <-chan os.Signal) {
	sig := <-ch

	fmt.Printf("ns=rack at=signal signal=%v terminate=true\n", sig)

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	r.Shutdown(ctx)

	os.Exit(0)
}
