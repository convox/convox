package rack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

var (
	TestLatest string
)

type Rack interface {
	Client() (sdk.Interface, error)
	Name() string
	Parameters() (map[string]string, error)
	Provider() string
	Remote() bool
	Status() string
	Uninstall() error
	Update(map[string]string) error
}

func Current(c *stdcli.Context) (Rack, error) {
	if url := os.Getenv("RACK_URL"); strings.TrimSpace(url) != "" {
		return LoadDirect(c, url)
	}

	if name := currentRack(c); name != "" {
		return Match(c, name)
	}

	data, err := c.SettingRead("current")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(data) == "" {
		return nil, fmt.Errorf("no current rack, use `convox racks` to list and `convox switch <name>` to select")
	}

	var attrs map[string]string

	if err := json.Unmarshal([]byte(data), &attrs); err != nil {
		return nil, err
	}

	switch attrs["type"] {
	case "console":
		return LoadConsole(c, attrs["name"])
	case "terraform":
		return LoadTerraform(c, attrs["name"])
	case "test":
		return LoadTest(c, attrs["name"])
	default:
		return nil, fmt.Errorf("unknown rack type: %s", attrs["type"])
	}
}

func Install(c *stdcli.Context, name, provider string, options map[string]string) error {
	switch len(strings.Split(name, "/")) {
	case 1:
		return InstallTerraform(c, name, provider, options)
	case 2:
		return InstallConsole(c, name, provider, options)
	default:
		return fmt.Errorf("invalid name: %s", name)
	}
}

func Latest() (string, error) {
	if TestLatest != "" {
		return TestLatest, nil
	}

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

func List(c *stdcli.Context) ([]Rack, error) {
	rs := []Rack{}

	trs, err := listTerraform(c)
	if err != nil {
		return nil, err
	}

	for _, tr := range trs {
		rs = append(rs, tr)
	}

	crs, err := listConsole(c)
	if err != nil {
		return nil, err
	}

	for _, cr := range crs {
		rs = append(rs, cr)
	}

	sort.Slice(rs, func(i, j int) bool {
		switch {
		case !rs[i].Remote() && rs[j].Remote():
			return true
		case rs[i].Remote() && !rs[j].Remote():
			return false
		default:
			return rs[i].Name() < rs[j].Name()
		}
	})

	return rs, nil
}

func Match(c *stdcli.Context, name string) (Rack, error) {
	rs, err := List(c)
	if err != nil {
		return nil, err
	}

	matches := []Rack{}

	for _, r := range rs {
		if r.Name() == name {
			return r, nil
		}

		if strings.Index(r.Name(), name) != -1 {
			matches = append(matches, r)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("could not find rack: %s", name)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous rack name: %s", name)
	}
}

func Switch(c *stdcli.Context, name string) (Rack, error) {
	r, err := Match(c, name)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}

	if err := c.SettingWrite("current", string(data)); err != nil {
		return nil, err
	}

	return r, nil
}

func currentRack(c *stdcli.Context) string {
	if r := c.String("rack"); r != "" {
		return r
	}

	if r := os.Getenv("CONVOX_RACK"); r != "" {
		return r
	}

	if r := c.LocalSetting("rack"); r != "" {
		return r
	}

	if r, _ := c.SettingRead("rack"); r != "" {
		return r
	}

	return ""
}
