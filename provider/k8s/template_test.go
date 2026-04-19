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

func TestRenderTemplateServiceHealthPort(t *testing.T) {
	baseParams := func(svc manifest.Service) map[string]interface{} {
		return map[string]interface{}{
			"Annotations":    svc.AnnotationsMap(),
			"App":            &structs.App{Name: "test-app"},
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
		}
	}

	p := Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(template.TemplatesFS)

	render := func(svc manifest.Service) string {
		data, err := p.RenderTemplate("app/service", baseParams(svc))
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
		return string(data)
	}

	assertContains := func(name, got, want string) {
		if !strings.Contains(got, want) {
			t.Errorf("%s: expected rendered output to contain %q; got:\n%s", name, want, got)
		}
	}

	assertNotContains := func(name, got, unwanted string) {
		if strings.Contains(got, unwanted) {
			t.Errorf("%s: rendered output unexpectedly contains %q; got:\n%s", name, unwanted, got)
		}
	}

	t.Run("health.port overrides service port in readinessProbe", func(t *testing.T) {
		svc := manifest.Service{
			Name: "web",
			Port: manifest.ServicePortScheme{Port: 3000, Scheme: "http"},
			Health: manifest.ServiceHealth{
				Path:     "/health",
				Port:     manifest.ServicePortScheme{Port: 9090, Scheme: "http"},
				Grace:    5,
				Interval: 5,
				Timeout:  4,
			},
		}
		out := render(svc)
		assertContains("readiness port", out, "port: 9090")
		assertContains("readiness scheme", out, "scheme: HTTP")
		assertContains("main container port", out, "containerPort: 3000")
	})

	t.Run("health.port scheme coalesces from service scheme when unset", func(t *testing.T) {
		svc := manifest.Service{
			Name: "api",
			Port: manifest.ServicePortScheme{Port: 8443, Scheme: "https"},
			Health: manifest.ServiceHealth{
				Path:     "/health",
				Port:     manifest.ServicePortScheme{Port: 9090},
				Grace:    5,
				Interval: 5,
				Timeout:  4,
			},
		}
		out := render(svc)
		assertContains("readiness port", out, "port: 9090")
		assertContains("readiness scheme falls back to HTTPS", out, "scheme: HTTPS")
	})

	t.Run("liveness.port overrides service port when path is set", func(t *testing.T) {
		svc := manifest.Service{
			Name: "worker",
			Port: manifest.ServicePortScheme{Port: 3000, Scheme: "http"},
			Liveness: manifest.ServiceLiveness{
				Path:             "/live",
				Port:             manifest.ServicePortScheme{Port: 9091},
				Grace:            10,
				Interval:         5,
				Timeout:          5,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
			},
		}
		out := render(svc)
		// livenessProbe block should target 9091, readinessProbe should still target 3000.
		livenessIdx := strings.Index(out, "livenessProbe:")
		readinessIdx := strings.Index(out, "readinessProbe:")
		if livenessIdx < 0 {
			t.Fatalf("no livenessProbe rendered: %s", out)
		}
		livenessBlock := out[livenessIdx:readinessIdx]
		assertContains("livenessProbe port", livenessBlock, "port: 9091")
		// Backward compat: with no explicit Liveness.Port.Scheme, the
		// livenessProbe must NOT render a scheme line (matches master).
		assertNotContains("livenessProbe has no scheme when unset", livenessBlock, "scheme:")
		readinessBlock := out[readinessIdx:]
		assertContains("readinessProbe port", readinessBlock, "port: 3000")
	})

	t.Run("grpc probes honor health and liveness port overrides", func(t *testing.T) {
		svc := manifest.Service{
			Name:              "grpcsvc",
			Port:              manifest.ServicePortScheme{Port: 50051, Scheme: "GRPC"},
			GrpcHealthEnabled: true,
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
				Port: manifest.ServicePortScheme{Port: 50052},
			},
			Liveness: manifest.ServiceLiveness{
				Port:             manifest.ServicePortScheme{Port: 50053},
				SuccessThreshold: 1, FailureThreshold: 5,
			},
		}
		out := render(svc)
		assertContains("grpc readiness", out, "grpc:\n            port: 50052")
		assertContains("grpc liveness", out, "grpc:\n            port: 50053")
	})

	t.Run("liveness.port.scheme renders into livenessProbe httpGet", func(t *testing.T) {
		svc := manifest.Service{
			Name: "tlsworker",
			Port: manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
			Liveness: manifest.ServiceLiveness{
				Path:             "/live",
				Port:             manifest.ServicePortScheme{Port: 9091, Scheme: "https"},
				Grace:            10,
				Interval:         5,
				Timeout:          5,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
			},
		}
		out := render(svc)
		livenessIdx := strings.Index(out, "livenessProbe:")
		readinessIdx := strings.Index(out, "readinessProbe:")
		if livenessIdx < 0 || readinessIdx < 0 {
			t.Fatalf("probes not rendered: %s", out)
		}
		livenessBlock := out[livenessIdx:readinessIdx]
		assertContains("liveness scheme", livenessBlock, "scheme: HTTPS")
		assertContains("liveness port", livenessBlock, "port: 9091")
	})

	t.Run("cross-scheme GRPC guard prevents invalid httpGet", func(t *testing.T) {
		// User sets health.port.scheme: grpc on a plain-HTTP service. Expect the
		// readiness httpGet branch to be skipped entirely rather than emit
		// scheme: GRPC inside an httpGet (which Kubernetes would reject).
		svc := manifest.Service{
			Name: "weird",
			Port: manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
			Health: manifest.ServiceHealth{
				Path:     "/health",
				Port:     manifest.ServicePortScheme{Port: 9090, Scheme: "GRPC"},
				Grace:    5,
				Interval: 5,
				Timeout:  4,
			},
		}
		out := render(svc)
		assertNotContains("no httpGet scheme GRPC", out, "scheme: GRPC")
		// Without grpcHealthEnabled, no readinessProbe should render at all.
		assertNotContains("no readinessProbe", out, "readinessProbe:")
	})

	t.Run("no health.port falls back to service port (backward compat)", func(t *testing.T) {
		svc := manifest.Service{
			Name: "legacy",
			Port: manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
			},
		}
		out := render(svc)
		assertContains("readiness port falls back", out, "port: 8080")
		assertContains("readiness scheme falls back", out, "scheme: HTTP")
		assertNotContains("no unexpected port", out, "port: 0")
	})

	t.Run("startupProbe ignores health and liveness port overrides", func(t *testing.T) {
		svc := manifest.Service{
			Name: "startup",
			Port: manifest.ServicePortScheme{Port: 3000, Scheme: "http"},
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
				Port: manifest.ServicePortScheme{Port: 9090},
			},
			Liveness: manifest.ServiceLiveness{
				Path:             "/live",
				Port:             manifest.ServicePortScheme{Port: 9091},
				Grace:            10,
				Interval:         5,
				Timeout:          5,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
			StartupProbe: manifest.ServiceStartupProbe{
				TcpSocketPort:    "3000",
				Grace:            30,
				Interval:         10,
				Timeout:          5,
				SuccessThreshold: 1,
				FailureThreshold: 10,
			},
		}
		out := render(svc)
		startupIdx := strings.Index(out, "startupProbe:")
		if startupIdx < 0 {
			t.Fatalf("no startupProbe rendered: %s", out)
		}
		// Take a window that stops at the next probe block after startupProbe.
		rest := out[startupIdx:]
		endIdx := len(rest)
		for _, marker := range []string{"\n        livenessProbe:", "\n        readinessProbe:"} {
			if i := strings.Index(rest, marker); i > 0 && i < endIdx {
				endIdx = i
			}
		}
		startupBlock := rest[:endIdx]
		assertContains("startupProbe tcpSocket port", startupBlock, "port: 3000")
		assertNotContains("startupProbe does not use health port", startupBlock, "port: 9090")
		assertNotContains("startupProbe does not use liveness port", startupBlock, "port: 9091")
	})
}
