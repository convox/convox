package cli

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdcli"
)

func init() {
	register("apps", "list apps", Apps, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("apps create", "create an app", AppsCreate, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.AppCreateOptions{}), flagRack, flagWait),
		Usage:    "[name]",
		Validate: stdcli.ArgsMax(1),
	})

	register("apps delete", "delete an app", AppsDelete, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagWait},
		Usage:    "<app>",
		Validate: stdcli.Args(1),
	})
}

func Apps(p structs.Provider, c *stdcli.Context) error {
	as, err := p.AppList()
	if err != nil {
		return err
	}

	t := c.Table("APP", "STATUS", "RELEASE")

	for _, a := range as {
		t.AddRow(a.Name, a.Status, a.Release)
	}

	return t.Print()
}

func AppsCreate(p structs.Provider, c *stdcli.Context) error {
	app := common.CoalesceString(c.Arg(0), app(c))

	if strings.TrimSpace(app) == "" {
		return fmt.Errorf("must specify an app name")
	}

	var opts structs.AppCreateOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	c.Startf("Creating <app>%s</app>", app)

	if _, err := p.AppCreate(app, opts); err != nil {
		return err
	}

	if c.Bool("wait") {
		if err := common.WaitForAppRunning(p, app); err != nil {
			return err
		}
	}

	return c.OK()
}

func AppsDelete(p structs.Provider, c *stdcli.Context) error {
	app := c.Args[0]

	c.Startf("Deleting <app>%s</app>", app)

	if err := p.AppDelete(app); err != nil {
		return err
	}

	if c.Bool("wait") {
		if err := common.WaitForAppDeleted(p, c, app); err != nil {
			return err
		}
	}

	return c.OK()
}
