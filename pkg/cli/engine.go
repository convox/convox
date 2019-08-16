package cli

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/convox/stdcli"
)

type Engine struct {
	*stdcli.Engine
	Provider structs.Provider
}

type HandlerFunc func(structs.Provider, *stdcli.Context) error

func (e *Engine) Command(command, description string, fn HandlerFunc, opts stdcli.CommandOptions) {
	wfn := func(c *stdcli.Context) error {
		p, err := e.currentProvider(c)
		if err != nil {
			return err
		}

		return fn(p, c)
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

func (e *Engine) currentProvider(c *stdcli.Context) (structs.Provider, error) {
	name := rack(c)

	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("must specify a rack name")
	}

	return k8s.New(name)
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
