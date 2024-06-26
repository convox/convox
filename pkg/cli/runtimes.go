package cli

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("runtimes", "get list of runtimes", Runtimes, stdcli.CommandOptions{
		Usage:    "<orgname>",
		Validate: stdcli.ArgsMin(1),
	})
}

func Runtimes(runtime sdk.Interface, c *stdcli.Context) error {
	org := c.Arg(0)

	rs, err := runtime.OrganizationRuntimes(org)
	if err != nil {
		return err
	}

	t := c.Table("ID", "TITLE")
	for _, r := range rs {
		t.AddRow(r.Id, r.Title)
	}

	return t.Print()
}
