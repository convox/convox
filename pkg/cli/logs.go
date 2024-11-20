package cli

import (
	"io"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("logs", "get logs for an app", Logs, stdcli.CommandOptions{
		Flags: append(stdcli.OptionFlags(structs.LogsOptions{}), flagApp, flagNoFollow, flagRack,
			stdcli.StringFlag("service", "s", "service name"),
		),
		Validate: stdcli.Args(0),
	})
}

func Logs(rack sdk.Interface, c *stdcli.Context) error {
	var opts structs.LogsOptions

	if err := c.Options(&opts); err != nil {
		return err
	}

	if c.Bool("no-follow") {
		opts.Follow = options.Bool(false)
	}

	opts.Prefix = options.Bool(true)

	var r io.ReadCloser
	var err error
	if c.String("service") != "" {
		r, err = rack.ServiceLogs(app(c), c.String("service"), opts)
		if err != nil {
			return err
		}
	} else {
		r, err = rack.AppLogs(app(c), opts)
		if err != nil {
			return err
		}
	}

	_, err = io.Copy(c, r)

	return nil
}
