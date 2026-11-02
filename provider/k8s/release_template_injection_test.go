package k8s

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	ktmpl "github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

var injectionPayloads = []string{
	"x\"\n---\nkind: ClusterRole\nconvoxInjected: 1",
	"x&#34;&#10;convoxInjected: 1&#10;---&#10;kind: ClusterRole",
	"x\nconvoxInjected: 1",
	"a\n---\nkind: ClusterRole\nconvoxInjected: 1",
	"x: y\n  convoxInjected: 1",
	"x convoxInjected: 1",
}

func injectionSites() map[string]func(string) string {
	q := strconv.Quote
	svc := func(s string) string { return "services:\n  web:\n    port: 3000\n" + s }

	return map[string]func(string) string{
		"label value":      func(p string) string { return svc("    labels:\n      k: " + q(p) + "\n") },
		"label key":        func(p string) string { return svc("    labels:\n      " + q(p) + ": v\n") },
		"annotation value": func(p string) string { return svc("    annotations:\n      - " + q("k="+p) + "\n") },
		"annotation key":   func(p string) string { return svc("    annotations:\n      - " + q(p+"=v") + "\n") },
		"ingress annotation": func(p string) string {
			return svc("    ingressAnnotations:\n      - " + q("x.example.com/k="+p) + "\n")
		},
		"ingress annotation key": func(p string) string { return svc("    ingressAnnotations:\n      - " + q(p+"=v") + "\n") },
		"env value":              func(p string) string { return svc("    environment:\n      - " + q("K="+p) + "\n") },
		"env key":                func(p string) string { return svc("    environment:\n      - " + q(p+"=v") + "\n") },
		"domain":                 func(p string) string { return svc("    domain:\n      - " + q(p) + "\n") },
		"whitelist":              func(p string) string { return svc("    whitelist: " + q(p) + "\n") },
		"health path":            func(p string) string { return svc("    health:\n      path: " + q(p) + "\n") },
		"command":                func(p string) string { return svc("    command: " + q("echo '"+p+"'") + "\n") },
		"node selector value":    func(p string) string { return svc("    nodeSelectorLabels:\n      k: " + q(p) + "\n") },
		"node selector key":      func(p string) string { return svc("    nodeSelectorLabels:\n      " + q(p) + ": v\n") },
		"mount path": func(p string) string {
			return svc("    volumeOptions:\n      - emptyDir:\n          id: a\n          mountPath: " + q(p) + "\n")
		},
		"config mount dir": func(p string) string {
			return svc("    configMounts:\n      - id: c\n        dir: " + q(p) + "\n        filename: f\n")
		},
		"certificate duration": func(p string) string { return svc("    certificate:\n      duration: " + q(p) + "\n") },
		"certificate id": func(p string) string {
			return svc("    domain:\n      - a.example.com\n    certificate:\n      id: " + q(p) + "\n")
		},
		"init image": func(p string) string { return svc("    initContainer:\n      image: " + q(p) + "\n") },
		"timer schedule": func(p string) string {
			return svc("") + "timers:\n  t:\n    command: x\n    schedule: " + q(p) + "\n    service: web\n"
		},
		"timer annotation": func(p string) string {
			return svc("") + "timers:\n  t:\n    command: x\n    schedule: \"0 * * * *\"\n    service: web\n    annotations:\n      - " + q("k="+p) + "\n"
		},
		"balancer annotation": func(p string) string {
			return svc("") + "balancers:\n  b:\n    service: web\n    ports:\n      80: 3000\n    annotations:\n      - " + q("k="+p) + "\n"
		},
		"balancer whitelist": func(p string) string {
			return svc("") + "balancers:\n  b:\n    service: web\n    ports:\n      80: 3000\n    whitelist:\n      - " + q(p) + "\n"
		},
		"resource version": func(p string) string {
			return svc("") + "resources:\n  db:\n    type: postgres\n    options:\n      version: " + q(p) + "\n"
		},
		"resource storage": func(p string) string {
			return svc("") + "resources:\n  db:\n    type: postgres\n    options:\n      storage: " + q(p) + "\n"
		},
		"resource image": func(p string) string {
			return svc("") + "resources:\n  db:\n    type: postgres\n    image: " + q(p) + "\n"
		},
	}
}

func renderInjectionCase(t *testing.T, data string) map[string]int {
	t.Helper()

	m, err := manifest.Load([]byte(data), map[string]string{})
	require.NoError(t, err)

	p := &Provider{Engine: &mock.TestEngine{}, Name: "rack", Domain: "example.org", DomainInternal: "internal.example.org"}
	p.templater = templater.New(ktmpl.TemplatesFS)

	docs := map[string]int{}

	for _, c := range goldenCases(p, m, &structs.App{Name: "myapp"}) {
		out, err := p.RenderTemplate(c.name, c.params)
		require.NoError(t, err, c.label)

		for _, d := range atom.SplitDocuments(out) {
			var v interface{}
			require.NoError(t, yaml.Unmarshal(d, &v), c.label)
			require.False(t, hasKey(v, "convoxInjected"), c.label)
			if mv, ok := v.(map[interface{}]interface{}); ok {
				require.NotEqual(t, "ClusterRole", mv["kind"], c.label)
			}
			docs[c.label]++
		}
	}

	return docs
}

func hasKey(v interface{}, key string) bool {
	switch t := v.(type) {
	case map[interface{}]interface{}:
		for k, vv := range t {
			if fmt.Sprint(k) == key || hasKey(vv, key) {
				return true
			}
		}
	case []interface{}:
		for _, vv := range t {
			if hasKey(vv, key) {
				return true
			}
		}
	}
	return false
}

func TestReleaseTemplatesQuoteManifestValues(t *testing.T) {
	for name, site := range injectionSites() {
		t.Run(name, func(t *testing.T) {
			baseline := renderInjectionCase(t, site("x"))
			for _, payload := range injectionPayloads {
				require.Equal(t, baseline, renderInjectionCase(t, site(payload)), "%q", payload)
			}
		})
	}
}

func TestLetsEncryptTemplateQuotesRoute53(t *testing.T) {
	p := &Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(ktmpl.TemplatesFS)

	for _, payload := range append([]string{"us-east-1"}, injectionPayloads...) {
		out, err := p.RenderTemplate("system/cert-manager-letsencrypt", map[string]interface{}{
			"Config": &structs.LetsEncryptConfig{Solvers: []*structs.Dns01Solver{{
				DnsZones: []string{"example.org"},
				Route53:  &structs.Route53{HostedZoneID: options.String(payload), Region: options.String(payload), Role: options.String("arn:aws:iam::123456789012:role/x")},
			}}},
			"IngressClass": "nginx",
		})
		require.NoError(t, err, "%q", payload)

		docs := atom.SplitDocuments(out)
		require.Len(t, docs, 1, "%q", payload)

		var v interface{}
		require.NoError(t, yaml.Unmarshal(docs[0], &v))
		require.False(t, hasKey(v, "convoxInjected"), "%q", payload)
	}
}
