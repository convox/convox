package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/inconshreveable/go-update"
)

func init() {
	registerWithoutProvider("update", "update the cli", Update, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{flagRack},
		Validate: stdcli.ArgsMax(1),
	})
}

func Update(rack sdk.Interface, c *stdcli.Context) error {
	binary, err := releaseBinary()
	if err != nil {
		return err
	}

	version := c.Arg(0)
	current := c.Version()

	if version == "" {
		v, err := latestRelease()
		if err != nil {
			return fmt.Errorf("could not fetch latest release: %s", err)
		}
		version = v
	}

	if (version == current) {
		c.Writef("Already on requested version <release>%s</release>\n", version)
		return nil
	}

	asset := fmt.Sprintf("https://github.com/convox/convox/releases/download/%s/%s", version, binary)

	res, err := http.Get(asset)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("invalid version")
	}

	defer res.Body.Close()

	c.Startf("Updating to <release>%s</release>", version)

	if err := update.Apply(res.Body, update.Options{}); err != nil {
		return err
	}

	return c.OK()
}

func releaseBinary() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return "convox-macos", nil
	case "linux":
		return "convox-linux", nil
	default:
		return "", fmt.Errorf("unknown platform: %s", runtime.GOOS)
	}
}

func latestRelease() (string, error) {
	res, err := http.Get("https://api.github.com/repos/convox/convox/releases/latest")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var release struct {
		Tag string `json:"tag_name"`
	}

	if err := json.Unmarshal(data, &release); err != nil {
		return "", err
	}

	return release.Tag, nil
}
