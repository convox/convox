package cli

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("services", "list services for an app", watch(Services), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack, flagWatchInterval},
		Validate: stdcli.Args(0),
	}, WithCloud())

	register("services restart", "restart a service", ServicesRestart, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("services update", "update a service", ServicesUpdate, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.ServiceUpdateOptions{}), flagApp, flagRack),
		Usage:    "<service>",
		Validate: stdcli.Args(1),
	}, WithCloud())
}

func Services(rack sdk.Interface, c *stdcli.Context) error {

	ss, err := rack.ServiceList(app(c))
	if err != nil {
		return err
	}

	t := c.Table("SERVICE", "DOMAIN", "PORTS")

	for _, s := range ss {
		ports := []string{}

		for _, p := range s.Ports {
			port := fmt.Sprintf("%d", p.Container)

			if p.Balancer != 0 {
				port = fmt.Sprintf("%d:%d", p.Balancer, p.Container)
			}

			ports = append(ports, port)
		}

		t.AddRow(s.Name, s.Domain, strings.Join(ports, " "))
	}

	return t.Print()
}

func ServicesRestart(rack sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	c.Startf("Restarting <service>%s</service>", name)

	if err := rack.ServiceRestart(app(c), name); err != nil {
		return err
	}

	return c.OK()
}

func ServicesUpdate(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.ServiceUpdateOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	name := c.Arg(0)

	c.Startf("Updating <service>%s</service>", name)

	if err := rack.ServiceUpdate(app(c), name, opts); err != nil {
		return err
	}

	if err := c.Writef("\n"); err != nil {
		return err
	}

	if err := common.WaitForAppWithLogs(rack, c, app(c)); err != nil {
		return err
	}

	return c.OK()
}
