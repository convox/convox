package cli

import (
	"fmt"
	"strings"

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
