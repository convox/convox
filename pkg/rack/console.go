package rack

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/console"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

type Console struct {
	ctx      *stdcli.Context
	host     string
	name     string
	provider string
	status   string
}

func CreateConsole(c *stdcli.Context, name string, md *Metadata) (*Console, error) {
	host, err := currentConsole(c)
	if err != nil {
		return nil, err
	}

	cc, err := consoleClient(c, host, "")
	if err != nil {
		return nil, err
	}

	r, err := cc.RackCreate(name, md.Provider, md.State, md.Vars)
	if err != nil {
		return nil, err
	}

	cr := &Console{ctx: c, name: name, provider: r.Provider, status: r.Status}

	return cr, nil
}

func InstallConsole(c *stdcli.Context, provider, name, version, runtime string, options map[string]string) error {
	host, err := currentConsole(c)
	if err != nil {
		return err
	}

	cc, err := consoleClient(c, host, "")
	if err != nil {
		return err
	}

	if options["region"] == "" {
		return fmt.Errorf("region not provided")
	}

	_, err = cc.RackInstall(name, provider, version, runtime, options)
	if err != nil {
		return err
	}

	return nil
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

func CurrentConsoleClientWithMachine(c *stdcli.Context, machine string) (*console.Client, error) {
	host, err := currentConsole(c)
	if err != nil {
		return nil, err
	}

	cc, err := consoleClient(c, host, "")
	if err != nil {
		return nil, err
	}

	mList := structs.Machines{}

	mstr, _ := c.SettingRead("machines")
	if mstr == "" || json.Unmarshal([]byte(mstr), &mList) != nil {
		mList, err = cc.Machines()
		if err != nil {
			return nil, err
		}
	}

	mid, err := findMachineId(mList, machine)
	if err != nil {
		// try to refresh the machine list from the server
		mList, err = cc.Machines()
		if err != nil {
			return nil, err
		}
		mid, err = findMachineId(mList, machine)
		if err != nil {
			return nil, err
		}
	}
	if mid == "" {
		return nil, fmt.Errorf("no machine found matching '%s'", machine)
	}
	cc.SetMachine(mid)

	mBytes, err := json.Marshal(mList)
	if err != nil {
		return nil, err
	}

	c.SettingWrite("machines", string(mBytes))

	return cc, nil
}

func findMachineId(mList structs.Machines, machine string) (string, error) {
	foundMatch := false
	mid := ""
	for _, m := range mList {
		orgMachine := fmt.Sprintf("%s/%s", m.OrganizationInfo["name"], m.Name)
		if strings.Contains(orgMachine, machine) {
			if foundMatch {
				return "", fmt.Errorf("multiple machines match for '%s', please be more specific (e.g. <org-name>/<machine-name>)", machine)
			}
			mid = m.ID
			foundMatch = true
			if orgMachine == machine || m.Name == machine {
				return mid, nil
			}
		}
	}

	if !foundMatch {
		return "", fmt.Errorf("no machine found matching '%s'", machine)
	}

	return mid, nil
}

func (c Console) Client() (sdk.Interface, error) {
	return c.client()
}

func (c Console) Delete() error {
	cc, err := c.client()
	if err != nil {
		return err
	}

	if err := cc.RackDelete(c.name); err != nil {
		return err
	}

	return nil
}

func (c Console) Endpoint() (*url.URL, error) {
	pw, err := currentPassword(c.ctx, c.host)
	if err != nil {
		return nil, err
	}

	username := base64.StdEncoding.EncodeToString([]byte(c.name))

	endpoint := fmt.Sprintf("https://%s:%s@%s", string(username), url.QueryEscape(pw), c.host)
	if os.Getenv("X_DEV_ALLOW_HTTP") == "true" {
		fmt.Println("waring: using http inscure mode")
		endpoint = fmt.Sprintf("http://%s:%s@%s", string(username), url.QueryEscape(pw), c.host)
	}

	return url.Parse(endpoint)
}

func (c Console) MarshalJSON() ([]byte, error) {
	h := map[string]string{
		"name": c.name,
		"type": "console",
	}

	return json.Marshal(h)
}

func (c Console) Metadata() (*Metadata, error) {
	cc, err := c.client()
	if err != nil {
		return nil, err
	}

	r, err := cc.RackGet(c.name)
	if err != nil {
		return nil, err
	}

	m := &Metadata{
		Deletable: r.Deletable,
		Provider:  r.Provider,
		State:     r.State,
		Vars:      r.Parameters,
	}

	return m, nil
}

func (c Console) Name() string {
	return c.name
}

func (c Console) Parameters() (map[string]string, error) {
	cc, err := c.client()
	if err != nil {
		return nil, err
	}

	r, err := cc.RackGet(c.name)
	if err != nil {
		return nil, err
	}

	if r.Parameters != nil {
		return r.Parameters, nil
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

func (c Console) Sync() error {
	cc, err := c.client()
	if err != nil {
		return err
	}
	return cc.RackSync(c.name)
}

func (c Console) Uninstall() error {
	return fmt.Errorf("console uninstall not yet supported")
}

func (c Console) UpdateParams(params map[string]string) error {
	cu, err := c.consoleUpdateSupported()
	if err != nil {
		return err
	}
	if !cu {
		return c.updateParamsDirect(params)
	}

	cc, err := c.client()
	if err != nil {
		return err
	}

	r, err := cc.RackGet(c.name)
	if err != nil {
		return err
	}

	if r.Version == "" {
		return fmt.Errorf("current version invalid")
	}

	if err := cc.RackUpdate(c.name, r.Version, false, params); err != nil {
		return err
	}

	return nil
}

func (c Console) updateParamsDirect(params map[string]string) error {
	d, err := c.direct()
	if err != nil {
		return err
	}

	if err := d.UpdateParams(params); err != nil {
		return err
	}

	return nil
}

func (c Console) UpdateVersion(version string, force bool) error {
	cu, err := c.consoleUpdateSupported()
	if err != nil {
		return err
	}
	if !cu {
		return c.updateVersionDirect(version, force)
	}

	cc, err := c.client()
	if err != nil {
		return err
	}

	r, err := cc.RackGet(c.name)
	if err != nil {
		return err
	}
	currentVersion := r.Version

	if version == "" {
		v, err := terraformLatestVersion(currentVersion)
		if err != nil {
			return err
		}
		version = v
	}

	if !force {
		if err := isSkippingMinor(currentVersion, version); err != nil {
			return err
		}
	}

	if err := cc.RackUpdate(c.name, version, force, nil); err != nil {
		return err
	}

	return nil
}

func isSkippingMinor(currentVersion, version string) error {
	splittedCurrent := strings.Split(currentVersion, ".")
	if len(splittedCurrent) <= 1 {
		return nil
	}
	cmv, err := strconv.Atoi(splittedCurrent[1])
	if err != nil {
		return err
	}

	splittedVersion := strings.Split(version, ".")
	if len(splittedVersion) <= 1 {
		return nil
	}
	mv, err := strconv.Atoi(splittedVersion[1])
	if err != nil {
		return err
	}

	if mv > (cmv + 1) {
		return fmt.Errorf("you can't skip a minor update, please update to the latest 3.%d.XX and so on before updating to 3.%d.XX", (cmv + 1), mv)
	}

	return nil
}

func (c Console) updateVersionDirect(version string, force bool) error {
	d, err := c.direct()
	if err != nil {
		return err
	}

	if err := d.UpdateVersion(version, force); err != nil {
		return err
	}

	return nil
}

func (c Console) client() (*console.Client, error) {
	cc, err := consoleClient(c.ctx, c.host, c.name)
	if err != nil {
		return nil, err
	}

	return cc, nil
}

func (c Console) direct() (*Direct, error) {
	cc, err := c.client()
	if err != nil {
		return nil, err
	}

	d, err := LoadDirect(cc)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (c Console) consoleUpdateSupported() (bool, error) {
	cc, err := c.client()
	if err != nil {
		return false, err
	}

	r, err := cc.RackGet(c.name)
	if err != nil {
		return false, err
	}

	if r.State == nil {
		return false, nil
	}

	return true, nil
}

func consoleClient(c *stdcli.Context, host, rack string) (*console.Client, error) {
	pw, err := currentPassword(c, host)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("https://convox:%s@%s", url.QueryEscape(pw), host)
	if os.Getenv("X_DEV_ALLOW_HTTP") == "true" {
		fmt.Println("waring: using http inscure mode")
		endpoint = fmt.Sprintf("http://convox:%s@%s", url.QueryEscape(pw), host)
	}

	cc, err := console.NewClient(endpoint, rack, c)
	if err != nil {
		return nil, err
	}

	return cc, nil
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

	cc, err := consoleClient(c, host, "")
	if err != nil {
		return nil, err
	}

	rs, err := cc.RackList()
	if err != nil && strings.Contains(err.Error(), "cli token is expired") {
		return nil, err
	}

	switch err.(type) {
	case console.AuthenticationError:
		return nil, err
	case nil:
	default:
		return []Console{}, nil
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
