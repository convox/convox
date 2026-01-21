package k8s

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s/template"
)

func TestRenderTemplate(t *testing.T) {
	p := Provider{}
	p.templater = templater.New(template.TemplatesFS)

	data, err := p.RenderTemplate(fmt.Sprintf("system/%s", "cert-manager-letsencrypt"), map[string]interface{}{
		"Config": structs.LetsEncryptConfig{
			Solvers: []*structs.Dns01Solver{
				{
					Id:       1,
					DnsZones: []string{"test.com"},
					Route53: &structs.Route53{
						HostedZoneID: options.String("host"),
						Region:       options.String("us"),
						Role:         options.String("role"),
					},
				},
				{
					Id:       1,
					DnsZones: []string{"test.com"},
					Route53: &structs.Route53{
						HostedZoneID: options.String("host"),
						Region:       options.String("us"),
						Role:         options.String("role"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(data))
}

func TestRenderTemplateServiceSecurityContext(t *testing.T) {
	a := &structs.App{Name: "test-app"}

	d, err := os.ReadFile("./testdata/securitycontext.yml")
	if err != nil {
		t.Fatal(err)
	}

	m, err := manifest.Load(d, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	p := Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(template.TemplatesFS)

	render := func(tmpl string, svc manifest.Service) string {
		params := map[string]interface{}{
			"Annotations":    svc.AnnotationsMap(),
			"App":            a,
			"Environment":    map[string]string{},
			"MaxSurge":       100,
			"MaxUnavailable": 100,
			"Namespace":      "ns",
			"Password":       "pass",
			"Rack":           "rack",
			"Release":        &structs.Release{Id: "r1"},
			"Replicas":       2,
			"Resources":      svc.ResourceMap(),
			"Service":        svc,
			"Timer":          m.Timers[0],
		}
		data, err := p.RenderTemplate(tmpl, params)
		if err != nil {
			t.Fatalf("%s render: %v", svc.Name, err)
		}
		return string(data)
	}

	mustService := func(name string) manifest.Service {
		for _, s := range m.Services {
			if s.Name == name {
				return s
			}
		}
		t.Fatalf("service %q not in fixture", name)
		return manifest.Service{}
	}

	t.Run("full securityContext renders all fields", func(t *testing.T) {
		out := render("app/service", mustService("secured"))
		for _, want := range []string{
			"securityContext:",
			"runAsNonRoot: true",
			"runAsUser: 1000",
			"runAsGroup: 1000",
			"readOnlyRootFilesystem: true",
			"allowPrivilegeEscalation: false",
			"capabilities:",
			"drop:",
			"- ALL",
			"add:",
			"- NET_BIND_SERVICE",
			"seccompProfile:",
			"type: RuntimeDefault",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("expected %q in output\n---\n%s", want, out)
			}
		}
	})

	t.Run("plain service renders no securityContext block", func(t *testing.T) {
		out := render("app/service", mustService("plain"))
		if strings.Contains(out, "securityContext:") {
			t.Errorf("did not expect securityContext in output for plain service\n---\n%s", out)
		}
	})

	t.Run("legacy privileged renders securityContext.privileged", func(t *testing.T) {
		out := render("app/service", mustService("legacy"))
		if !strings.Contains(out, "securityContext:") || !strings.Contains(out, "privileged: true") {
			t.Errorf("expected privileged rendering for legacy\n---\n%s", out)
		}
	})

	t.Run("empty capabilities struct is suppressed", func(t *testing.T) {
		out := render("app/service", mustService("emptycaps"))
		if strings.Contains(out, "capabilities:") {
			t.Errorf("empty capabilities should not render\n---\n%s", out)
		}
	})

	t.Run("explicit zero values render", func(t *testing.T) {
		out := render("app/service", mustService("zerouid"))
		if !strings.Contains(out, "runAsUser: 0") {
			t.Errorf("expected runAsUser: 0 to render\n---\n%s", out)
		}
		if !strings.Contains(out, "readOnlyRootFilesystem: false") {
			t.Errorf("expected readOnlyRootFilesystem: false to render\n---\n%s", out)
		}
	})

	t.Run("timer inherits service securityContext", func(t *testing.T) {
		out := render("app/timer", mustService("secured"))
		if !strings.Contains(out, "seccompProfile:") || !strings.Contains(out, "type: RuntimeDefault") {
			t.Errorf("expected timer to render securityContext from referenced service\n---\n%s", out)
		}
	})
}

func TestRenderTemplateService(t *testing.T) {
	a := &structs.App{
		Name: "test-app",
	}

	d, err := os.ReadFile("./testdata/convox1.yml")
	if err != nil {
		t.Fatal(err)
	}

	m, err := manifest.Load(d, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	params := map[string]interface{}{
		"Annotations":    m.Services[0].AnnotationsMap(),
		"App":            a,
		"Environment":    map[string]string{},
		"MaxSurge":       100,
		"MaxUnavailable": 100,
		"Namespace":      "ns",
		"Password":       "pass",
		"Rack":           "rack",
		"Release": &structs.Release{
			Id: "r1",
		},
		"Replicas":  2,
		"Resources": m.Services[0].ResourceMap(),
		"Service":   m.Services[0],
		"Timer":     m.Timers[0],
	}

	p := Provider{
		Engine: &mock.TestEngine{},
	}
	p.templater = templater.New(template.TemplatesFS)

	var data []byte

	data, err = p.RenderTemplate("app/timer", params)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(data))

	data, err = p.RenderTemplate("app/service", params)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(data))
}
