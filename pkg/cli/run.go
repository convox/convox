package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	entrypoint := stdcli.BoolFlag("entrypoint", "e", "set to false to disable the original entrypoint in your container")
	entrypoint.Default = true

	register("run", "execute a command in a new process", Run, stdcli.CommandOptions{
		Flags: append(
			stdcli.OptionFlags(structs.ProcessRunOptions{}),
			flagRack,
			flagApp,
			stdcli.BoolFlag("detach", "d", "run process in the background"),
			stdcli.IntFlag("timeout", "t", "timeout"),
			entrypoint,
		),
		Usage:    "<service> <command>",
		Validate: stdcli.ArgsMin(2),
	}, WithCloud())
}

func Run(rack sdk.Interface, c *stdcli.Context) error {
	service := c.Arg(0)
	command := strings.Join(c.Args[1:], " ")

	var opts structs.ProcessRunOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	opts.Command = options.String(command)

	timeout := 3600

	if t := c.Int("timeout"); t > 0 {
		timeout = t
	}

	if w, h, err := c.TerminalSize(); err == nil {
		opts.Height = options.Int(h)
		opts.Width = options.Int(w)
	}

	restore := c.TerminalRaw()
	defer restore()

	if c.Bool("detach") {
		c.Startf("Running detached process")

		ps, err := rack.ProcessRun(app(c), service, opts)
		if err != nil {
			return err
		}

		return c.OK(ps.Id)
	}

	opts.Command = options.String(fmt.Sprintf("sleep %d", timeout))

	ps, err := rack.ProcessRun(app(c), c.Arg(0), opts)
	if err != nil {
		return err
	}

	defer rack.ProcessStop(app(c), ps.Id)

	if err := common.WaitForProcessRunning(rack, c, app(c), ps.Id); err != nil {
		return err
	}

	eopts := structs.ProcessExecOptions{
		Entrypoint: options.Bool(c.Bool("entrypoint")),
		Height:     opts.Height,
		Width:      opts.Width,
	}

	if !stdcli.IsTerminal(os.Stdin) {
		eopts.Tty = options.Bool(false)
	}

	code, err := rack.ProcessExec(app(c), ps.Id, command, c, eopts)
	if err != nil {
		return err
	}

	return stdcli.Exit(code)
}
