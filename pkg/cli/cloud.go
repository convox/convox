package cli

import (
	"fmt"

	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("cloud machines", "list machines", watch(Machines), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagMachine, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

}

func Machines(cc sdk.Interface, c *stdcli.Context) error {
	ms, err := rack.ListMachines(c)
	if err != nil {
		return err
	}

	t := c.Table("NAME", "CPU", "MEMORY", "AGE")

	for _, m := range ms {
		t.AddRow(fmt.Sprintf("%s/%s", m.OrganizationInfo["name"], m.Name), m.Dimensions["vcpu"], m.Dimensions["mem"], m.Age)
	}

	return t.Print()
}
