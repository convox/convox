package cli

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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
	register("apps", "list apps", watch(Apps), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

	register("apps cancel", "cancel an app update", AppsCancel, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps create", "create an app", AppsCreate, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.AppCreateOptions{}), flagRack),
		Usage:    "[name]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps delete", "delete an app", AppsDelete, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})

	register("apps export", "export an app", AppsExport, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagRack,
			stdcli.StringFlag("file", "f", "export to file"),
		},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps import", "import an app", AppsImport, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagRack,
			stdcli.StringFlag("file", "f", "import from file"),
		},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps info", "get information about an app", AppsInfo, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps lock", "enable termination protection", AppsLock, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps params", "display app parameters", AppsParams, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps params set", "set app parameters", AppsParamsSet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "<Key=Value> [Key=Value]...",
		Validate: stdcli.ArgsMin(1),
	})

	register("apps unlock", "disable termination protection", AppsUnlock, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagApp, flagRack},
		Usage:    "[app]",
		Validate: stdcli.ArgsMax(1),
	})
}

func Apps(rack sdk.Interface, c *stdcli.Context) error {
	as, err := rack.AppList()
	if err != nil {
		return err
	}

	t := c.Table("APP", "STATUS", "RELEASE")

	for _, a := range as {
		t.AddRow(a.Name, a.Status, a.Release)
	}

	return t.Print()
}

func AppsCancel(rack sdk.Interface, c *stdcli.Context) error {
	aname := coalesce(c.Arg(0), app(c))

	c.Writef("Cancelling deployment of <app>%s</app>...\n", aname)

	if err := rack.AppCancel(aname); err != nil {
		return err
	}

	c.Writef("Rewriting last active release...\n")

	rl, err := common.ReleaseLatest(rack, aname)

	if err != nil {
		return fmt.Errorf("failed to fetch last active release - %s", err.Error())
	}

	_, err = rack.ReleaseCreate(aname, structs.ReleaseCreateOptions{Build: &rl.Build, Description: &rl.Description, Env: &rl.Env})
	if err != nil {
		return err
	}

	return c.OK()
}

func AppsCreate(rack sdk.Interface, c *stdcli.Context) error {
	app := coalesce(c.Arg(0), app(c))

	var opts structs.AppCreateOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	c.Startf("Creating <app>%s</app>", app)

	if _, err := rack.AppCreate(app, opts); err != nil {
		return err
	}

	if err := common.WaitForAppRunning(rack, app); err != nil {
		return err
	}

	return c.OK()
}

func AppsDelete(rack sdk.Interface, c *stdcli.Context) error {
	app := c.Args[0]

	c.Startf("Deleting <app>%s</app>", app)

	if err := rack.AppDelete(app); err != nil {
		return err
	}

	if err := common.WaitForAppDeleted(rack, c, app); err != nil {
		return err
	}

	return c.OK()
}

func AppsExport(rack sdk.Interface, c *stdcli.Context) error {
	app := coalesce(c.Arg(0), app(c))

	var w io.Writer

	if file := c.String("file"); file != "" {
		f, err := os.Create(file)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	} else {
		if c.Writer().IsTerminal() {
			return fmt.Errorf("pipe this command into a file or specify --file")
		}
		w = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	if err := appExport(rack, c, app, w); err != nil {
		return err
	}

	return nil
}

func AppsImport(rack sdk.Interface, c *stdcli.Context) error {
	app := coalesce(c.Arg(0), app(c))

	var r io.ReadCloser

	if file := c.String("file"); file != "" {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		r = f
	} else {
		if c.Reader().IsTerminal() {
			return fmt.Errorf("pipe a file into this command or specify --file")
		}
		r = ioutil.NopCloser(c.Reader())
	}

	defer r.Close()

	if err := appImport(rack, c, app, r); err != nil {
		return err
	}

	return nil
}

func AppsInfo(rack sdk.Interface, c *stdcli.Context) error {
	a, err := rack.AppGet(coalesce(c.Arg(0), app(c)))
	if err != nil {
		return err
	}

	i := c.Info()

	i.Add("Name", a.Name)
	i.Add("Status", a.Status)

	i.Add("Generation", a.Generation)
	i.Add("Locked", fmt.Sprintf("%t", a.Locked))
	i.Add("Release", a.Release)

	if a.Router != "" {
		i.Add("Router", a.Router)
	}

	return i.Print()
}

func AppsLock(rack sdk.Interface, c *stdcli.Context) error {
	app := coalesce(c.Arg(0), app(c))

	c.Startf("Locking <app>%s</app>", app)

	if err := rack.AppUpdate(app, structs.AppUpdateOptions{Lock: options.Bool(true)}); err != nil {
		return err
	}

	return c.OK()
}

func AppsParams(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	var params map[string]string

	app := coalesce(c.Arg(0), app(c))

	if s.Version <= "20180708231844" {
		params, err = rack.AppParametersGet(app)
		if err != nil {
			return err
		}
	} else {
		a, err := rack.AppGet(app)
		if err != nil {
			return err
		}
		params = a.Parameters
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

func AppsParamsSet(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	opts := structs.AppUpdateOptions{
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
		if err := rack.AppParametersSet(app(c), opts.Parameters); err != nil {
			return err
		}
	} else {
		if err := rack.AppUpdate(app(c), opts); err != nil {
			return err
		}
	}

	c.Writef("\n")

	if err := common.WaitForAppWithLogs(rack, c, app(c)); err != nil {
		return err
	}

	a, err := rack.AppGet(app(c))
	if err != nil {
		return err
	}

	for k, v := range opts.Parameters {
		if a.Parameters[k] != v {
			return fmt.Errorf("failed to set params")
		}
	}

	return c.OK()
}

func AppsUnlock(rack sdk.Interface, c *stdcli.Context) error {
	app := coalesce(c.Arg(0), app(c))

	c.Startf("Unlocking <app>%s</app>", app)

	if err := rack.AppUpdate(app, structs.AppUpdateOptions{Lock: options.Bool(false)}); err != nil {
		return err
	}

	return c.OK()
}

func appExport(rack sdk.Interface, c *stdcli.Context, app string, w io.Writer) error {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	c.Startf("Exporting app <app>%s</app>", app)

	a, err := rack.AppGet(app)
	if err != nil {
		return err
	}

	for k, v := range a.Parameters {
		if v == "****" {
			delete(a.Parameters, k)
		}
	}

	data, err := json.Marshal(a)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(tmp, "app.json"), data, 0600); err != nil {
		return err
	}

	c.OK()

	if a.Release != "" {
		c.Startf("Exporting env")

		_, r, err := common.AppManifest(rack, app)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(tmp, "env"), []byte(r.Env), 0600); err != nil {
			return err
		}

		c.OK()

		if r.Build != "" {
			c.Startf("Exporting build <build>%s</build>", r.Build)

			fd, err := os.OpenFile(filepath.Join(tmp, "build.tgz"), os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer fd.Close()

			if err := rack.BuildExport(app, r.Build, fd); err != nil {
				return err
			}

			c.OK()
		}
	}

	rs, err := rack.ResourceList(app)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(tmp, "resources"), 0700); err != nil {
		return err
	}

	for _, r := range rs {
		c.Startf("Exporting resource <resource>%s</resource>", r.Name)

		fd, err := os.OpenFile(filepath.Join(tmp, "resources", fmt.Sprintf("%s.tgz", r.Name)), os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer fd.Close()

		rr, err := rack.ResourceExport(app, r.Name)
		if err != nil {
			return err
		}

		if _, err := io.Copy(fd, rr); err != nil {
			return err
		}

		c.OK()
	}

	c.Startf("Packaging export")

	tgz, err := common.Tarball(tmp)
	if err != nil {
		return err
	}

	if _, err := w.Write(tgz); err != nil {
		return err
	}

	c.OK()

	return nil
}

func appImport(rack sdk.Interface, c *stdcli.Context, app string, r io.Reader) error {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}

	if err := common.Unarchive(gz, tmp); err != nil {
		return err
	}

	var a structs.App

	data, err := ioutil.ReadFile(filepath.Join(tmp, "app.json"))
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	c.Startf("Creating app <app>%s</app>", app)

	if _, err := rack.AppCreate(app, structs.AppCreateOptions{Generation: options.String(a.Generation)}); err != nil {
		return err
	}

	if err := common.WaitForAppRunning(rack, app); err != nil {
		return err
	}

	c.OK()

	build := filepath.Join(tmp, "build.tgz")
	env := filepath.Join(tmp, "env")
	release := ""

	if _, err := os.Stat(build); !os.IsNotExist(err) {
		fd, err := os.Open(build)
		if err != nil {
			return err
		}

		c.Startf("Importing build")

		b, err := rack.BuildImport(app, fd)
		if err != nil {
			return err
		}

		c.OK(b.Release)

		release = b.Release
	}

	if _, err := os.Stat(env); !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(env)
		if err != nil {
			return err
		}

		c.Startf("Importing env")

		r, err := rack.ReleaseCreate(app, structs.ReleaseCreateOptions{Env: options.String(string(data))})
		if err != nil {
			return err
		}

		c.OK(r.Id)

		release = r.Id
	}

	if release != "" {
		c.Startf("Promoting <release>%s</release>", release)

		if err := rack.ReleasePromote(app, release, structs.ReleasePromoteOptions{}); err != nil {
			return err
		}

		if err := common.WaitForAppRunning(rack, app); err != nil {
			return err
		}

		c.OK()
	}

	if _, err := os.Stat(filepath.Join(tmp, "resources")); !os.IsNotExist(err) {
		err = filepath.Walk(filepath.Join(tmp, "resources"), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

			c.Startf("Importing resource <resource>%s</resource>", name)

			fd, err := os.Open(path)
			if err != nil {
				return err
			}

			if err := rack.ResourceImport(app, name, fd); err != nil {
				return err
			}
			if err != nil {
				return err
			}

			return c.OK()
		})
		if err != nil {
			return err
		}
	}

	if len(a.Parameters) > 0 {
		ae, err := rack.AppGet(app)
		if err != nil {
			return err
		}

		change := false

		for k, v := range a.Parameters {
			if v != ae.Parameters[k] {
				change = true
				break
			}
		}

		if change {
			c.Startf("Updating parameters")

			if err := rack.AppUpdate(app, structs.AppUpdateOptions{Parameters: a.Parameters}); err != nil {
				return err
			}

			if err := common.WaitForAppRunning(rack, app); err != nil {
				return err
			}

			c.OK()
		}
	}

	return nil
}
