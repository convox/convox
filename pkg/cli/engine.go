package cli

import (
	"fmt"

	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

type Engine struct {
	*stdcli.Engine
	Client sdk.Interface
}

func (e *Engine) Command(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		r, err := rack.Current(c)
		if err != nil {
			return err
		}

		rc, err := r.Client()
		if err != nil {
			return err
		}

		return fn(rc, c)
	}

	// the wait command flag is added for making the cli tool v2 backwards compatible
	flagWait.SkipHelpCommand = true
	opts.Flags = append(opts.Flags, flagWait)

	e.Engine.Command(command, description, wfn, opts)
}

func (e *Engine) CommandWithCloud(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		machine := c.String("machine")
		if machine == "" {
			return fmt.Errorf("machine not specified")
		}
		cc, err := rack.CurrentConsoleClientWithMachine(c, machine)
		if err != nil {
			return err
		}

		return fn(cc, c)
	}

	e.Engine.Command(command, description, wfn, opts)
}

func (e *Engine) CommandWithoutProvider(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		return fn(nil, c)
	}

	// the wait command flag is added for making the cli tool v2 backwards compatible
	flagWait.SkipHelpCommand = true
	opts.Flags = append(opts.Flags, flagWait)

	e.Engine.Command(command, description, wfn, opts)
}

func (e *Engine) RegisterCommands() {
	for _, c := range commands {
		if c.Rack {
			e.Command(c.Command, c.Description, c.Handler, c.Opts)
		} else if c.Cloud {
			e.CommandWithCloud(c.Command, c.Description, c.Handler, c.Opts)
		} else {
			e.CommandWithoutProvider(c.Command, c.Description, c.Handler, c.Opts)
		}
	}
}

var commands = []command{}

type command struct {
	Command     string
	Description string
	Handler     HandlerFunc
	Opts        stdcli.CommandOptions
	Rack        bool
	Cloud       bool
}

type RegisterCmdOptions struct {
	Cloud bool
}

type RegisterCmdOptionsFunc func(*RegisterCmdOptions)

func WithCloud() RegisterCmdOptionsFunc {
	return func(opts *RegisterCmdOptions) {
		opts.Cloud = true
	}
}

func register(cmd, description string, fn HandlerFunc, opts stdcli.CommandOptions, regOpts ...RegisterCmdOptionsFunc) {
	rco := &RegisterCmdOptions{}
	for _, ro := range regOpts {
		ro(rco)
	}

	commands = append(commands, command{
		Command:     cmd,
		Description: description,
		Handler:     fn,
		Opts:        opts,
		Rack:        true,
		Cloud:       false,
	})

	if rco.Cloud {
		for i := range opts.Flags {
			if opts.Flags[i].Name == "rack" {
				opts.Flags[i] = flagMachine
				break
			}
		}
		commands = append(commands, command{
			Command:     fmt.Sprintf("cloud %s", cmd),
			Description: description,
			Handler:     fn,
			Opts:        opts,
			Rack:        false,
			Cloud:       true,
		})
	}
}

func registerWithoutProvider(cmd, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	commands = append(commands, command{
		Command:     cmd,
		Description: description,
		Handler:     fn,
		Opts:        opts,
		Rack:        false,
		Cloud:       false,
	})
}

func registerWithCloud(cmd, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	commands = append(commands, command{
		Command:     cmd,
		Description: description,
		Handler:     fn,
		Opts:        opts,
		Rack:        false,
		Cloud:       true,
	})
}
