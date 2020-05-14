package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/rack"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("rack", "get information about the rack", Rack, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack install", "install a new rack", RackInstall, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			stdcli.BoolFlag("prepare", "", "prepare the install but don't run it"),
			stdcli.StringFlag("version", "v", "rack version"),
		},
		Usage:    "<provider> <name> [option=value]...",
		Validate: stdcli.ArgsMin(2),
	})

	register("rack logs", "get logs for the rack", RackLogs, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.LogsOptions{}), flagNoFollow, flagRack),
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack mv", "move a rack to or from console", RackMv, stdcli.CommandOptions{
		Usage:    "<from> <to>",
		Validate: stdcli.Args(2),
	})

	registerWithoutProvider("rack params", "display rack parameters", RackParams, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack params set", "set rack parameters", RackParamsSet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<Key=Value> [Key=Value]...",
		Validate: stdcli.ArgsMin(1),
	})

	register("rack ps", "list rack processes", RackPs, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.SystemProcessesOptions{}), flagRack),
		Validate: stdcli.Args(0),
	})

	register("rack releases", "list rack version history", RackReleases, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("rack scale", "scale the rack", RackScale, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagRack,
			stdcli.IntFlag("count", "c", "instance count"),
			stdcli.StringFlag("type", "t", "instance type"),
		},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack uninstall", "uninstall a rack", RackUninstall, stdcli.CommandOptions{
		Usage:    "<name>",
		Validate: stdcli.Args(1),
	})

	registerWithoutProvider("rack update", "update a rack", RackUpdate, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "[version]",
		Validate: stdcli.ArgsMax(1),
	})
}

func Rack(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	i := c.Info()

	i.Add("Name", s.Name)
	i.Add("Provider", s.Provider)

	if s.Region != "" {
		i.Add("Region", s.Region)
	}

	if s.Domain != "" {
		if ri := s.Outputs["DomainInternal"]; ri != "" {
			i.Add("Router", fmt.Sprintf("%s (external)\n%s (internal)", s.Domain, ri))
		} else {
			i.Add("Router", s.Domain)
		}
	}

	i.Add("Status", s.Status)
	i.Add("Version", s.Version)

	return i.Print()
}

func RackInstall(_ sdk.Interface, c *stdcli.Context) error {
	slug := c.Arg(0)
	name := c.Arg(1)
	args := c.Args[2:]
	version := c.String("version")

	if !provider.Valid(slug) {
		return fmt.Errorf("unknown provider: %s", slug)
	}

	opts := argsToOptions(args)

	if c.Bool("prepare") {
		opts["release"] = version

		md := &rack.Metadata{
			Provider: slug,
			Vars:     opts,
		}

		if _, err := rack.Create(c, name, md); err != nil {
			return err
		}

		return nil
	}

	if err := rack.Install(c, slug, name, version, opts); err != nil {
		return err
	}

	if _, err := rack.Current(c); err != nil {
		if _, err := rack.Switch(c, name); err != nil {
			return err
		}
	}

	return nil
}

func RackLogs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.LogsOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if c.Bool("no-follow") {
		opts.Follow = options.Bool(false)
	}

	opts.Prefix = options.Bool(true)

	r, err := rack.SystemLogs(opts)
	if err != nil {
		return err
	}

	io.Copy(c, r)

	return nil
}

func RackMv(_ sdk.Interface, c *stdcli.Context) error {
	from := c.Arg(0)
	to := c.Arg(1)

	c.Startf("moving rack <rack>%s</rack> to <rack>%s</rack>", from, to)

	fr, err := rack.Load(c, from)
	if err != nil {
		return err
	}

	md, err := fr.Metadata()
	if err != nil {
		return err
	}

	if !md.Deletable {
		return fmt.Errorf("rack %s has dependencies and can not be moved", from)
	}

	if _, err := rack.Create(c, to, md); err != nil {
		return err
	}

	if err := fr.Delete(); err != nil {
		return err
	}

	return c.OK()
}

func RackParams(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	params, err := r.Parameters()
	if err != nil {
		return err
	}

	keys := []string{}

	for k := range params {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	i := c.Info()

	for _, k := range keys {
		i.Add(k, params[k])
	}

	return i.Print()
}

func RackParamsSet(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	c.Startf("Updating parameters")

	if err := r.UpdateParams(argsToOptions(c.Args)); err != nil {
		return err
	}

	return c.OK()
}

func RackPs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.SystemProcessesOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	ps, err := rack.SystemProcesses(opts)
	if err != nil {
		return err
	}

	t := c.Table("ID", "APP", "SERVICE", "STATUS", "RELEASE", "CPU", "MEM", "STARTED", "COMMAND")

	for _, p := range ps {
		t.AddRow(p.Id, p.App, p.Name, p.Status, p.Release, p.CpuString(), p.MemoryString(), common.Ago(p.Started), p.Command)
	}

	return t.Print()
}

func RackReleases(rack sdk.Interface, c *stdcli.Context) error {
	rs, err := rack.SystemReleases()
	if err != nil {
		return err
	}

	t := c.Table("VERSION", "UPDATED")

	for _, r := range rs {
		t.AddRow(r.Id, common.Ago(r.Created))
	}

	return t.Print()
}

func RackScale(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	var opts structs.SystemUpdateOptions
	update := false

	if v, ok := c.Value("count").(int); ok {
		opts.Count = options.Int(v)
		update = true
	}

	if v, ok := c.Value("type").(string); ok {
		opts.Type = options.String(v)
		update = true
	}

	if update {
		c.Startf("Scaling rack")

		if err := rack.SystemUpdate(opts); err != nil {
			return err
		}

		return c.OK()
	}

	i := c.Info()

	i.Add("Autoscale", s.Parameters["Autoscale"])
	i.Add("Count", fmt.Sprintf("%d", s.Count))
	i.Add("Status", s.Status)
	i.Add("Type", s.Type)

	return i.Print()
}

func RackUninstall(_ sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	r, err := rack.Match(c, name)
	if err != nil {
		return err
	}

	if err := r.Uninstall(); err != nil {
		return err
	}

	return nil
}

func RackUpdate(_ sdk.Interface, c *stdcli.Context) error {
	r, err := rack.Current(c)
	if err != nil {
		return err
	}

	if err := r.UpdateVersion(c.Arg(0)); err != nil {
		return err
	}

	return nil
}
