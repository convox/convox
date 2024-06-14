package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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

func argsToOptions(args []string) map[string]string {
	options := map[string]string{}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		options[parts[0]] = parts[1]
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

func clearScreen() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls") //Windows example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		cmd := exec.Command("clear") //Linux example, its tested
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func watch(fn func(r sdk.Interface, c *stdcli.Context) error) func(sdk.Interface, *stdcli.Context) error {
	return func(rr sdk.Interface, cc *stdcli.Context) error {
		watchIntervalStr := cc.String("watch")
		watchInterval, _ := strconv.Atoi(watchIntervalStr)
		if watchInterval <= 0 {
			return fn(rr, cc)
		}

		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		ticker := time.NewTicker(time.Second * time.Duration(watchInterval))
		for {
			select {
			case <-ch:
				ticker.Stop()
				return nil
			case <-ticker.C:
				clearScreen()
				cc.Writef("Every %ds:\n\n", watchInterval)
				err := fn(rr, cc)
				if err != nil {
					cc.Error(err)
				}
			}
		}
		return nil
	}
}

func checkRackNameRegex(name string) error {
	if !regexp.MustCompile(`^[a-z0-9/-]+$`).MatchString(name) {
		return fmt.Errorf("only lowercase alphanumeric characters and hyphen allowed and must not start with hyphen")
	}
	return nil
}
