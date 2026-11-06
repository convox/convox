package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("login", "authenticate with a rack", Login, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			stdcli.StringFlag("token", "t", "cli token"),
		},
		Usage:    "[hostname]",
		Validate: stdcli.ArgsMax(1),
	})
}

func Login(rack sdk.Interface, c *stdcli.Context) error {
	hostname := coalesce(c.Arg(0), "console.convox.com")

	auth, err := c.SettingReadKey("auth", hostname)
	if err != nil {
		return err
	}

	password := coalesce(c.String("token"), os.Getenv("CONVOX_TOKEN"), auth)

	if password == "" {
		c.Writef("CLI Token: ")

		password, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	c.Startf("Authenticating with <info>%s</info>", hostname)

	hostUrl := fmt.Sprintf("https://convox:%s@%s", url.QueryEscape(password), hostname)
	if os.Getenv("X_DEV_ALLOW_HTTP") == "true" {
		fmt.Println("waring: using http inscure mode")
		hostUrl = fmt.Sprintf("http://convox:%s@%s", url.QueryEscape(password), hostname)
	}
	cl, err := sdk.New(hostUrl)
	if err != nil {
		return err
	}

	id, err := cl.Auth()
	if err != nil {
		if strings.Contains(err.Error(), "cli token is expired") {
			return err
		}
		return fmt.Errorf("invalid login")
	}

	if err := c.SettingWriteKey("auth", hostname, password); err != nil {
		return err
	}

	if err := c.SettingWrite("console", hostname); err != nil {
		return err
	}

	if id != "" {
		if err := c.SettingWrite("id", id); err != nil {
			return err
		}
	}

	if err := c.SettingDelete("rack"); err != nil {
		return err
	}

	return c.OK()
}
