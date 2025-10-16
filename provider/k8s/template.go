package k8s

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"
	"sort"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

func (p *Provider) RenderTemplate(name string, params map[string]interface{}) ([]byte, error) {
	data, err := p.templater.Render(fmt.Sprintf("%s.yml.tmpl", name), params)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return common.FormatYAML(data)
}

type kvItem struct {
	Key   string
	Value string
}

func (p *Provider) templateHelpers() template.FuncMap {
	return template.FuncMap{
		"base64": func(s string) string {
			return string(base64.StdEncoding.EncodeToString([]byte(s)))
		},
		"coalesce": func(ss ...string) string {
			return common.CoalesceString(ss...)
		},
		"domains": func(app string, s manifest.Service) []string {
			ds := []string{
				p.Engine.ServiceHost(app, &s),
				// fmt.Sprintf("%s.%s.%s.local", s.Name, app, p.Name),
			}
			for _, d := range s.Domains {
				ds = append(ds, d)
			}
			return ds
		},
		"hasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},
		"keyValue": func(inputItems ...map[string]string) []kvItem {
			kv := map[string]string{}
			for _, e := range inputItems {
				for k, v := range e {
					kv[k] = v
				}
			}
			keys := []string{}
			for k := range kv {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			sorted := []kvItem{}
			for _, k := range keys {
				sorted = append(sorted, kvItem{Key: k, Value: kv[k]})
			}
			return sorted
		},
		"image": func(a *structs.App, s manifest.Service, r *structs.Release) (string, error) {
			repo, _, err := p.Engine.RepositoryHost(a.Name)
			if err != nil {
				return "", errors.WithStack(err)
			}
			return fmt.Sprintf("%s:%s.%s", repo, s.Name, r.Build), nil
		},
		"indent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.Replace(v, "\n", "\n"+pad, -1)
		},
		"nindent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return "\n" + pad + strings.Replace(v, "\n", "\n"+pad, -1)
		},
		"json": func(v interface{}) (string, error) {
			data, err := json.Marshal(v)
			if err != nil {
				return "", errors.WithStack(err)
			}
			return string(data), nil
		},
		"k8sname": func(s string) string {
			return nameFilter(s)
		},
		"k8snamev2": func(s string) string {
			return nameFilterV2(s)
		},
		"lower": func(s string) string {
			return strings.ToLower(s)
		},
		"pathJoin": filepath.Join,
		"quoteEscape": func(s string) string {
			return strings.ReplaceAll(s, "\"", "\\\"")
		},
		"safe": func(s string) template.HTML {
			return template.HTML(fmt.Sprintf("%q", s))
		},
		"shellsplit": func(s string) ([]string, error) {
			return shellquote.Split(s)
		},
		"sortedKeys": func(m map[string]string) []string {
			ks := []string{}
			for k := range m {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			return ks
		},
		"systemHost": func() string {
			return p.Engine.SystemHost()
		},
		"systemVolume": func(v string) bool {
			return systemVolume(v)
		},
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"volumeFrom": func(app, service, v string) string {
			return p.volumeFrom(app, service, v)
		},
		"volumeSources": func(app, service string, vs []string) []string {
			return p.volumeSources(app, service, vs)
		},
		"volumeName": func(app, v string) string {
			return p.volumeName(app, v)
		},
		"volumeTo": func(v string) (string, error) {
			return volumeTo(v)
		},
		"yamlMarshal": func(v interface{}) (string, error) {
			d, err := yaml.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("yamlMarshal: %s", err)
			}
			return string(d), nil
		},
	}
}

// func templateResources(filter string) ([]string, error) {
//   data, err := exec.Command("kubectl", "api-resources", "--verbs=list", "--namespaced", "-o", "name").CombinedOutput()
//   if err != nil {
//     return []string{}, nil
//   }

//   ars := strings.Split(strings.TrimSpace(string(data)), "\n")

//   rsh := map[string]bool{}

//   data, err = exec.Command("kubectl", "get", "-l", filter, "--all-namespaces", "-o", "json", strings.Join(ars, ",")).CombinedOutput()
//   if err != nil {
//     return []string{}, nil
//   }

//   if strings.TrimSpace(string(data)) == "" {
//     return []string{}, nil
//   }

//   var res struct {
//     Items []struct {
//       ApiVersion string `json:"apiVersion"`
//       Kind       string `json:"kind"`
//     }
//   }

//   if err := json.Unmarshal(data, &res); err != nil {
//     return nil, err
//   }

//   for _, i := range res.Items {
//     av := i.ApiVersion

//     if !strings.Contains(av, "/") {
//       av = fmt.Sprintf("core/%s", av)
//     }

//     rsh[fmt.Sprintf("%s/%s", av, i.Kind)] = true
//   }

//   rs := []string{}

//   for r := range rsh {
//     rs = append(rs, r)
//   }

//   sort.Strings(rs)

//   return rs, nil
// }
