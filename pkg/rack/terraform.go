package rack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

type Terraform struct {
	endpoint string
	name     string
	provider string
	status   string
}

func LoadTerraform(c *stdcli.Context, name string) (*Terraform, error) {
	dir, err := c.SettingDirectory("racks")
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no such terraform rack: %s", name)
	}

	tf := filepath.Join(dir, name, "main.tf")

	if _, err := os.Stat(tf); os.IsNotExist(err) {
		return nil, fmt.Errorf("no such terraform rack: %s", name)
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer os.Chdir(wd)

	os.Chdir(filepath.Dir(tf))

	data, err := c.Execute("terraform", "output", "-json")
	if err != nil {
		return nil, err
	}

	var output map[string]struct {
		Sensitive bool
		Type      string
		Value     string
	}

	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}

	endpoint := ""
	provider := "unknown"
	status := "unknown"

	if o, ok := output["api"]; ok {
		endpoint = o.Value
		status = "running"
	}

	if _, err := os.Stat(".terraform.tfstate.lock.info"); !os.IsNotExist(err) {
		status = "updating"
	}

	if o, ok := output["provider"]; ok {
		provider = o.Value
	}

	t := &Terraform{
		endpoint: strings.TrimSpace(string(endpoint)),
		name:     name,
		provider: strings.TrimSpace(string(provider)),
		status:   status,
	}

	return t, nil
}

func (t Terraform) Client() (sdk.Interface, error) {
	return sdk.New(t.endpoint)
}

func (t Terraform) MarshalJSON() ([]byte, error) {
	h := map[string]string{
		"name": t.name,
		"type": "terraform",
	}

	return json.Marshal(h)
}

func (t Terraform) Name() string {
	return t.name
}

func (t Terraform) Provider() string {
	return t.provider
}

func (t Terraform) Remote() bool {
	return false
}

func (t Terraform) Status() string {
	return t.status
}

func listTerraform(c *stdcli.Context) ([]Terraform, error) {
	dir, err := c.SettingDirectory("racks")
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []Terraform{}, nil
	}

	subs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	ts := []Terraform{}

	for _, sub := range subs {
		if !sub.IsDir() {
			continue
		}

		t, err := LoadTerraform(c, sub.Name())
		if err != nil {
			fmt.Printf("err: %+v\n", err)
			continue
		}

		ts = append(ts, *t)
	}

	return ts, nil
}
