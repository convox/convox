package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/convox/convox/pkg/start"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("start", "start an application for local development", Start, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			flagApp,
			stdcli.BoolFlag("external", "e", "use external build",false),
			stdcli.StringFlag("manifest", "m", "manifest file"),
			stdcli.StringFlag("generation", "g", "generation"),
			stdcli.BoolFlag("no-build", "", "skip build",false),
			stdcli.BoolFlag("no-cache", "", "build withoit layer cache",false),
			stdcli.BoolFlag("no-sync", "", "do not sync local changes into the running containers",false),
			stdcli.IntFlag("shift", "s", "shift local port numbers (generation 1 only)"),
		},
		Usage: "[service] [service...]",
	})
}

func Start(rack sdk.Interface, c *stdcli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())

	go handleInterrupt(cancel)

	if c.String("generation") == "1" || c.LocalSetting("generation") == "1" || filepath.Base(c.String("manifest")) == "docker-compose.yml" {
		return fmt.Errorf("gen1 is no longer supported")
	}

	opts := start.Options2{
		App:      app(c),
		Build:    !c.Bool("no-build"),
		Cache:    !c.Bool("no-cache"),
		External: c.Bool("external"),
		Manifest: c.String("manifest"),
		Provider: rack,
		Sync:     !c.Bool("no-sync"),
	}

	if len(c.Args) > 0 {
		opts.Services = c.Args
	}

	return Starter.Start2(ctx, c, opts)
}

func handleInterrupt(cancel context.CancelFunc) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	<-ch
	fmt.Println("")
	cancel()
}
