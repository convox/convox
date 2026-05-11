package cli

import (
	"fmt"
	"strconv"
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

	register("services triggers enable", "enable Console-driven autoscale triggers for a service", ServicesTriggersEnable, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp, flagRack,
			stdcli.IntFlag("min", "", "minimum replicas"),
			stdcli.IntFlag("max", "", "maximum replicas"),
			stdcli.IntFlag("cpu", "", "CPU target utilization (1-100)"),
			stdcli.IntFlag("memory", "", "memory target utilization (1-100)"),
			stdcli.IntFlag("gpu", "", "GPU target utilization (1-100); requires KEDA"),
			stdcli.IntFlag("queue", "", "queue-depth target (>=1); requires KEDA"),
		},
		Usage:    "<service>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("services triggers disable", "disable Console-driven autoscale triggers override", ServicesTriggersDisable, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "<service>",
		Validate: stdcli.Args(1),
	}, WithCloud())

	register("services triggers threshold-set", "update one trigger threshold on an active override", ServicesTriggersThresholdSet, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp, flagRack,
			stdcli.StringFlag("type", "", "trigger type (cpu|memory|gpu|queue)"),
			stdcli.StringFlag("threshold", "", "new threshold value (numeric)"),
		},
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
	hasAutoscale := false
	hasCold := false
	hasScale := false
	for i := range ss {
		if len(ss[i].Nlb) > 0 {
			hasNlb = true
		}
		if ss[i].Autoscale != nil && ss[i].Autoscale.Enabled {
			hasAutoscale = true
		}
		if ss[i].ColdStart != nil && *ss[i].ColdStart {
			hasCold = true
		}
		if ss[i].Min != nil || ss[i].Max != nil {
			hasScale = true
		}
	}

	// budgetCapStatusWithServices is best-effort and CLI-side only. The BUDGET
	// column only renders when the app is at-cap so users without budget
	// configured see zero output difference (purely additive — pre-3.24.6
	// callers see no new column). Use the WithServices variant since we already
	// have ss in hand — avoids a redundant rack.ServiceList round-trip.
	cs, _ := budgetCapStatusWithServices(rack, app(c), ss, c.Writer().Stderr)
	hasBudget := cs.AtCap

	headers := []string{"SERVICE", "DOMAIN", "PORTS"}
	if hasNlb {
		headers = append(headers, "NLB PORTS")
	}
	if hasScale {
		headers = append(headers, "SCALE")
	}
	if hasAutoscale {
		headers = append(headers, "AUTOSCALE")
	}
	if hasCold {
		headers = append(headers, "COLD")
	}
	if hasBudget {
		headers = append(headers, "BUDGET")
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

		if hasScale {
			row = append(row, formatScaleCell(s.Min, s.Max))
		}
		if hasAutoscale {
			row = append(row, formatAutoscaleSummary(s.Autoscale))
		}
		if hasCold {
			cold := "-"
			if s.ColdStart != nil && *s.ColdStart {
				cold = "yes"
			}
			row = append(row, cold)
		}
		if hasBudget {
			row = append(row, capSubStateToken(s.Name, cs))
		}

		t.AddRow(row...)
	}

	return t.Print()
}

func formatScaleCell(min, max *int) string {
	if min == nil && max == nil {
		return "-"
	}
	if min != nil && max != nil {
		if *min == *max {
			return fmt.Sprintf("%d", *min)
		}
		return fmt.Sprintf("%d-%d", *min, *max)
	}
	if min != nil {
		return fmt.Sprintf("%d-", *min)
	}
	return fmt.Sprintf("-%d", *max)
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

// cliTriggerTypeAliases maps the CLI-facing short forms to the canonical
// wire types shared between rack handler, SDK, and GraphQL enum.
var cliTriggerTypeAliases = map[string]string{
	"cpu":            structs.TriggerTypeCPU,
	"memory":         structs.TriggerTypeMemory,
	"gpu":            structs.TriggerTypeGPUUtilization,
	"gpuUtilization": structs.TriggerTypeGPUUtilization,
	"queue":          structs.TriggerTypeQueueDepth,
	"queueDepth":     structs.TriggerTypeQueueDepth,
}

func ServicesTriggersEnable(rack sdk.Interface, c *stdcli.Context) error {
	service := c.Arg(0)

	opts := structs.ServiceTriggersOptions{
		Min: c.Int("min"),
		Max: c.Int("max"),
	}
	if v := c.Int("cpu"); v > 0 {
		opts.Triggers = append(opts.Triggers, structs.TriggerSpec{Type: structs.TriggerTypeCPU, Threshold: float64(v)})
	}
	if v := c.Int("memory"); v > 0 {
		opts.Triggers = append(opts.Triggers, structs.TriggerSpec{Type: structs.TriggerTypeMemory, Threshold: float64(v)})
	}
	if v := c.Int("gpu"); v > 0 {
		opts.Triggers = append(opts.Triggers, structs.TriggerSpec{Type: structs.TriggerTypeGPUUtilization, Threshold: float64(v)})
	}
	if v := c.Int("queue"); v > 0 {
		opts.Triggers = append(opts.Triggers, structs.TriggerSpec{Type: structs.TriggerTypeQueueDepth, Threshold: float64(v)})
	}
	if len(opts.Triggers) == 0 {
		return fmt.Errorf("at least one of --cpu, --memory, --gpu, --queue is required")
	}

	if err := servicesTriggersRackAtLeast3246(rack, "services triggers enable"); err != nil {
		return err
	}

	c.Startf("Enabling triggers override on <service>%s</service>", service)
	if err := rack.ServiceTriggersEnable(app(c), service, opts, ""); err != nil {
		return err
	}
	return c.OK()
}

func ServicesTriggersDisable(rack sdk.Interface, c *stdcli.Context) error {
	service := c.Arg(0)

	if err := servicesTriggersRackAtLeast3246(rack, "services triggers disable"); err != nil {
		return err
	}

	c.Startf("Disabling triggers override on <service>%s</service>", service)
	if err := rack.ServiceTriggersDisable(app(c), service, ""); err != nil {
		return err
	}
	return c.OK()
}

func ServicesTriggersThresholdSet(rack sdk.Interface, c *stdcli.Context) error {
	service := c.Arg(0)
	rawType := strings.TrimSpace(c.String("type"))
	canonical, ok := cliTriggerTypeAliases[rawType]
	if !ok || canonical == "" {
		return fmt.Errorf("invalid --type %q; expected cpu|memory|gpu|queue", rawType)
	}
	thresholdStr := strings.TrimSpace(c.String("threshold"))
	if thresholdStr == "" {
		return fmt.Errorf("--threshold is required")
	}
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return fmt.Errorf("--threshold must be a number: %v", err)
	}

	if err := servicesTriggersRackAtLeast3246(rack, "services triggers threshold-set"); err != nil {
		return err
	}

	c.Startf("Setting <service>%s</service> %s threshold to %s", service, rawType, thresholdStr)
	if err := rack.ServiceTriggersThresholdSet(app(c), service, canonical, threshold, ""); err != nil {
		return err
	}
	return c.OK()
}

// servicesTriggersRackAtLeast3246 reuses the scale-side rack-version
// helper so any pre-3.24.6 rack target gets a clean upgrade-required
// error instead of the older rack silently 404-ing the new endpoint.
func servicesTriggersRackAtLeast3246(rack sdk.Interface, commandName string) error {
	sys, err := rack.SystemGet()
	if err != nil {
		return err
	}
	if !scaleRackAtLeast3246(sys.Version) {
		return fmt.Errorf("%s requires rack version 3.24.6 or later; rack reports %q", commandName, sys.Version)
	}
	return nil
}
