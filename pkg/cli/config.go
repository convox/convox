package cli

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("configs", "list of app configs", watch(Configs), stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp, flagWatchInterval},
		Validate: stdcli.Args(0),
	})

	register("config get", "get the config", ConfigGet, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagApp},
		Usage:    "<name>",
		Validate: stdcli.Args(1),
	})

	register("config set", "set the config", ConfigSet, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			flagApp,
			flagRack,
			stdcli.StringFlag("file", "f", "config file"),
			stdcli.StringFlag("value", "", "config value"),
			stdcli.BoolFlag("restart", "", "restart the app"),
		},
		Usage:    "<name>",
		Validate: stdcli.Args(1),
	})
}

func Configs(rack sdk.Interface, c *stdcli.Context) error {
	cfgs, err := rack.AppConfigList(app(c))
	if err != nil {
		return err
	}

	t := c.Table("CONFIG-ID")

	for _, a := range cfgs {
		t.AddRow(a.Name)
	}

	return t.Print()
}

func ConfigGet(rack sdk.Interface, c *stdcli.Context) error {
	cfg, err := rack.AppConfigGet(app(c), c.Arg(0))
	if err != nil {
		return err
	}

	return c.Writef("%s\n", cfg.Value)
}

func ConfigSet(rack sdk.Interface, c *stdcli.Context) error {
	f, isFile := c.Value("file").(string)
	value, isValue := c.Value("value").(string)
	if !isFile && !isValue {
		return fmt.Errorf("--file or --value flag is required")
	}
	var err error
	data := []byte(value)
	if isFile {
		data, err = os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read the file: %s", err)
		}
	}

	err = rack.AppConfigSet(app(c), c.Arg(0), base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return err
	}
	c.OK()

	if c.Bool("restart") {
		return Restart(rack, c)
	}

	return nil
}
