package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func app(c *stdcli.Context) string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return coalesce(c.String("app"), c.LocalSetting("app"), filepath.Base(wd))
}

func argsToOptions(args []string) map[string]interface{} {
	options := map[string]interface{}{}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		optionName := parts[0]
		if strings.Contains(parts[1], ",") {
			options[optionName] = strings.Split(parts[1], ",")
		} else {
			options[optionName] = parts[1]
		}
	}

	return options
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}

	return ""
}

func executableName() string {
	switch runtime.GOOS {
	case "windows":
		return "convox.exe"
	default:
		return "convox"
	}
}

func generateTempKey() (string, error) {
	data := make([]byte, 1024)

	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("tmp/%s", hex.EncodeToString(hash[:])[0:30]), nil
}

func hostRacks(c *stdcli.Context) map[string]string {
	data, err := c.SettingRead("switch")
	if err != nil {
		return map[string]string{}
	}

	var rs map[string]string

	if err := json.Unmarshal([]byte(data), &rs); err != nil {
		return map[string]string{}
	}

	return rs
}

func tag(name, value string) string {
	return fmt.Sprintf("<%s>%s</%s>", name, value, name)
}

func waitForResourceDeleted(rack sdk.Interface, c *stdcli.Context, resource string) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	time.Sleep(WaitDuration) // give the stack time to start updating

	return common.Wait(WaitDuration, 30*time.Minute, 2, func() (bool, error) {
		var err error
		if s.Version <= "20190111211123" {
			_, err = rack.SystemResourceGetClassic(resource)
		} else {
			_, err = rack.SystemResourceGet(resource)
		}
		if err == nil {
			return false, nil
		}
		if strings.Contains(err.Error(), "no such resource") {
			return true, nil
		}
		if strings.Contains(err.Error(), "does not exist") {
			return true, nil
		}
		return false, err
	})
}

func waitForResourceRunning(rack sdk.Interface, c *stdcli.Context, resource string) error {
	s, err := rack.SystemGet()
	if err != nil {
		return err
	}

	time.Sleep(WaitDuration) // give the stack time to start updating

	return common.Wait(WaitDuration, 30*time.Minute, 2, func() (bool, error) {
		var r *structs.Resource
		var err error

		if s.Version <= "20190111211123" {
			r, err = rack.SystemResourceGetClassic(resource)
		} else {
			r, err = rack.SystemResourceGet(resource)
		}
		if err != nil {
			return false, err
		}

		return r.Status == "running", nil
	})
}
