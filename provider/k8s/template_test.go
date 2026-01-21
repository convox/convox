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
	"github.com/stretchr/testify/require"
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

// gpuTemplateFixture builds the params map required by both service.yml.tmpl
// and timer.yml.tmpl from an inline manifest string, returning a fresh
// Provider wired with the embedded template FS.
func gpuTemplateFixture(t *testing.T, src string) (*Provider, map[string]interface{}) {
	t.Helper()
	m, err := manifest.Load([]byte(src), map[string]string{})
	require.NoError(t, err)
	require.Equal(t, 1, len(m.Services))

	s := m.Services[0]
	params := map[string]interface{}{
		"Annotations":    s.AnnotationsMap(),
		"App":            &structs.App{Name: "test-app"},
		"Environment":    map[string]string{},
		"MaxSurge":       100,
		"MaxUnavailable": 100,
		"Namespace":      "ns",
		"Password":       "pass",
		"Rack":           "rack",
		"Release":        &structs.Release{Id: "r1"},
		"Replicas":       1,
		"Resources":      s.ResourceMap(),
		"Service":        s,
	}
	if len(m.Timers) > 0 {
		params["Timer"] = m.Timers[0]
	}

	p := &Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(template.TemplatesFS)

	return p, params
}

// countKey returns the number of lines in rendered that contain key when
// leading/trailing whitespace is stripped (guards against duplicate YAML keys).
func countKey(rendered, key string) int {
	n := 0
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == key {
			n++
		}
	}
	return n
}

func TestRenderServiceGpuVendorKey(t *testing.T) {
	cases := []struct {
		name     string
		vendor   string
		expected string
	}{
		{"nvidia short", "nvidia", `nvidia.com/gpu: "1"`},
		{"nvidia long", "nvidia.com", `nvidia.com/gpu: "1"`},
		{"amd short", "amd", `amd.com/gpu: "1"`},
		{"amd long", "amd.com", `amd.com/gpu: "1"`},
		{"bogus defaults nvidia", "bogus", `nvidia.com/gpu: "1"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`services:
  web:
    build: .
    port: 3000
    scale:
      gpu:
        count: 1
        vendor: %s
`, tc.vendor)
			p, params := gpuTemplateFixture(t, src)
			data, err := p.RenderTemplate("app/service", params)
			require.NoError(t, err)
			require.Contains(t, string(data), tc.expected)
			// requests + limits each should carry the same key; ensure no
			// stray residue from the pre-R1 suffix heuristic.
			require.NotContains(t, string(data), "bogus.com/gpu")
			require.NotContains(t, string(data), ".com.com/gpu")
		})
	}
}

func TestRenderServiceGpuVendorUnset(t *testing.T) {
	// Blank vendor is rewritten to "nvidia" by manifest.Validate (see
	// manifest.go:282) so the rendered key is still nvidia.com/gpu.
	src := `services:
  web:
    build: .
    port: 3000
    scale:
      gpu:
        count: 1
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)
	require.Contains(t, string(data), `nvidia.com/gpu: "1"`)
}

func TestRenderServiceTolerationMerger(t *testing.T) {
	cases := []struct {
		name            string
		src             string
		expectTolerKey  int
		expectDedicated bool
		expectGpuToler  bool
	}{
		{
			name: "no gpu, no dedicated",
			src: `services:
  web:
    build: .
    port: 3000
`,
			expectTolerKey:  0,
			expectDedicated: false,
			expectGpuToler:  false,
		},
		{
			name: "dedicated only",
			src: `services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/label: special
`,
			expectTolerKey:  1,
			expectDedicated: true,
			expectGpuToler:  false,
		},
		{
			name: "gpu only",
			src: `services:
  web:
    build: .
    port: 3000
    scale:
      gpu:
        count: 1
        vendor: nvidia
`,
			expectTolerKey:  1,
			expectDedicated: false,
			expectGpuToler:  true,
		},
		{
			name: "gpu and dedicated",
			src: `services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/label: gpu-pool
    scale:
      gpu:
        count: 1
        vendor: nvidia
`,
			expectTolerKey:  1,
			expectDedicated: true,
			expectGpuToler:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, params := gpuTemplateFixture(t, tc.src)
			data, err := p.RenderTemplate("app/service", params)
			require.NoError(t, err)
			rendered := string(data)
			require.Equal(t, tc.expectTolerKey, countKey(rendered, "tolerations:"),
				"unexpected number of tolerations: keys:\n%s", rendered)
			if tc.expectDedicated {
				require.Contains(t, rendered, `key: dedicated-node`)
			} else {
				require.NotContains(t, rendered, `key: dedicated-node`)
			}
			if tc.expectGpuToler {
				require.Contains(t, rendered, "key: nvidia.com/gpu")
			} else {
				require.NotContains(t, rendered, "key: nvidia.com/gpu")
			}
		})
	}
}

func TestRenderTimerTolerationMerger(t *testing.T) {
	cases := []struct {
		name            string
		src             string
		expectTolerKey  int
		expectDedicated bool
		expectGpuToler  bool
	}{
		{
			name: "no gpu, no dedicated",
			src: `services:
  worker:
    build: .
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`,
			expectTolerKey: 0,
		},
		{
			name: "dedicated only",
			src: `services:
  worker:
    build: .
    nodeSelectorLabels:
      convox.io/label: dedicated-pool
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`,
			expectTolerKey:  1,
			expectDedicated: true,
		},
		{
			name: "gpu only",
			src: `services:
  worker:
    build: .
    scale:
      gpu:
        count: 1
        vendor: nvidia
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`,
			expectTolerKey: 1,
			expectGpuToler: true,
		},
		{
			name: "gpu and dedicated",
			src: `services:
  worker:
    build: .
    nodeSelectorLabels:
      convox.io/label: gpu-pool
    scale:
      gpu:
        count: 1
        vendor: amd
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`,
			expectTolerKey:  1,
			expectDedicated: true,
			expectGpuToler:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, params := gpuTemplateFixture(t, tc.src)
			data, err := p.RenderTemplate("app/timer", params)
			require.NoError(t, err)
			rendered := string(data)
			require.Equal(t, tc.expectTolerKey, countKey(rendered, "tolerations:"),
				"unexpected number of tolerations: keys:\n%s", rendered)
			if tc.expectDedicated {
				require.Contains(t, rendered, `key: dedicated-node`)
			}
			if tc.expectGpuToler {
				// GPU vendor for this case can be nvidia or amd; assert the
				// extended-resource key appears as a toleration key.
				keyLine := "key: amd.com/gpu"
				if !strings.Contains(rendered, keyLine) {
					keyLine = "key: nvidia.com/gpu"
				}
				require.Contains(t, rendered, keyLine)
			}
		})
	}
}

func TestRenderServiceEmptyDirSizeLimit(t *testing.T) {
	src := `services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - emptyDir:
          id: shm
          mountPath: /dev/shm
          sizeLimit: "2Gi"
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)
	rendered := string(data)
	require.Contains(t, rendered, "sizeLimit: 2Gi")
	// When SizeLimit is set, the outer "emptyDir: {}" must not also emit.
	require.NotContains(t, rendered, "emptyDir: {}")
}

func TestRenderServiceEmptyDirMediumAndSizeLimit(t *testing.T) {
	src := `services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - emptyDir:
          id: shm
          mountPath: /dev/shm
          medium: Memory
          sizeLimit: "1Gi"
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)
	rendered := string(data)
	require.Contains(t, rendered, "medium: Memory")
	require.Contains(t, rendered, "sizeLimit: 1Gi")
}

func TestRenderTimerEmptyDirSizeLimit(t *testing.T) {
	src := `services:
  worker:
    build: .
    volumeOptions:
      - emptyDir:
          id: scratch
          mountPath: /scratch
          sizeLimit: "4Gi"
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/timer", params)
	require.NoError(t, err)
	rendered := string(data)
	require.Contains(t, rendered, "sizeLimit: 4Gi")
}

func TestGpuResourceKey(t *testing.T) {
	cases := []struct {
		vendor   string
		expected string
	}{
		{"", "nvidia.com/gpu"},
		{"nvidia", "nvidia.com/gpu"},
		{"nvidia.com", "nvidia.com/gpu"},
		{"amd", "amd.com/gpu"},
		{"amd.com", "amd.com/gpu"},
		{"bogus", "nvidia.com/gpu"},
		{"neuron", "nvidia.com/gpu"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("vendor=%q", tc.vendor), func(t *testing.T) {
			require.Equal(t, tc.expected, gpuResourceKey(tc.vendor))
		})
	}
}
