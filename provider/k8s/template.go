package k8s

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	shellquote "github.com/kballard/go-shellquote"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

func (p *Provider) RenderTemplate(name string, params map[string]interface{}) ([]byte, error) {
	data, err := p.templater.Render(fmt.Sprintf("%s.yml.tmpl", name), params, p.templateHelpers())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	out, err := common.FormatYAML(data)
	if err != nil {
		return nil, err
	}

	if strings.HasPrefix(name, "app/") || strings.HasPrefix(name, "resource/") {
		if err := checkRenderedKinds(name, out, params); err != nil {
			return nil, err
		}
	}

	return out, nil
}

var yamlPlain = regexp.MustCompile(`^[ \t]*([A-Za-z0-9._/:+~=-][A-Za-z0-9._/:@+~=-]*)?[ \t]*$`)

func yamlHTML(s string) template.HTML {
	return template.HTML(s) //nolint:gosec // YAML output; callers pass only a quoted scalar or a value matching yamlPlain
}

func yamlQuote(s string) template.HTML {
	return yamlHTML(strings.ReplaceAll(strconv.Quote(s), "&", `\x26`))
}

// yamlScalar leaves plain values bare so YAML reads them as before, and quotes the rest after trimming what a plain scalar drops.
func yamlScalar(s string) template.HTML {
	if yamlPlain.MatchString(s) {
		return yamlHTML(s)
	}

	v := strings.TrimRight(strings.TrimLeft(s, " \t"), " \t\r\n\u0085\u2028\u2029")
	if yamlPlain.MatchString(v) {
		return yamlHTML(v)
	}

	return yamlQuote(v)
}

var renderedKinds = map[string][]string{
	"app/app":              {"Namespace"},
	"app/azurefiles":       {"PersistentVolumeClaim"},
	"app/balancer":         {"Service"},
	"app/ca":               {"ConfigMap"},
	"app/certificate":      {"Certificate"},
	"app/efs":              {"PersistentVolume", "PersistentVolumeClaim"},
	"app/httpproxy":        {"HTTPProxy"},
	"app/ingress":          {"Ingress"},
	"app/ingress-internal": {"Ingress"},
	"app/service":          {"Secret", "ServiceAccount", "Deployment", "StatefulSet", "DaemonSet", "HorizontalPodAutoscaler", "Service"},
	"app/timer":            {"ServiceAccount", "CronJob"},
}

func checkRenderedKinds(name string, data []byte, params map[string]interface{}) error {
	kinds := renderedKinds[name]
	if strings.HasPrefix(name, "resource/") {
		kinds = []string{"ConfigMap", "PersistentVolumeClaim", "Deployment", "Service"}
	}

	ns, _ := params["Namespace"].(string)

	for _, d := range atom.SplitDocuments(data) {
		var raw interface{}
		if err := yaml.Unmarshal(d, &raw); err != nil {
			return errors.WithStack(err)
		}
		if raw == nil {
			continue
		}

		var o struct {
			Kind     string
			Metadata struct {
				Name      string
				Namespace string
			}
		}
		if err := yaml.Unmarshal(d, &o); err != nil {
			return errors.WithStack(err)
		}

		if !slices.Contains(kinds, o.Kind) {
			return errors.Errorf("template %s rendered kind %q", name, o.Kind)
		}
		if o.Metadata.Namespace != "" && o.Metadata.Namespace != ns {
			return errors.Errorf("template %s rendered %s in namespace %q", name, o.Kind, o.Metadata.Namespace)
		}
		if name == "app/app" && o.Metadata.Name != ns {
			return errors.Errorf("template %s rendered namespace %q", name, o.Metadata.Name)
		}
	}

	return nil
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
		"deref": func(b *bool) bool {
			if b == nil {
				return false
			}
			return *b
		},
		"domains": func(app string, s manifest.Service) []string {
			ds := []string{
				p.ServiceHost(app, s),
				// fmt.Sprintf("%s.%s.%s.local", s.Name, app, p.Name),
			}
			for _, d := range s.Domains {
				ds = append(ds, d)
			}
			return ds
		},
		"gpuResourceKey": func(vendor string) string {
			return gpuResourceKey(vendor)
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
		"internalResourceDotDomainSuffix": func() string {
			if options.GetFeatureGateValue(options.FeatureGateResourceInternalDomainSuffix) != "" {
				return options.GetFeatureGateValue(options.FeatureGateResourceInternalDomainSuffix)
			}
			return "svc.cluster.local"
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
			return yamlQuote(html.UnescapeString(s))
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
		"yamlQuote":  yamlQuote,
		"yamlScalar": yamlScalar,
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
