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
	yaml "gopkg.in/yaml.v3"
)

func TestRenderTemplate(t *testing.T) {
	p := Provider{}
	p.templater = templater.New(template.TemplatesFS)

	data, err := p.RenderTemplate(fmt.Sprintf("system/%s", "cert-manager-letsencrypt"), map[string]interface{}{
		"IngressClass": "nginx",
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

func TestRenderTemplateCertManagerContour(t *testing.T) {
	p := Provider{}
	p.templater = templater.New(template.TemplatesFS)

	data, err := p.RenderTemplate("system/cert-manager-letsencrypt", map[string]interface{}{
		"IngressClass": "contour",
		"Config":       structs.LetsEncryptConfig{},
	})
	require.NoError(t, err)
	require.Contains(t, string(data), "class: contour")
	require.NotContains(t, string(data), "class: nginx")
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

func TestRenderStatefulServicePersistentVolumeClaim(t *testing.T) {
	src := `services:
  database:
    build: .
    port: 6333
    stateful: true
    podManagementPolicy: Parallel
    scale:
      count: 3
    volumeOptions:
      - persistentVolumeClaim:
          id: data
          mountPath: /qdrant/storage
          size: 200Gi
          storageClass: gp3
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)

	rendered := string(data)
	require.Contains(t, rendered, "kind: StatefulSet")
	require.Contains(t, rendered, "atom.conditions: Ready=True")
	require.Contains(t, rendered, "serviceName: database-headless")
	require.Contains(t, rendered, "podManagementPolicy: Parallel")
	require.Contains(t, rendered, "name: pvc-data")
	require.Contains(t, rendered, "mountPath: /qdrant/storage")
	require.Contains(t, rendered, "storageClassName: gp3")
	require.Contains(t, rendered, "storage: 200Gi")
	require.Contains(t, rendered, "name: database-headless")
	require.Contains(t, rendered, "clusterIP: None")
	require.NotContains(t, rendered, "kind: Deployment")
}

func TestRenderStatefulReadOnlyPersistentVolumeClaim(t *testing.T) {
	src := `services:
  database:
    build: .
    stateful: true
    volumeOptions:
      - persistentVolumeClaim:
          id: data
          accessMode: ReadOnlyMany
          mountPath: /data
          size: 1Gi
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)
	require.Contains(t, string(data), "readOnly: true")
}

func TestRenderStatefulServiceInitContainerClaimAndLabels(t *testing.T) {
	src := `services:
  database:
    build: .
    stateful: true
    labels:
      team: data
    initContainer:
      image: busybox
      command: chown -R 1000:1000 /data
      volumeOptions:
        - persistentVolumeClaim:
            id: data
            mountPath: /data
    securityContext:
      runAsUser: 1000
      fsGroup: 1000
    volumeOptions:
      - persistentVolumeClaim:
          id: data
          mountPath: /data
          size: 1Gi
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)

	docs := splitYamlDocuments(t, string(data))

	sts := findRenderedDocument(t, docs, "StatefulSet", "database")
	podSpec := nestedMap(t, sts, "spec", "template", "spec")
	sc := nestedMap(t, podSpec, "securityContext")
	require.Equal(t, 1000, sc["fsGroup"])
	require.Equal(t, "OnRootMismatch", sc["fsGroupChangePolicy"])

	initContainers, _ := podSpec["initContainers"].([]interface{})
	require.Len(t, initContainers, 1)
	init, _ := initContainers[0].(map[string]interface{})
	require.Contains(t, fmt.Sprintf("%v", init["volumeMounts"]), "pvc-data")

	headless := findRenderedDocument(t, docs, "Service", "database-headless")
	labels := nestedMap(t, headless, "metadata", "labels")
	require.Equal(t, "data", labels["team"])
}

func TestRenderServiceOmitsPodSecurityContextWithoutFsGroup(t *testing.T) {
	src := `services:
  web:
    build: .
    securityContext:
      runAsUser: 1000
`
	p, params := gpuTemplateFixture(t, src)
	data, err := p.RenderTemplate("app/service", params)
	require.NoError(t, err)

	require.NotContains(t, string(data), "fsGroup")

	deployment := findRenderedDocument(t, splitYamlDocuments(t, string(data)), "Deployment", "web")
	podSpec := nestedMap(t, deployment, "spec", "template", "spec")
	require.NotContains(t, podSpec, "securityContext")
}

func splitYamlDocuments(t *testing.T, data string) []map[string]interface{} {
	t.Helper()
	docs := []map[string]interface{}{}
	for _, part := range strings.Split(data, "---\n") {
		var v map[string]interface{}
		require.NoError(t, yaml.Unmarshal([]byte(part), &v))
		if v != nil {
			docs = append(docs, v)
		}
	}
	return docs
}

func findRenderedDocument(t *testing.T, docs []map[string]interface{}, kind, name string) map[string]interface{} {
	t.Helper()
	for _, d := range docs {
		if d["kind"] != kind {
			continue
		}
		if nestedMap(t, d, "metadata")["name"] == name {
			return d
		}
	}
	t.Fatalf("no rendered %s named %s", kind, name)
	return nil
}

func nestedMap(t *testing.T, v map[string]interface{}, keys ...string) map[string]interface{} {
	t.Helper()
	for _, k := range keys {
		next, ok := v[k].(map[string]interface{})
		require.True(t, ok, "expected map at %s", k)
		v = next
	}
	return v
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

func TestRenderTimerTerminationGrace(t *testing.T) {
	set := `services:
  worker:
    build: .
    termination:
      grace: 300
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`
	p, params := gpuTemplateFixture(t, set)
	data, err := p.RenderTemplate("app/timer", params)
	require.NoError(t, err)
	require.Contains(t, string(data), "terminationGracePeriodSeconds: 300")

	unset := `services:
  worker:
    build: .
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`
	p, params = gpuTemplateFixture(t, unset)
	data, err = p.RenderTemplate("app/timer", params)
	require.NoError(t, err)
	require.Contains(t, string(data), "terminationGracePeriodSeconds: 30")
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
		{"google", "google.com/tpu"},
		{"google.com", "google.com/tpu"},
		{"tpu", "google.com/tpu"},
		{"bogus", "nvidia.com/gpu"},
		{"neuron", "nvidia.com/gpu"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("vendor=%q", tc.vendor), func(t *testing.T) {
			require.Equal(t, tc.expected, gpuResourceKey(tc.vendor))
		})
	}
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

	// Labels elsewhere render as "service:" but never this deep inside a grpc block.
	const grpcProbeService = "\n            service:"

	grpcService := func(name string) manifest.Service {
		return manifest.Service{
			Name:              name,
			Port:              manifest.ServicePortScheme{Port: 50051, Scheme: "GRPC"},
			GrpcHealthEnabled: true,
			Health: manifest.ServiceHealth{
				Path: "/", Grace: 5, Interval: 5, Timeout: 4,
			},
		}
	}

	t.Run("grpc probes omit the service name when health.grpcService is unset", func(t *testing.T) {
		out := render(grpcService("legacygrpc"))
		assertNotContains("no grpc service name", out, grpcProbeService)
		// Pins the bytes: port is the only key inside grpc, so the next key is the probe's.
		assertContains("grpc block unchanged", out, "grpc:\n            port: 50051\n          initialDelaySeconds: 5")
	})

	t.Run("health.grpcService renders in the grpc readinessProbe only", func(t *testing.T) {
		svc := grpcService("namedgrpc")
		svc.Health.GrpcService = "myapp.v1.Greeter"
		out := render(svc)
		assertContains("readiness service name", out, "grpc:\n            port: 50051\n            service: myapp.v1.Greeter")
		if n := countKey(out, "service: myapp.v1.Greeter"); n != 1 {
			t.Errorf("expected the service name exactly once (readiness only); got %d:\n%s", n, out)
		}
	})

	t.Run("health.grpcService coexists with health.port and liveness.port", func(t *testing.T) {
		svc := grpcService("portedgrpc")
		svc.Health.GrpcService = "myapp.v1.Greeter"
		svc.Health.Port = manifest.ServicePortScheme{Port: 50052}
		svc.Liveness = manifest.ServiceLiveness{
			Port:             manifest.ServicePortScheme{Port: 50053},
			SuccessThreshold: 1,
			FailureThreshold: 5,
		}
		out := render(svc)
		livenessIdx := strings.Index(out, "livenessProbe:")
		readinessIdx := strings.Index(out, "readinessProbe:")
		if livenessIdx < 0 || readinessIdx < 0 || livenessIdx > readinessIdx {
			t.Fatalf("unexpected probe layout: %s", out)
		}
		livenessBlock := out[livenessIdx:readinessIdx]
		readinessBlock := out[readinessIdx:]
		assertContains("readiness keeps health.port and the service name", readinessBlock, "grpc:\n            port: 50052\n            service: myapp.v1.Greeter")
		assertContains("liveness keeps liveness.port", livenessBlock, "port: 50053")
		assertNotContains("liveness has no service name", livenessBlock, grpcProbeService)
	})

	t.Run("health.path never becomes the grpc service name", func(t *testing.T) {
		svc := grpcService("pathgrpc")
		svc.Health.Path = "/ready"
		out := render(svc)
		assertNotContains("no grpc service name", out, grpcProbeService)
		assertNotContains("health.path is not reused", out, "service: /ready")
		assertContains("grpc probes still render", out, "grpc:\n            port: 50051")
	})

	t.Run("http service ignores health.grpcService", func(t *testing.T) {
		svc := manifest.Service{
			Name: "httpsvc",
			Port: manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
			Health: manifest.ServiceHealth{
				Path: "/health", GrpcService: "myapp.v1.Greeter", Grace: 5, Interval: 5, Timeout: 4,
			},
		}
		out := render(svc)
		assertNotContains("no grpc block", out, "grpc:")
		assertNotContains("no grpc service name", out, "service: myapp.v1.Greeter")
		assertContains("httpGet still uses health.path", out, "path: /health")
	})

	t.Run("health.disable removes both grpc probes", func(t *testing.T) {
		svc := grpcService("disabledgrpc")
		svc.Health.Disable = true
		out := render(svc)
		assertNotContains("no readinessProbe", out, "readinessProbe:")
		assertNotContains("no livenessProbe", out, "livenessProbe:")
	})

	t.Run("health.disable on an http service does not emit grpc probes", func(t *testing.T) {
		svc := manifest.Service{
			Name:              "httpdisabled",
			Port:              manifest.ServicePortScheme{Port: 3000, Scheme: "http"},
			GrpcHealthEnabled: true,
			Health: manifest.ServiceHealth{
				Path: "/", Disable: true, Grace: 5, Interval: 5, Timeout: 4,
			},
		}
		out := render(svc)
		assertNotContains("no readinessProbe", out, "readinessProbe:")
		assertNotContains("no livenessProbe", out, "livenessProbe:")
		assertNotContains("no grpc block against the http port", out, "grpc:")
	})

	t.Run("health.disable on an http service preserves the configured liveness probe", func(t *testing.T) {
		svc := manifest.Service{
			Name:              "httpdisabledlive",
			Port:              manifest.ServicePortScheme{Port: 3000, Scheme: "http"},
			GrpcHealthEnabled: true,
			Health: manifest.ServiceHealth{
				Path: "/", Disable: true, Grace: 5, Interval: 5, Timeout: 4,
			},
			Liveness: manifest.ServiceLiveness{
				Path:             "/live",
				Grace:            10,
				Interval:         5,
				Timeout:          5,
				SuccessThreshold: 1,
				FailureThreshold: 3,
			},
		}
		out := render(svc)
		assertContains("liveness keeps the configured path", out, "path: /live")
		assertContains("liveness keeps the configured threshold", out, "failureThreshold: 3")
		assertNotContains("no grpc block against the http port", out, "grpc:")
		assertNotContains("no readinessProbe", out, "readinessProbe:")
	})

	t.Run("grpc port without grpcHealthEnabled renders no probes", func(t *testing.T) {
		svc := grpcService("optedout")
		svc.GrpcHealthEnabled = false
		svc.Liveness = manifest.ServiceLiveness{
			Path:             "/live",
			Grace:            10,
			Interval:         5,
			Timeout:          5,
			SuccessThreshold: 1,
			FailureThreshold: 3,
		}
		out := render(svc)
		assertNotContains("no readinessProbe", out, "readinessProbe:")
		assertNotContains("no livenessProbe", out, "livenessProbe:")
	})
}
