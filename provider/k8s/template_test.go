package k8s

import (
	"fmt"
	"os"
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
	p.templater = templater.New(template.TemplatesFS, p.templateHelpers())

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
					Id:       2,
					DnsZones: []string{"example.org"},
					Cloudflare: &structs.Cloudflare{
						ApiKeySecretRefName: options.String("cloudflare-dns-credential-2"),
						ApiKeySecretRefKey:  options.String("api-key"),
						Email:               options.String("ops@example.com"),
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
	p.templater = templater.New(template.TemplatesFS, p.templateHelpers())

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
