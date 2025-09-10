package cli

import (
	"fmt"
	"io"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("deploy", "create and promote a build", Deploy, stdcli.CommandOptions{
		Flags:    append(stdcli.OptionFlags(structs.BuildCreateOptions{}), flagApp, flagId, flagRack, flagForce),
		Usage:    "[dir]",
		Validate: stdcli.ArgsMax(1),
	}, WithCloud())
}

func Deploy(rack sdk.Interface, c *stdcli.Context) error {
	var stdout io.Writer

	if c.Bool("id") {
		stdout = c.Writer().Stdout
		c.Writer().Stdout = c.Writer().Stderr
	}

	b, err := build(rack, c, false)
	if err != nil {
		return err
	}

	if err := releasePromote(rack, c, app(c), b.Release, c.Bool("force")); err != nil {
		return err
	}

	if c.Bool("id") {
		fmt.Fprintf(stdout, b.Release)
	}

	return nil
}
