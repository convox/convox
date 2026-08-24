package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

const waitRetainSeconds = 60

func init() {
	entrypoint := stdcli.BoolFlag("entrypoint", "e", "set to false to disable the original entrypoint in your container")
	entrypoint.Default = true

	register("run", "execute a command in a new process", Run, stdcli.CommandOptions{
		Flags: append(
			stdcli.OptionFlags(structs.ProcessRunOptions{}),
			flagRack,
			flagApp,
			flagId,
			stdcli.BoolFlag("detach", "d", "run process in the background"),
			stdcli.IntFlag("timeout", "t", "timeout"),
			stdcli.IntFlag("retain", "", "with --detach, seconds to keep the finished process readable by ps info"),
			stdcli.BoolFlag("wait", "w", "with --detach, wait for the process to finish and exit with its status"),
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

	if c.Bool("detach") {
		return runDetached(rack, c, service, &opts, timeout)
	}

	if c.Int("retain") > 0 {
		return fmt.Errorf("--retain is only valid with --detach")
	}

	// raw mode belongs only to the attached path, which is the one that reads stdin;
	// setting it around a --wait would clear ISIG and swallow the caller's ctrl-c
	restore := c.TerminalRaw()
	defer restore()

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
		return execExitError(err, "This run's process has been stopped.")
	}

	return stdcli.Exit(code)
}

func runDetached(rack sdk.Interface, c *stdcli.Context, service string, opts *structs.ProcessRunOptions, timeout int) error {
	wait := c.Bool("wait")

	// divert before anything is printed so the id lands on stdout by itself
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	retain := c.Int("retain")

	// --wait has to read the record back, so the record must outlive a poll
	if wait && retain < waitRetainSeconds {
		retain = waitRetainSeconds
	}

	if retain > 0 {
		opts.Retain = options.Int(retain)
	}

	c.Startf("Running detached process")

	ps, err := rack.ProcessRun(app(c), service, *opts)
	if err != nil {
		return err
	}

	if err := c.OK(ps.Id); err != nil {
		return err
	}

	if stdout != nil {
		fmt.Fprintf(stdout, "%s", ps.Id)
	}

	if !wait {
		fmt.Fprintf(c.Writer().Stderr, "  convox logs -a %s\n  convox ps stop %s -a %s\n", app(c), ps.Id, app(c))

		return nil
	}

	return waitForProcessExit(rack, c, ps.Id, timeout)
}

func waitForProcessExit(rack sdk.Interface, c *stdcli.Context, pid string, timeout int) error {
	var done *structs.Process

	// common.Wait wants two consecutive successes; latch so the second never races cleanup
	err := common.Wait(WaitDuration, time.Duration(timeout)*time.Second, 2, func() (bool, error) {
		if done != nil {
			return true, nil
		}

		ps, err := rack.ProcessGet(app(c), pid)
		if err != nil {
			return false, err
		}

		if !ps.Terminal() {
			return false, nil
		}

		done = ps

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("could not confirm the outcome of process %s: %s\n       convox ps info %s -a %s", pid, err, pid, app(c))
	}

	if done.ExitCode == nil {
		return fmt.Errorf("process %s ended with status %s but the rack did not report an exit status.\n       It may have been stopped before its command ran, or the rack may predate the release that reports one", pid, done.Status)
	}

	return stdcli.Exit(*done.ExitCode)
}
