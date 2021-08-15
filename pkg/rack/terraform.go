package rack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/convox/convox/pkg/common"
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

func CreateTerraform(c *stdcli.Context, name string, md *Metadata) (*Terraform, error) {
	if !terraformInstalled(c) {
		return nil, fmt.Errorf("terraform required")
	}

	t := &Terraform{ctx: c, name: name, provider: md.Provider}

	if err := t.create(md.Vars["release"], md.Vars, md.State); err != nil {
		t.Delete()
		return nil, err
	}

	if err := t.init(); err != nil {
		t.Delete()
		return nil, err
	}

	return t, nil
}

func InstallTerraform(c *stdcli.Context, provider, name, version string, options map[string]string) error {
	if !terraformInstalled(c) {
		return fmt.Errorf("terraform required")
	}

	if _, err := terraformEnv(provider); err != nil {
		return err
	}

	t := Terraform{ctx: c, name: name, provider: provider}

	if err := t.create(version, options, nil); err != nil {
		return err
	}

	if err := t.init(); err != nil {
		return err
	}

	if err := t.apply(); err != nil {
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

func (t Terraform) Delete() error {
	dir, err := t.settingsDirectory()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	return nil
}

func (t Terraform) Endpoint() (*url.URL, error) {
	return url.Parse(t.endpoint)
}

func (t Terraform) Latest() (string, error) {
	return terraformLatestVersion()
}

func (t Terraform) Metadata() (*Metadata, error) {
	dir, err := t.settingsDirectory()
	if err != nil {
		return nil, err
	}

	state, err := ioutil.ReadFile(filepath.Join(dir, "terraform.tfstate"))
	if err != nil {
		return nil, err
	}

	vars, err := t.vars()
	if err != nil {
		return nil, err
	}

	vars["name"] = common.CoalesceString(vars["name"], t.name)

	m := &Metadata{
		Deletable: true,
		Provider:  t.provider,
		State:     state,
		Vars:      vars,
	}

	return m, nil
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
	vars, err := t.vars()
	if err != nil {
		return nil, err
	}

	delete(vars, "name")
	delete(vars, "region")
	delete(vars, "release")

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

	if err := terraform(t.ctx, dir, env, "init", "-no-color", "-upgrade"); err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "destroy", "-auto-approve", "-no-color"); err != nil {
		return err
	}

	if err := t.Delete(); err != nil {
		return err
	}

	return nil
}

func (t Terraform) UpdateParams(params map[string]string) error {
	vars, err := t.vars()
	if err != nil {
		return err
	}

	release, ok := vars["release"]
	if !ok {
		return fmt.Errorf("could not determine current release")
	}

	for k, v := range params {
		vars[k] = v
	}

	if err := t.update(release, vars); err != nil {
		return err
	}

	if err := t.init(); err != nil {
		return err
	}

	if err := t.apply(); err != nil {
		return err
	}

	return nil
}

func (t Terraform) UpdateVersion(version string) error {
	vars, err := t.vars()
	if err != nil {
		return err
	}

	if err := t.update(version, vars); err != nil {
		return err
	}

	if err := t.init(); err != nil {
		return err
	}

	if err := t.apply(); err != nil {
		return err
	}

	return nil
}

func (t Terraform) apply() error {
	dir, err := t.settingsDirectory()
	if err != nil {
		return err
	}

	env, err := terraformEnv(t.provider)
	if err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, env, "apply", "-auto-approve", "-no-color"); err != nil {
		return err
	}

	return nil
}

func (t Terraform) create(release string, vars map[string]string, state []byte) error {
	dir, err := t.settingsDirectory()
	if err != nil {
		return err
	}

	if _, err := terraformEnv(t.provider); err != nil {
		return err
	}

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		return fmt.Errorf("rack name in use: %s", t.name)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := t.update(release, vars); err != nil {
		return err
	}

	if state != nil {
		if err := ioutil.WriteFile(filepath.Join(dir, "terraform.tfstate"), state, 0644); err != nil {
			return err
		}
	}

	return nil
}

func (t Terraform) init() error {
	dir, err := t.settingsDirectory()
	if err != nil {
		return err
	}

	if err := terraform(t.ctx, dir, nil, "init", "-force-copy", "-no-color", "-upgrade"); err != nil {
		return err
	}

	return nil
}

func (t Terraform) settingsDirectory() (string, error) {
	return t.ctx.SettingDirectory(fmt.Sprintf("racks/%s", t.name))
}

func (t Terraform) update(release string, vars map[string]string) error {
	if release == "" {
		v, err := terraformLatestVersion()
		if err != nil {
			return err
		}
		release = v

	}

	if vars == nil {
		vars = map[string]string{}
	}

	vars["name"] = common.CoalesceString(vars["name"], t.name)
	vars["release"] = release

	pv, err := terraformProviderVars(t.provider)
	if err != nil {
		return err
	}

	if err := t.writeVars(vars); err != nil {
		return err
	}

	for k, v := range pv {
		if _, ok := vars[k]; !ok {
			vars[k] = v
		}
	}

	dir, err := t.settingsDirectory()
	if err != nil {
		return err
	}

	tf := filepath.Join(dir, "main.tf")

	params := map[string]interface{}{
		"Name":     t.name,
		"Provider": t.provider,
		"Vars":     vars,
	}

	if err := terraformWriteTemplate(tf, release, params); err != nil {
		return err
	}

	if backend := os.Getenv("CONVOX_TERRAFORM_BACKEND"); backend != "" {
		if err := terraformWriteBackend(filepath.Join(dir, "backend.tf"), backend); err != nil {
			return err
		}
	}

	return nil
}

func (t Terraform) vars() (map[string]string, error) {
	vars := map[string]string{}

	vf, err := t.varsFile()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(vf); !os.IsNotExist(err) {
		data, err := ioutil.ReadFile(vf)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(data, &vars); err != nil {
			return nil, err
		}
	}

	return vars, nil
}

func (t Terraform) varsFile() (string, error) {
	dir, err := t.settingsDirectory()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("error loading rack: %s", t.name)
	}

	vf := filepath.Join(dir, "vars.json")

	return vf, nil
}

func (t Terraform) writeVars(vars map[string]string) error {
	for k, v := range vars {
		if strings.TrimSpace(v) == "" {
			delete(vars, k)
		}
	}

	data, err := json.MarshalIndent(vars, "", "  ")
	if err != nil {
		return err
	}

	vf, err := t.varsFile()
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(vf, data, 0600); err != nil {
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

func optionalEnv(vars ...string) map[string]string {
	env := map[string]string{}

	for _, k := range vars {
		if v := os.Getenv(k); v != "" {
			env[k] = v
		}
	}

	return env
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
		env, err := requireEnv("AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY")
		if err != nil {
			return nil, err
		}
		for k, v := range optionalEnv("AWS_SESSION_TOKEN") {
			env[k] = v
		}
		return env, nil
	case "azure":
		return requireEnv("ARM_CLIENT_ID", "ARM_CLIENT_SECRET", "ARM_SUBSCRIPTION_ID", "ARM_TENANT_ID")
	case "do":
		return requireEnv("DIGITALOCEAN_ACCESS_ID", "DIGITALOCEAN_SECRET_KEY", "DIGITALOCEAN_TOKEN")
	case "gcp":
		return requireEnv("GOOGLE_CREDENTIALS", "GOOGLE_PROJECT")
	default:
		return map[string]string{}, nil
	}
}

func terraformInstalled(c *stdcli.Context) bool {
	_, err := c.Execute("terraform", "version")
	return err == nil
}

func terraformLatestVersion() (string, error) {
	if TestLatest != "" {
		return TestLatest, nil
	}

	res, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", Image))
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

func terraformProviderVars(provider string) (map[string]string, error) {
	switch provider {
	case "do":
		vars := map[string]string{
			"access_id":  os.Getenv("DIGITALOCEAN_ACCESS_ID"),
			"secret_key": os.Getenv("DIGITALOCEAN_SECRET_KEY"),
			"token":      os.Getenv("DIGITALOCEAN_TOKEN"),
		}
		return vars, nil
	default:
		return map[string]string{}, nil
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

func terraformWriteBackend(filename, backend string) error {
	u, err := url.Parse(backend)
	if err != nil {
		return err
	}

	pw, _ := u.User.Password()

	params := map[string]interface{}{
		"Address":    fmt.Sprintf("https://%s%s", u.Host, u.Path),
		"Password":   pw,
		"SkipVerify": fmt.Sprintf("%t", os.Getenv("CONVOX_TERRAFORM_BACKEND_INSECURE") == "true"),
		"Username":   u.User.Username(),
	}

	t, err := template.New("main").Funcs(terraformTemplateHelpers()).Parse(`
		terraform {
			backend "http" {
				address        = "{{.Address}}/state"
				username       = "{{.Username}}"
				password       = "{{.Password}}"
				lock_address   = "{{.Address}}/lock"
				lock_method    = "POST"
				unlock_address = "{{.Address}}/lock"
				unlock_method  = "DELETE"
				skip_cert_verification = {{.SkipVerify}}
			}
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

func terraformWriteTemplate(filename, version string, params map[string]interface{}) error {
	if source := os.Getenv("CONVOX_TERRAFORM_SOURCE"); source != "" {
		params["Source"] = fmt.Sprintf(source, params["Provider"])
	} else {
		params["Source"] = fmt.Sprintf("github.com/%s//terraform/system/%s?ref=%s", Image, params["Provider"], version)
	}

	params["Release"] = version

	t, err := template.New("main").Funcs(terraformTemplateHelpers()).Parse(`
		module "system" {
			source = "{{.Source}}"

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
