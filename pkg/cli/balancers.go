package cli

import (
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("balancers", "list balancers for an app", Balancers, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Validate: stdcli.Args(0),
	})
}

func Balancers(rack sdk.Interface, c *stdcli.Context) error {
	bs, err := rack.BalancerList(app(c))
	if err != nil {
		return err
	}

	t := c.Table("BALANCER", "SERVICE", "ENDPOINT")

	for _, b := range bs {
		t.AddRow(b.Name, b.Service, b.Endpoint)
	}

	return t.Print()
}
