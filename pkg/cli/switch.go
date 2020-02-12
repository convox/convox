package cli

import (
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("switch", "switch current rack", Switch, stdcli.CommandOptions{
		Validate: stdcli.ArgsMax(1),
	})
}

func Switch(_ sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	if name == "" {
		r, err := rack.Current(c)
		if err != nil {
			return err
		}

		c.Writef("%s\n", r.Name())

		return nil
	}

	r, err := rack.Switch(c, name)
	if err != nil {
		return err
	}

	c.Writef("Switched to <rack>%s</rack>\n", r.Name())

	return nil
}
