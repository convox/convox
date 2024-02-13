package cli

import (
	"encoding/json"
	"fmt"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("workflows", "get list of workflows", Workflows, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("workflows run", "run workflow for specified branch or commit", WorkflowCustomRun, stdcli.CommandOptions{
		Flags:    stdcli.OptionFlags(structs.WorkflowCustomRunOptions{}),
		Usage:    "<id>",
		Validate: stdcli.Args(1),
	})

}

func Workflows(rack sdk.Interface, c *stdcli.Context) error {
	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	ws, err := rack.WorkflowList(attrs["name"])
	if err != nil {
		return err
	}

	t := c.Table("ID", "KIND", "NAME")
	for _, r := range ws.Workflows {
		t.AddRow(r.Id, r.Kind, r.Name)
	}

	return t.Print()
}

func WorkflowCustomRun(rack sdk.Interface, c *stdcli.Context) error {
	wid := c.Arg(0)

	var opts structs.WorkflowCustomRunOptions
	if err := c.Options(&opts); err != nil {
		return err
	}

	if opts.App == nil || *opts.App == "" {
		return fmt.Errorf("app is required")
	}

	if opts.Branch == nil && opts.Commit == nil {
		return fmt.Errorf("branch or commit is required")
	}

	data, err := c.SettingRead("current")
	if err != nil {
		return err
	}
	var attrs map[string]string
	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return err
	}

	resp, err := rack.WorkflowCustomRun(attrs["name"], wid, opts)
	if err != nil {
		return err
	}

	return c.Writef("Successfully trigger the workflow, job id: %s", resp.JobID)
}
