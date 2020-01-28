package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("rack", "get information about the rack", Rack, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	registerWithoutProvider("rack install", "install a new rack", RackInstall, stdcli.CommandOptions{
		Usage:    "<provider> <name> [option=value]...",
		Validate: stdcli.ArgsMin(2),
	})

	register("rack logs", "get logs for the rack", RackLogs, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.LogsOptions{}), flagNoFollow, flagRack),
		Validate: stdcli.Args(0),
	})

	register("rack params", "display rack parameters", RackParams, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("rack params set", "set rack parameters", RackParamsSet, stdcli.CommandOptions{
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
		Usage:    "<name> [option=value]...",
		Validate: stdcli.ArgsMin(1),
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

func RackInstall(rack sdk.Interface, c *stdcli.Context) error {
	provider := c.Arg(0)
	name := c.Arg(1)

	env, err := terraformEnv(provider)
	if err != nil {
		return err
	}

	dir, err := c.SettingDirectory(fmt.Sprintf("racks/%s", name))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	vars, err := terraformProviderVars(provider)
	if err != nil {
		return err
	}

	ov, err := terraformOptionVars(dir, c.Args[2:])
	if err != nil {
		return err
	}

	for k, v := range ov {
		vars[k] = v
	}

	tf := filepath.Join(dir, "main.tf")

	if _, err := os.Stat(tf); !os.IsNotExist(err) {
		return fmt.Errorf("rack name in use: %s", name)
	}

	params := map[string]interface{}{
		"Name":     name,
		"Provider": provider,
		"Vars":     vars,
	}

	if err := terraformWriteTemplate(tf, params); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "init"); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "apply", "-auto-approve"); err != nil {
		return err
	}

	if err := switchRack(c, name); err != nil {
		return err
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

func RackParams(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	keys := []string{}

	for k := range s.Parameters {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	i := c.Info()

	for _, k := range keys {
		i.Add(k, s.Parameters[k])
	}

	return i.Print()
}

func RackParamsSet(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	opts := structs.SystemUpdateOptions{
		Parameters: map[string]string{},
	}

	for _, arg := range c.Args {
		parts := strings.SplitN(arg, "=", 2)

		if len(parts) != 2 {
			return fmt.Errorf("Key=Value expected: %s", arg)
		}

		opts.Parameters[parts[0]] = parts[1]
	}

	c.Startf("Updating parameters")

	if s.Version <= "20180708231844" {
		if err := rack.AppParametersSet(s.Name, opts.Parameters); err != nil {
			return err
		}
	} else {
		if err := rack.SystemUpdate(opts); err != nil {
			return err
		}
	}

	c.Writef("\n")

	if err := common.WaitForRackWithLogs(rack, c); err != nil {
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

	t := c.Table("ID", "APP", "SERVICE", "STATUS", "RELEASE", "STARTED", "COMMAND")

	for _, p := range ps {
		t.AddRow(p.Id, p.App, p.Name, p.Status, p.Release, common.Ago(p.Started), p.Command)
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

func RackUninstall(rack sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	r, err := matchRack(c, name)
	if err != nil {
		return err
	}

	if r.Remote {
		return rackUninstallRemote(c, name)
	}

	env, err := terraformEnv(r.Provider)
	if err != nil {
		return err
	}

	dir, err := c.SettingDirectory(fmt.Sprintf("racks/%s", name))
	if err != nil {
		return err
	}

	if err := terraform(c, dir, env, "init", "-upgrade"); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "destroy", "-auto-approve"); err != nil {
		return err
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	return nil
}

func RackUpdate(rack sdk.Interface, c *stdcli.Context) error {
	name := c.Arg(0)

	r, err := matchRack(c, name)
	if err != nil {
		return err
	}

	if r.Remote {
		return rackUpdateRemote(c, name)
	}

	dir, err := c.SettingDirectory(fmt.Sprintf("racks/%s", name))
	if err != nil {
		return err
	}

	env, err := terraformEnv(r.Provider)
	if err != nil {
		return err
	}

	vars, err := terraformProviderVars(r.Provider)
	if err != nil {
		return err
	}

	ov, err := terraformOptionVars(dir, c.Args[1:])
	if err != nil {
		return err
	}

	for k, v := range ov {
		vars[k] = v
	}

	tf := filepath.Join(dir, "main.tf")

	params := map[string]interface{}{
		"Name":     name,
		"Provider": r.Provider,
		"Vars":     vars,
	}

	if err := terraformWriteTemplate(tf, params); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "init", "-upgrade"); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "apply", "-auto-approve"); err != nil {
		return err
	}

	return nil
}

func rackUninstallRemote(c *stdcli.Context, name string) error {
	return fmt.Errorf("uninstalling remote racks not yet supported")
}

func rackUpdateRemote(c *stdcli.Context, name string) error {
	return fmt.Errorf("updating remote racks not yet supported")
}
