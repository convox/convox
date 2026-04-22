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

	// NLB PORTS column only renders when at least one service declares nlb: ports.
	// v3 racks never populate Nlb; v2 racks populate it from the release manifest.
	// Index-based iteration avoids copying the full Service struct per scan step.
	hasNlb := false
	for i := range ss {
		if len(ss[i].Nlb) > 0 {
			hasNlb = true
			break
		}
	}

	headers := []string{"SERVICE", "DOMAIN", "PORTS"}
	if hasNlb {
		headers = append(headers, "NLB PORTS")
	}
	t := c.Table(headers...)

	for _, s := range ss {
		ports := []string{}

		for _, p := range s.Ports {
			port := fmt.Sprintf("%d", p.Container)

			if p.Balancer != 0 {
				port = fmt.Sprintf("%d:%d", p.Balancer, p.Container)
			}

			ports = append(ports, port)
		}

		row := []string{s.Name, s.Domain, strings.Join(ports, " ")}

		if hasNlb {
			nlbs := []string{}
			for _, np := range s.Nlb {
				cell := fmt.Sprintf("%d:%d", np.Port, np.ContainerPort)
				if np.Protocol == "tls" {
					cell += "/tls"
				}
				if np.Scheme == "internal" {
					cell += "(internal)"
				}
				// cz = CrossZone, allow = AllowCIDR count, pcip = PreserveClientIP.
				// Abbreviations and order match v2 CLI output exactly.
				var attrs []string
				if np.CrossZone != nil {
					attrs = append(attrs, fmt.Sprintf("cz=%t", *np.CrossZone))
				}
				if len(np.AllowCIDR) > 0 {
					attrs = append(attrs, fmt.Sprintf("allow=%d", len(np.AllowCIDR)))
				}
				if np.PreserveClientIP != nil {
					attrs = append(attrs, fmt.Sprintf("pcip=%t", *np.PreserveClientIP))
				}
				if len(attrs) > 0 {
					cell += "[" + strings.Join(attrs, " ") + "]"
				}
				nlbs = append(nlbs, cell)
			}
			row = append(row, strings.Join(nlbs, " "))
		}

		t.AddRow(row...)
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
