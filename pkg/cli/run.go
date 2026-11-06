package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

const (
	waitRetainSeconds = 60
	runLogRetries     = 5
	runLogGrace       = 2 * time.Second
)

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

	out := c.Writer().Stdout

	signal.Ignore(syscall.SIGPIPE)

	logs := openProcessLogs(rack, app(c), pid, out)
	retries := runLogRetries

	follow := func(terminal bool) {
		if logs == nil {
			return
		}

		if terminal {
			select {
			case <-logs.done:
			case <-time.After(runLogGrace):
				return
			}
		} else {
			select {
			case <-logs.done:
			default:
				return
			}
		}

		switch {
		case logs.denied:
			fmt.Fprintln(c.Writer().Stderr, "WARNING: process output is not available to this deploy key; assign it a role with read access to the app to see output here")
		case logs.broken || terminal:
		case !logs.seen:
			// reconnecting a poll later keeps the rack's whole-second start from repeating lines
			logs.seen = true
			return
		default:
			if logs.printed || time.Since(logs.opened) >= 12*WaitDuration {
				retries = runLogRetries
			}

			if retries > 0 {
				retries--
				logs = openProcessLogs(rack, app(c), pid, out)
				return
			}

			if logs.message != "" {
				fmt.Fprintf(c.Writer().Stderr, "WARNING: process output stopped (%s); still waiting for the exit status\n", logs.message)
			} else {
				fmt.Fprintln(c.Writer().Stderr, "WARNING: process output stopped; still waiting for the exit status")
			}
		}

		logs = nil
	}

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
			follow(false)
			return false, nil
		}

		done = ps

		return true, nil
	})
	if err != nil && done == nil {
		return fmt.Errorf("could not confirm the outcome of process %s: %s\n       convox ps info %s -a %s", pid, err, pid, app(c))
	}

	follow(true)

	if done.ExitCode == nil {
		return fmt.Errorf("process %s ended with status %s but the rack did not report an exit status.\n       It may have been stopped before its command ran, or the rack may predate the release that reports one", pid, done.Status)
	}

	return stdcli.Exit(*done.ExitCode)
}

type processLogs struct {
	done    chan struct{}
	opened  time.Time
	printed bool
	denied  bool
	broken  bool
	seen    bool
	message string
}

func openProcessLogs(rack sdk.Interface, app, pid string, w io.Writer) *processLogs {
	l := &processLogs{done: make(chan struct{}), opened: time.Now()}

	go func() {
		defer close(l.done)
		defer func() { _ = recover() }()

		// an absent Since becomes the rack's 2m default, which it adds to now
		r, err := rack.ProcessLogs(app, pid, structs.LogsOptions{Prefix: options.Bool(true), Since: options.Duration(0)})
		if err != nil {
			l.message = err.Error()
			return
		}

		defer r.Close()

		br := bufio.NewReader(r)

		for {
			line, err := br.ReadString('\n')
			line = strings.TrimSuffix(line, "\n")

			if text, ok := processLogText(line, pid); ok {
				if _, err := io.WriteString(w, text+"\n"); err != nil {
					l.broken = true
					return
				}
				l.printed = true
			} else if line == "ERROR: operation not permitted for deploy keys" {
				l.denied = true
				return
			} else if strings.HasPrefix(line, "ERROR: ") {
				l.message = strings.TrimPrefix(line, "ERROR: ")
				return
			}

			if err != nil {
				return
			}
		}
	}()

	return l
}

func processLogText(line, pid string) (string, bool) {
	parts := strings.SplitN(line, " ", 3)

	if len(parts) != 3 || !strings.HasPrefix(parts[1], "service/") || !strings.HasSuffix(parts[1], "/"+pid) {
		return "", false
	}

	if _, err := time.Parse(time.RFC3339, parts[0]); err != nil {
		return "", false
	}

	return parts[2], true
}
