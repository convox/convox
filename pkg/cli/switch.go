package cli

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("switch", "switch current rack", Switch, stdcli.CommandOptions{
		Validate: stdcli.ArgsMax(1),
	})
}

func Switch(rack sdk.Interface, c *stdcli.Context) error {
	host, err := currentHost(c)
	if err != nil {
		return err
	}

	name := c.Arg(0)

	if name == "" {
		c.Writef("%s\n", currentRack(c, host))
		return nil
	}

	r, err := matchRack(c, name)
	if err != nil {
		return err
	}

	if err := switchRack(c, r.Name); err != nil {
		return err
	}

	c.Writef("Switched to <rack>%s</rack>\n", r.Name)

	return nil
}
