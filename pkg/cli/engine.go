package cli

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

type Engine struct {
	*stdcli.Engine
	Client sdk.Interface
}

func (e *Engine) Command(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		return fn(e.currentClient(c), c)
	}

	e.Engine.Command(command, description, wfn, opts)
}

func (e *Engine) CommandWithoutProvider(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		return fn(nil, c)
	}

	e.Engine.Command(command, description, wfn, opts)
}

func (e *Engine) RegisterCommands() {
	for _, c := range commands {
		if c.Rack {
			e.Command(c.Command, c.Description, c.Handler, c.Opts)
		} else {
			e.CommandWithoutProvider(c.Command, c.Description, c.Handler, c.Opts)
		}
	}
}

func (e *Engine) currentClient(c *stdcli.Context) sdk.Interface {
	if e.Client != nil {
		return e.Client
	}

	// url, err := currentEndpoint(c)
	// if err != nil {
	// 	c.Fail(err)
	// }

	host, err := currentHost(c)
	if err != nil {
		c.Fail(err)
	}

	r, err := matchRack(c, currentRack(c, host))
	if err != nil {
		c.Fail(err)
	}

	// if r == nil {
	// 	return nil
	// }

	sc, err := sdk.New(r.Url)
	if err != nil {
		c.Fail(err)
	}

	sc.Authenticator = authenticator(c)
	sc.Rack = r.Name
	sc.Session = currentSession(c)

	return sc
}

var commands = []command{}

type command struct {
	Command     string
	Description string
	Handler     HandlerFunc
	Opts        stdcli.CommandOptions
	Rack        bool
}

func register(cmd, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	commands = append(commands, command{
		Command:     cmd,
		Description: description,
		Handler:     fn,
		Opts:        opts,
		Rack:        true,
	})
}

func registerWithoutProvider(cmd, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	commands = append(commands, command{
		Command:     cmd,
		Description: description,
		Handler:     fn,
		Opts:        opts,
		Rack:        false,
	})
}
