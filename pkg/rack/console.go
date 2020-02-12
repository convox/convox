package rack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/console"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/convox/stdsdk"
)

type Console struct {
	ctx      *stdcli.Context
	host     string
	name     string
	provider string
	status   string
}

func InstallConsole(c *stdcli.Context, name, provider string, options map[string]string) error {
	return fmt.Errorf("console install not yet supported")
}

func LoadConsole(c *stdcli.Context, name string) (*Console, error) {
	rs, err := listConsole(c)
	if err != nil {
		return nil, err
	}

	for _, r := range rs {
		if r.Name() == name {
			return &r, nil
		}
	}

	return nil, fmt.Errorf("no such console rack: %s", name)
}

func (c Console) Client() (sdk.Interface, error) {
	cc, err := consoleClient(c.ctx, c.host)
	if err != nil {
		return nil, err
	}

	cc.Rack = c.name

	return cc, nil
}

func (c Console) MarshalJSON() ([]byte, error) {
	h := map[string]string{
		"name": c.name,
		"type": "console",
	}

	return json.Marshal(h)
}

func (c Console) Name() string {
	return c.name
}

func (c Console) Parameters() (map[string]string, error) {
	cc, err := c.Client()
	if err != nil {
		return nil, err
	}

	s, err := cc.SystemGet()
	if err != nil {
		return nil, err
	}

	return s.Parameters, nil
}

func (c Console) Provider() string {
	return c.provider
}

func (c Console) Remote() bool {
	return true
}

func (c Console) Status() string {
	return c.status
}

func (c Console) Uninstall() error {
	return fmt.Errorf("console uninstall not yet supported")
}

func (c Console) Update(opts map[string]string) error {
	uopts := structs.SystemUpdateOptions{}

	if v, ok := opts["release"]; ok {
		uopts.Version = options.String(v)
	}

	delete(opts, "release")

	if len(opts) > 0 {
		uopts.Parameters = opts
	}

	cc, err := c.Client()
	if err != nil {
		return err
	}

	if err := cc.SystemUpdate(uopts); err != nil {
		return err
	}

	return nil
}

func consoleClient(c *stdcli.Context, host string) (*sdk.Client, error) {
	pw, err := currentPassword(c, host)
	if err != nil {
		return nil, err
	}

	remote := fmt.Sprintf("https://convox:%s@%s", url.QueryEscape(pw), host)

	s, err := sdk.New(remote)
	if err != nil {
		return nil, err
	}

	s.Authenticator = console.Authenticator(c)
	s.Session = console.Session(c)

	return s, nil
}

func currentConsole(c *stdcli.Context) (string, error) {
	if h := os.Getenv("CONVOX_HOST"); h != "" {
		return h, nil
	}

	if h, _ := c.SettingRead("console"); h != "" {
		return h, nil
	}

	return "", nil
}

func currentPassword(c *stdcli.Context, host string) (string, error) {
	if pw := os.Getenv("CONVOX_PASSWORD"); pw != "" {
		return pw, nil
	}

	return c.SettingReadKey("auth", host)
}

func listConsole(c *stdcli.Context) ([]Console, error) {
	cs := []Console{}

	host, err := currentConsole(c)
	if err != nil {
		return nil, err
	}
	if host == "" {
		return []Console{}, nil
	}

	p, err := consoleClient(c, host)
	if err != nil {
		return nil, err
	}

	var rs []struct {
		Name         string
		Organization struct {
			Name string
		}
		Provider string
		Status   string
	}

	if err := p.Get("/racks", stdsdk.RequestOptions{}, &rs); err != nil {
		if _, ok := err.(console.AuthenticationError); ok {
			return nil, err
		}
	}

	for _, r := range rs {
		cs = append(cs, Console{
			ctx:      c,
			host:     host,
			name:     fmt.Sprintf("%s/%s", r.Organization.Name, r.Name),
			provider: common.CoalesceString(r.Provider, "unknown"),
			status:   r.Status,
		})
	}

	return cs, nil
}
