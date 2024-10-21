package cli

import (
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("runtimes", "get list of runtimes", Runtimes, stdcli.CommandOptions{
		Usage:    "<orgname>",
		Validate: stdcli.ArgsMin(1),
	})
}

func Runtimes(_ sdk.Interface, c *stdcli.Context) error {
	org := c.Arg(0)

	rs, err := rack.Listruntimes(c, org)
	if err != nil {
		return err
	}

	t := c.Table("ID", "TITLE")
	for _, r := range rs {
		t.AddRow(r.Id, r.Title)
	}

	return t.Print()
}
