package cli

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/convox/stdsdk"
)

type rack struct {
	Name     string
	Provider string
	Remote   bool
	Status   string
	Url      string
}

func app(c *stdcli.Context) string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return coalesce(c.String("app"), c.LocalSetting("app"), filepath.Base(wd))
}

func coalesce(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}

	return ""
}

func currentHost(c *stdcli.Context) (string, error) {
	if h := os.Getenv("CONVOX_HOST"); h != "" {
		return h, nil
	}

	if h, _ := c.SettingRead("host"); h != "" {
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

func currentEndpoint(c *stdcli.Context) (string, error) {
	if e := os.Getenv("RACK_URL"); e != "" {
		return e, nil
	}

	host, err := currentHost(c)
	if err != nil {
		return "", err
	}

	if pw := os.Getenv("CONVOX_PASSWORD"); host != "" && pw != "" {
		return fmt.Sprintf("https://convox:%s@%s", url.QueryEscape(pw), host), nil
	}

	r, err := matchRack(c, currentRack(c, host))
	if err != nil {
		return "", err
	}

	return r.Url, nil
}

func currentRack(c *stdcli.Context, host string) string {
	if r := c.String("rack"); r != "" {
		return r
	}

	if r := os.Getenv("CONVOX_RACK"); r != "" {
		return r
	}

	if r := c.LocalSetting("rack"); r != "" {
		return r
	}

	if r := hostRacks(c)[host]; r != "" {
		return r
	}

	if r, _ := c.SettingRead("rack"); r != "" {
		return r
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

func localRackRunning(c *stdcli.Context) bool {
	rs, err := localRacks(c)
	if err != nil {
		return false
	}

	return len(rs) > 0
}

func localRacks(c *stdcli.Context) ([]rack, error) {
	dir, err := c.SettingDirectory("racks")
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []rack{}, nil
	}

	subs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	racks := []rack{}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	defer os.Chdir(wd)

	for _, sub := range subs {
		if !sub.IsDir() {
			continue
		}

		tf := filepath.Join(dir, sub.Name(), "main.tf")

		if _, err := os.Stat(tf); os.IsNotExist(err) {
			continue
		}

		os.Chdir(filepath.Dir(tf))

		data, err := c.Execute("terraform", "output", "-json")
		if err != nil {
			fmt.Printf("err: %+v\n", err)
			continue
		}

		var output map[string]struct {
			Sensitive bool
			Type      string
			Value     string
		}

		if err := json.Unmarshal(data, &output); err != nil {
			fmt.Printf("err: %+v\n", err)
			continue
		}

		api := ""
		provider := "unknown"
		status := "unknown"

		if o, ok := output["api"]; ok {
			api = o.Value
			status = "running"
		}

		if _, err := os.Stat(".terraform.tfstate.lock.info"); !os.IsNotExist(err) {
			status = "updating"
		}

		if o, ok := output["provider"]; ok {
			provider = o.Value
		}

		racks = append(racks, rack{
			Name:     sub.Name(),
			Provider: strings.TrimSpace(string(provider)),
			Status:   status,
			Url:      strings.TrimSpace(string(api)),
		})
	}

	return racks, nil
}

func matchRack(c *stdcli.Context, name string) (*rack, error) {
	rs, err := racks(c)
	if err != nil {
		return nil, err
	}

	matches := []rack{}

	for _, r := range rs {
		if r.Name == name {
			return &r, nil
		}

		if strings.Index(r.Name, name) != -1 {
			matches = append(matches, r)
		}
	}

	if name == "" {
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("no racks found")
		case 1:
			return &matches[0], nil
		default:
			return nil, fmt.Errorf("multiple racks detected, use `convox switch` to select one")
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("could not find rack: %s", name)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("ambiguous rack name: %s", name)
	}
}

func racks(c *stdcli.Context) ([]rack, error) {
	rs := []rack{}

	lrs, err := localRacks(c)
	if err != nil {
		return nil, err
	}

	rs = append(rs, lrs...)

	rrs, err := remoteRacks(c)
	if err != nil {
		return nil, err
	}

	rs = append(rs, rrs...)

	sort.Slice(rs, func(i, j int) bool {
		switch {
		case !rs[i].Remote && rs[j].Remote:
			return true
		case rs[i].Remote && !rs[j].Remote:
			return false
		default:
			return rs[i].Name < rs[j].Name
		}
	})

	return rs, nil
}

func remoteRacks(c *stdcli.Context) ([]rack, error) {
	host, err := currentHost(c)
	if err != nil {
		return nil, err
	}
	if host == "" {
		return []rack{}, nil
	}

	pw, err := currentPassword(c, host)
	if err != nil {
		return nil, err
	}

	remote := fmt.Sprintf("https://convox:%s@%s", url.QueryEscape(pw), host)

	p, err := sdk.New(remote)
	if err != nil {
		return nil, err
	}

	p.Authenticator = authenticator(c)
	p.Session = currentSession(c)

	var rs []struct {
		Name         string
		Organization struct {
			Name string
		}
		Provider string
		Status   string
	}

	if err := p.Get("/racks", stdsdk.RequestOptions{}, &rs); err != nil {
		if _, ok := err.(AuthenticationError); ok {
			return nil, err
		}
	}

	racks := []rack{}

	for _, r := range rs {
		racks = append(racks, rack{
			Name:     fmt.Sprintf("%s/%s", r.Organization.Name, r.Name),
			Provider: coalesce(r.Provider, "unknown"),
			Remote:   true,
			Status:   r.Status,
			Url:      remote,
		})
	}

	return racks, nil
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

func switchRack(c *stdcli.Context, name string) error {
	rs := hostRacks(c)

	host, err := currentHost(c)
	if err != nil {
		return err
	}

	rs[host] = name

	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return err
	}

	if err := c.SettingWrite("switch", string(data)); err != nil {
		return err
	}

	return nil
}

func tag(name, value string) string {
	return fmt.Sprintf("<%s>%s</%s>", name, value, name)
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

func terraformOptionVars(dir string, args []string) (map[string]string, error) {
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

	for _, arg := range args {
		parts := strings.Split(arg, "=")
		k := strings.TrimSpace(parts[0])
		if v := strings.TrimSpace(parts[1]); v != "" {
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
