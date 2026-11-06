package cli

import (
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("racks", "list available racks", watch(Racks), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagWatchInterval},
		Validate: stdcli.Args(0),
	})
}

func Racks(_ sdk.Interface, c *stdcli.Context) error {
	rs, err := rack.List(c)
	if err != nil {
		return err
	}

	t := c.Table("NAME", "PROVIDER", "STATUS")

	for _, r := range rs {
		t.AddRow(r.Name(), r.Provider(), r.Status())
	}

	return t.Print()
}
