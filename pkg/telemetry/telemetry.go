package telemetry

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/convox/stdcli"
)

func Track(c *stdcli.Context, params map[string]string) error {
	if saasCustomer(c) {
		return post(c, params)
	}

	if selfHostCustomer(c) && isTelemetrySet(c) {
		return post(c, params)
	}

	return nil
}

func post(c *stdcli.Context, params map[string]string) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	res, err := http.Post("https://telemetry.convox.com/telemetry", "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer res.Body.Close()

	return nil
}

func saasCustomer(c *stdcli.Context) bool {
	host, err := c.SettingRead("host")
	if err != nil {
		return false
	}

	return host == "console.convox.com"
}

func selfHostCustomer(c *stdcli.Context) bool {
	host, err := c.SettingRead("host")
	if err != nil {
		return false
	}

	return host != "console.convox.com"
}

func isTelemetrySet(c *stdcli.Context) bool {
	telemetry, err := c.SettingRead("telemetry")
	if err != nil {
		return true
	}

	return telemetry == "true"
}
