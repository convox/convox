package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

var (
	flagKey = stdcli.StringFlag("key", "", "private key file")
)

func init() {
	register("instances", "list instances", Instances, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("instances keyroll", "roll ssh key on instances", InstancesKeyroll, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.Args(0),
	})

	register("instances ssh", "run a shell on an instance", InstancesSsh, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack, flagKey},
		Validate: stdcli.ArgsMin(1),
	})

	register("instances terminate", "terminate an instance", InstancesTerminate, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.ArgsMin(1),
	})
}

func Instances(rack sdk.Interface, c *stdcli.Context) error {
	is, err := rack.InstanceList()
	if err != nil {
		return err
	}

	t := c.Table("ID", "STATUS", "STARTED", "PS", "CPU", "MEM", "PUBLIC", "PRIVATE")

	for _, i := range is {
		cpuPercent, memPercent := common.Percent(i.Cpu), common.Percent(i.Memory)
		if i.CpuAllocatable > 0 && i.MemoryAllocatable > 0 {
			cpuPercent, memPercent = common.Percent(i.Cpu/i.CpuAllocatable), common.Percent(i.Memory/i.MemoryAllocatable)
		}

		t.AddRow(i.Id, i.Status, common.Ago(i.Started), fmt.Sprintf("%d", i.Processes), cpuPercent, memPercent, i.PublicIp, i.PrivateIp)
	}

	return t.Print()
}

func InstancesKeyroll(r sdk.Interface, c *stdcli.Context) error {
	c.Startf("Rolling instance key")

	resp, err := r.InstanceKeyroll()
	if err != nil {
		return err
	}

	// only for v3 it will return resp
	if resp != nil && resp.Name != nil {
		c.Args = append(c.Args, fmt.Sprintf("key_pair_name=%s", *resp.Name))
		c.Writef("\n")
		if err := RackParamsSet(r, c); err != nil {
			return err
		}
		c.Writef("Generated private key:\n")
		c.Write([]byte(*resp.PrivateKey))
		c.Writef("\n")
		return nil
	}

	c.Writef("\n")

	if err := common.WaitForRackWithLogs(r, c); err != nil {
		return err
	}

	return c.OK()
}

func InstancesSsh(rack sdk.Interface, c *stdcli.Context) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	opts := structs.InstanceShellOptions{}

	if w, h, err := c.TerminalSize(); err == nil {
		opts.Height = options.Int(h)
		opts.Width = options.Int(w)
	}

	restore := c.TerminalRaw()
	defer restore()

	command := strings.Join(c.Args[1:], " ")

	if command != "" {
		opts.Command = options.String(command)
	}

	if key := c.String("key"); key != "" {
		data, err := os.ReadFile(key)
		if err != nil {
			return fmt.Errorf("invalid key file: %s", err)
		}
		opts.PrivateKey = options.String(base64.StdEncoding.EncodeToString(data))
	}

	if s.Version <= "20180708231844" {
		code, err := rack.InstanceShellClassic(c.Arg(0), c, opts)
		if err != nil {
			return err
		}

		return stdcli.Exit(code)
	}

	code, err := rack.InstanceShell(c.Arg(0), c, opts)
	if err != nil {
		return err
	}

	return stdcli.Exit(code)
}

func InstancesTerminate(rack sdk.Interface, c *stdcli.Context) error {
	c.Startf("Terminating instance")

	if err := rack.InstanceTerminate(c.Arg(0)); err != nil {
		return err
	}

	return c.OK()
}
