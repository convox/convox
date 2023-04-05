package cli

import (
	"fmt"
	"strconv"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("telemetry set", "Activate/Desactivate Telemetry", TelemetrySet, stdcli.CommandOptions{
		Validate: stdcli.ArgsMax(1),
	})
}

func TelemetrySet(rack sdk.Interface, c *stdcli.Context) error {
	activate := c.Arg(0)
	_, err := strconv.ParseBool(activate)
	if err != nil {
		return fmt.Errorf("command accepts just 'true' or 'false' as argument")
	}

	if err := c.SettingWrite("telemetry", activate); err != nil {
		return err
	}

	return c.OK()
}
