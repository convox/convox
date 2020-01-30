package cli

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("racks", "list available racks", Racks, stdcli.CommandOptions{
		Validate: stdcli.Args(0),
	})
}

func Racks(rack sdk.Interface, c *stdcli.Context) error {
	rs, err := racks(c)
	if err != nil {
		return err
	}

	t := c.Table("NAME", "PROVIDER", "STATUS")

	for _, r := range rs {
		t.AddRow(r.Name, r.Provider, r.Status)
	}

	return t.Print()
}
