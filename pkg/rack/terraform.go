package rack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

type Terraform struct {
	ctx      *stdcli.Context
	endpoint string
	name     string
	provider string
	status   string
}

func InstallTerraform(c *stdcli.Context, provider, name string, options map[string]string) error {
	if !terraformInstalled() {
		return fmt.Errorf("terraform required")
	}

	env, err := terraformEnv(provider)
	if err != nil {
		return err
	}

	dir, err := c.SettingDirectory(fmt.Sprintf("racks/%s", name))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	vars, err := terraformProviderVars(provider)
	if err != nil {
		return err
	}

	ov, err := terraformOptionVars(dir, options)
	if err != nil {
		return err
	}

	for k, v := range ov {
		vars[k] = v
	}

	tf := filepath.Join(dir, "main.tf")

	if _, err := os.Stat(tf); !os.IsNotExist(err) {
		return fmt.Errorf("rack name in use: %s", name)
	}

	params := map[string]interface{}{
		"Name":     name,
		"Provider": provider,
		"Vars":     vars,
	}

	if err := terraformWriteTemplate(tf, params); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "init"); err != nil {
		return err
	}

	if err := terraform(c, dir, env, "apply", "-auto-approve"); err != nil {
		return err
	}

	if _, err := Switch(c, name); err != nil {
		return err
	}

	return nil
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
		ctx:      c,
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

func (t Terraform) Parameters() (map[string]string, error) {
	dir, err := t.ctx.SettingDirectory(fmt.Sprintf("racks/%s", t.name))
	if err != nil {
		return nil, err
	}

	vars, err := terraformOptionVars(dir, map[string]string{})
	if err != nil {
		return nil, err
	}

	return vars, nil
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

func (t Terraform) Uninstall() error {
	env, err := terraformEnv(t.provider)
	if err != nil {
		return err
	}

	dir, err := t.ctx.SettingDirectory(fmt.Sprintf("racks/%s", t.name))
	if err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "init", "-upgrade"); err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "destroy", "-auto-approve"); err != nil {
		return err
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	return nil
}

func (t Terraform) Update(options map[string]string) error {
	dir, err := t.ctx.SettingDirectory(fmt.Sprintf("racks/%s", t.name))
	if err != nil {
		return err
	}

	env, err := terraformEnv(t.provider)
	if err != nil {
		return err
	}

	vars, err := terraformProviderVars(t.provider)
	if err != nil {
		return err
	}

	ov, err := terraformOptionVars(dir, options)
	if err != nil {
		return err
	}

	for k, v := range ov {
		vars[k] = v
	}

	tf := filepath.Join(dir, "main.tf")

	params := map[string]interface{}{
		"Name":     t.name,
		"Provider": t.provider,
		"Vars":     vars,
	}

	if err := terraformWriteTemplate(tf, params); err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "init", "-upgrade"); err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "apply", "-auto-approve"); err != nil {
		return err
	}

	return nil
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

func requireEnv(vars ...string) (map[string]string, error) {
	env := map[string]string{}
	missing := []string{}

	for _, k := range vars {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		} else {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("required env: %s", strings.Join(missing, ", "))
	}

	return env, nil
}

func terraform(c *stdcli.Context, dir string, env map[string]string, args ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(wd)

	if err := os.Chdir(dir); err != nil {
		return err
	}

	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	if err := c.Terminal("terraform", args...); err != nil {
		return err
	}

	return nil
}

func terraformEnv(provider string) (map[string]string, error) {
	switch provider {
	case "aws":
		return requireEnv("AWS_DEFAULT_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY")
	case "azure":
		return requireEnv("ARM_CLIENT_ID", "ARM_CLIENT_SECRET", "ARM_SUBSCRIPTION_ID", "ARM_TENANT_ID")
	case "gcp":
		return requireEnv("GOOGLE_CREDENTIALS", "GOOGLE_PROJECT", "GOOGLE_REGION")
	default:
		return map[string]string{}, nil
	}
}

func terraformInstalled() bool {
	return exec.Command("terraform", "version").Run() == nil
}

func terraformOptionVars(dir string, options map[string]string) (map[string]string, error) {
	vars := map[string]string{}

	vf := filepath.Join(dir, "vars.json")

	if _, err := os.Stat(vf); !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(vf)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, &vars); err != nil {
			return nil, err
		}
	}

	for k, v := range options {
		if strings.TrimSpace(v) != "" {
			vars[k] = v
		} else {
			delete(vars, k)
		}
	}

	data, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(vf, data, 0600); err != nil {
		return nil, err
	}

	return vars, nil
}

func terraformProviderVars(provider string) (map[string]string, error) {
	switch provider {
	case "do":
		env, err := requireEnv("DIGITALOCEAN_ACCESS_ID", "DIGITALOCEAN_SECRET_KEY", "DIGITALOCEAN_TOKEN")
		if err != nil {
			return nil, err
		}
		vars := map[string]string{
			"access_id":  env["DIGITALOCEAN_ACCESS_ID"],
			"secret_key": env["DIGITALOCEAN_SECRET_KEY"],
			"release":    "",
			"token":      env["DIGITALOCEAN_TOKEN"],
		}
		return vars, nil
	default:
		vars := map[string]string{
			"release": "",
		}
		return vars, nil
	}
}

func terraformTemplateHelpers() template.FuncMap {
	return template.FuncMap{
		"keys": func(h map[string]string) []string {
			ks := []string{}
			for k := range h {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			return ks
		},
	}
}

func terraformWriteTemplate(filename string, params map[string]interface{}) error {
	if source := os.Getenv("CONVOX_TERRAFORM_SOURCE"); source != "" {
		params["Source"] = fmt.Sprintf(source, params["Provider"])
	} else {
		params["Source"] = fmt.Sprintf("github.com/convox/convox//terraform/system/%s", params["Provider"])
	}

	t, err := template.New("main").Funcs(terraformTemplateHelpers()).Parse(`
		module "system" {
			source = "{{.Source}}"

			name = "{{.Name}}"

			{{- range (keys .Vars) }}
			{{.}} = "{{index $.Vars .}}"
			{{- end }}
		}

		output "api" {
			value = module.system.api
		}

		output "provider" {
			value = "{{.Provider}}"
		}`,
	)
	if err != nil {
		return err
	}

	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	if err := t.Execute(fd, params); err != nil {
		return err
	}

	return nil
}
