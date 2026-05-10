package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("scale", "scale a service", Scale, stdcli.CommandOptions{
		Flags: append(stdcli.OptionFlags(structs.ServiceUpdateOptions{}), flagApp, flagRack, flagWatchInterval),
		Usage: "[<service>]",
		Validate: func(c *stdcli.Context) error {
			if scaleHasImperativeFlag(c) {
				if len(c.Args) < 1 {
					return fmt.Errorf("service name required")
				}
				return stdcli.Args(1)(c)
			}
			return stdcli.ArgsMax(1)(c)
		},
	}, WithCloud())
}

func scaleHasImperativeFlag(c *stdcli.Context) bool {
	return c.Value("count") != nil || c.Value("cpu") != nil || c.Value("memory") != nil ||
		c.Value("gpu") != nil || c.Value("gpu-vendor") != nil ||
		c.Value("min") != nil || c.Value("max") != nil
}

func scaleOptsImperative(opts structs.ServiceUpdateOptions) bool {
	return opts.Count != nil || opts.Cpu != nil || opts.Memory != nil ||
		opts.Gpu != nil || opts.GpuVendor != nil ||
		opts.Min != nil || opts.Max != nil
}

// scaleRackAtLeast3246 reports whether the rack version string parses
// to 3.24.6 or later. Strips a leading "v" and any prerelease or
// build-metadata suffix (semver "-" / "+" markers), then compares
// major.minor.patch. Conservatively returns false on parse error so
// an unparseable version does not bypass the range-mode gate.
func scaleRackAtLeast3246(version string) bool {
	v := strings.TrimPrefix(version, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return false
	}
	nums := [3]int{}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return false
		}
		nums[i] = n
	}
	target := [3]int{3, 24, 6}
	for i := 0; i < 3; i++ {
		if nums[i] != target[i] {
			return nums[i] > target[i]
		}
	}
	return true
}

func Scale(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.ServiceUpdateOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if scaleOptsImperative(opts) {
		if opts.Count != nil && (opts.Min != nil || opts.Max != nil) {
			return fmt.Errorf("--min/--max and --count are mutually exclusive")
		}
		if opts.Min != nil && *opts.Min < 0 {
			return fmt.Errorf("--min must be >= 0")
		}
		if opts.Max != nil && *opts.Max < 1 {
			return fmt.Errorf("--max must be >= 1")
		}
		if opts.Min != nil && opts.Max != nil && *opts.Max < *opts.Min {
			return fmt.Errorf("--max must be >= --min")
		}

		service := c.Arg(0)

		// Range mode (--min / --max) is a 3.24.6+ rack-side surface.
		// Older racks silently ignore unknown JSON fields, leaving the
		// CLI claiming success while the rack does nothing. Probe the
		// rack version and surface a clean upgrade-required error
		// instead of letting the user think the scale change applied.
		// --count is the long-standing wire surface; skip the probe so
		// existing scripts targeting older racks keep working.
		if opts.Min != nil || opts.Max != nil {
			sys, err := rack.SystemGet()
			if err != nil {
				return err
			}
			if !scaleRackAtLeast3246(sys.Version) {
				return fmt.Errorf("--min / --max range scale requires rack version 3.24.6 or later; rack reports %q", sys.Version)
			}
		}

		if opts.Min != nil && *opts.Min == 0 {
			if err := scalePreflightDeadPods(rack, c, service); err != nil {
				return err
			}
		}

		c.Startf("Scaling <service>%s</service>", service)

		if err := rack.ServiceUpdate(app(c), service, opts); err != nil {
			return err
		}

		c.Writef("\n")

		if err := common.WaitForAppWithLogs(rack, c, app(c)); err != nil {
			return err
		}

		return c.OK()
	}

	// Read-mode optional positional filters the table to one service.
	// Pre-watch existence check ensures a typo errors once and exits non-zero
	// rather than spamming "not found" every watch tick.
	filterService := c.Arg(0)
	if filterService != "" {
		ss, err := rack.ServiceList(app(c))
		if err != nil {
			return err
		}
		found := false
		for i := range ss {
			if ss[i].Name == filterService {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("service %q not found in app %s", filterService, app(c))
		}
	}

	return watch(func(r sdk.Interface, c *stdcli.Context) error {
		running := map[string]int{}

		ss, err := rack.ServiceList(app(c))
		if err != nil {
			return err
		}

		// Filter to the named service. Mid-watch service deletion produces
		// an empty filtered slice (empty rendered table) rather than per-tick
		// errors; pre-watch validation already confirmed the service existed.
		if filterService != "" {
			filtered := make([]structs.Service, 0, 1)
			for i := range ss {
				if ss[i].Name == filterService {
					filtered = append(filtered, ss[i])
					break
				}
			}
			ss = filtered
		}

		sort.Slice(ss, func(i, j int) bool { return ss[i].Name < ss[j].Name })

		ps, err := rack.ProcessList(app(c), structs.ProcessListOptions{})
		if err != nil {
			return err
		}

		for _, p := range ps {
			running[p.Name] += 1
		}

		showAutoscale := false
		for i := range ss {
			if ss[i].Autoscale != nil && ss[i].Autoscale.Enabled {
				showAutoscale = true
				break
			}
		}

		// budgetCapStatus is best-effort; errors are logged inside the helper.
		// Use WithServices variant since the watch closure already has ss.
		cs, _ := budgetCapStatusWithServices(rack, app(c), ss, c.Writer().Stderr)

		// Column-position contract: positions 1-6 (SERVICE, DESIRED, RUNNING,
		// CPU, MEMORY, GPU) match 3.24.5 exactly so user scripts parsing
		// `convox scale` output positionally (`awk '{print $2}'`, `cut -f3`)
		// keep working unchanged. New columns (MIN, MAX, AUTOSCALE, STATUS)
		// append at positions 7+. AUTOSCALE renders only when at least one
		// service has autoscale enabled. STATUS is always present at the
		// trailing position so the cap-state sub-token (`armed-Nm`,
		// `at-cap-keda`, etc.) and the cold-start hint have a stable home.
		headers := []string{"SERVICE", "DESIRED", "RUNNING", "CPU", "MEMORY", "GPU", "MIN", "MAX"}
		if showAutoscale {
			headers = append(headers, "AUTOSCALE")
		}
		headers = append(headers, "STATUS")
		t := c.Table(headers...)

		for i := range ss {
			s := &ss[i]
			gpu := "-"
			if s.Gpu > 0 {
				gpu = fmt.Sprintf("%d", s.Gpu)
			}

			row := []string{
				s.Name,
				fmt.Sprintf("%d", s.Count),
				fmt.Sprintf("%d", running[s.Name]),
				fmt.Sprintf("%d", s.Cpu),
				fmt.Sprintf("%d", s.Memory),
				gpu,
				intPtrCell(s.Min),
				intPtrCell(s.Max),
			}
			if showAutoscale {
				row = append(row, formatAutoscaleSummary(s.Autoscale))
			}
			status := ""
			if s.ColdStart != nil && *s.ColdStart {
				status = "COLD (~2-5m first req)"
			}
			// Append at-cap* sub-state when budget cap state is true.
			// Short forms only here: long forms (e.g. `at-cap (keda-managed)`)
			// are reserved for `convox budget show` banner per R3.
			if cs.AtCap {
				sub := capSubStateToken(s.Name, cs)
				if status == "" {
					status = sub
				} else {
					status = status + " " + sub
				}
			}
			row = append(row, status)

			t.AddRow(row...)
		}

		return t.Print()
	})(rack, c)
}

func intPtrCell(p *int) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}

func formatAutoscaleSummary(a *structs.ServiceAutoscaleState) string {
	if a == nil || !a.Enabled {
		return "-"
	}
	parts := []string{}
	if a.CpuThreshold != nil {
		parts = append(parts, fmt.Sprintf("cpu>%d", *a.CpuThreshold))
	}
	if a.MemThreshold != nil {
		parts = append(parts, fmt.Sprintf("mem>%d", *a.MemThreshold))
	}
	if a.GpuThreshold != nil {
		parts = append(parts, fmt.Sprintf("gpu-util>%d", *a.GpuThreshold))
	}
	if a.QueueThreshold != nil {
		parts = append(parts, fmt.Sprintf("queue>%d", *a.QueueThreshold))
	}
	if a.CustomTriggers > 0 {
		parts = append(parts, fmt.Sprintf("custom=%d", a.CustomTriggers))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

func scalePreflightDeadPods(rack sdk.Interface, c *stdcli.Context, service string) error {
	ss, err := rack.ServiceList(app(c))
	if err != nil {
		return err
	}
	for i := range ss {
		s := &ss[i]
		if s.Name == service {
			if s.Autoscale != nil && s.Autoscale.Enabled {
				return nil
			}
			return fmt.Errorf(
				"service %s has no autoscale configured; --min 0 would terminate pods with no wake-up mechanism. Set scale.autoscale.* in convox.yml and promote a release first, or use --min 1+",
				service,
			)
		}
	}
	return fmt.Errorf("service %s not found in app %s", service, app(c))
}
