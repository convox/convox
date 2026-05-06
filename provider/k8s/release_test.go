package k8s_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestReleaseCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kc := p.Convox.(*cvfake.Clientset)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, buildCreate(kc, "rack1-app1", "build1", "basic"))
		require.NoError(t, buildCreate(kc, "rack1-app1", "build2", "basic"))

		opts1 := structs.ReleaseCreateOptions{
			Build: options.String("build1"),
		}

		r1, err := p.ReleaseCreate("app1", opts1)
		require.NoError(t, err)
		require.NotNil(t, r1)
		require.Equal(t, "app1", r1.App)
		require.Equal(t, "build1", r1.Build)
		require.Equal(t, "foo", r1.Description)
		require.Equal(t, "", r1.Env)
		require.Equal(t, "services:\n  web:\n    build: .\n    port: 5000\n", r1.Manifest)

		opts2 := structs.ReleaseCreateOptions{
			Env: options.String("FOO=bar"),
		}

		r2, err := p.ReleaseCreate("app1", opts2)
		require.NoError(t, err)
		require.NotNil(t, r2)
		require.Equal(t, "app1", r2.App)
		require.Equal(t, "build1", r2.Build)
		require.Equal(t, "env add:FOO", r2.Description)
		require.Equal(t, "FOO=bar", r2.Env)
		require.Equal(t, "services:\n  web:\n    build: .\n    port: 5000\n", r2.Manifest)

		opts3 := structs.ReleaseCreateOptions{
			Build:       options.String("build2"),
			Description: options.String("desc3"),
		}

		r3, err := p.ReleaseCreate("app1", opts3)
		require.NoError(t, err)
		require.NotNil(t, r3)
		require.Equal(t, "app1", r3.App)
		require.Equal(t, "build2", r3.Build)
		require.Equal(t, "desc3", r3.Description)
		require.Equal(t, "FOO=bar", r3.Env)
		require.Equal(t, "services:\n  web:\n    build: .\n    port: 5000\n", r3.Manifest)
	})
}

func TestReleasePromote(t *testing.T) {
	t.Skip()
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kc := p.Convox.(*cvfake.Clientset)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, buildCreate(kc, "rack1-app1", "build1", "basic"))
		require.NoError(t, releaseCreate(kc, "rack1-app1", "release1", "basic"))
		require.NoError(t, releaseCreate(kc, "rack1-app1", "release2", "basic"))

		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)
		require.NoError(t, releaseApply(aa, "rack1-app1", "release2", "app", "basic-app"))

		err := p.ReleasePromote("app1", "release2", structs.ReleasePromoteOptions{})
		require.NoError(t, err)
	})
}

func releaseApply(aa *atom.MockInterface, ns, id, atm, fixture string) error {
	data, err := os.ReadFile(fmt.Sprintf("testdata/release-%s.yml", fixture))
	if err != nil {
		return errors.WithStack(err)
	}

	aa.On("Apply", ns, atm, mock.Anything).Return(func(args mock.Arguments) error {
		cfg := args.Get(2).(*atom.ApplyConfig)
		if string(cfg.Template) != string(data) {
			return fmt.Errorf("data didn't match")
		}
		return nil
	}).Once()

	return nil
}

func releaseCreate(kc cv.Interface, ns, id, fixture string) error {
	spec, err := releaseFixture(fixture)
	if err != nil {
		return errors.WithStack(err)
	}

	r := &ca.Release{
		ObjectMeta: am.ObjectMeta{
			Name: id,
		},
		Spec: *spec,
	}

	if _, err := kc.ConvoxV1().Releases(ns).Create(r); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func releaseFixture(name string) (*ca.ReleaseSpec, error) {
	data, err := os.ReadFile(fmt.Sprintf("testdata/release-%s.yml", name))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var fixture struct {
		Build       string
		Created     string
		Description string
		Env         map[string]string
		Manifest    interface{}
	}

	if err := yaml.Unmarshal(data, &fixture); err != nil {
		return nil, errors.WithStack(err)
	}

	ep := []string{}

	for k, v := range fixture.Env {
		ep = append(ep, fmt.Sprintf("%s=%s", k, v))
	}

	sort.Strings(ep)

	mdata, err := yaml.Marshal(fixture.Manifest)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	s := &ca.ReleaseSpec{
		Build:       fixture.Build,
		Created:     fixture.Created,
		Description: fixture.Description,
		Env:         strings.Join(ep, "\n"),
		Manifest:    string(mdata),
	}

	return s, nil
}

// kedaPrometheusManifestYaml returns a manifest that activates the KEDA
// prometheus-trigger code path: gpuUtilization autoscale without an explicit
// prometheusUrl ⇒ NeedsPrometheus()=true; scale.min < scale.max so the
// wantsAutoscale gate fires inside releaseTemplateServices.
func kedaPrometheusManifestYaml() string {
	return `services:
  worker:
    image: docker.io/library/nginx
    port: 5000
    scale:
      min: 1
      max: 3
      gpu:
        count: 1
      autoscale:
        gpuUtilization:
          threshold: 75
`
}

// setupKedaPrometheusTest builds the Provider-side fixtures shared by the 3
// TestKedaPrometheusTrigger_* cases: app+release+atom-Status mock, Provider's
// IsKedaEnabled flipped on, and a manifest.Services that needs Prometheus.
func setupKedaPrometheusTest(t *testing.T) func(*k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
	t.Helper()
	return func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		p.IsKedaEnabled = true

		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cc, _ := p.Convox.(*cvfake.Clientset)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", kedaPrometheusManifestYaml()))
		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		m, err := manifest.Load([]byte(kedaPrometheusManifestYaml()), structs.Environment{})
		require.NoError(t, err)
		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			m.Services
	}
}

// TestKedaPrometheusTrigger_PromURLEmpty_SkipsCreation locks the post-redesign
// behavior: when PROMETHEUS_URL is empty, the KEDA ScaledObject for a service
// that needs a Prometheus trigger is NOT created. The for-loop's `continue`
// skips this service while leaving the rest of the render flow intact. Pins
// the SPEC §3.13 anti-regression: the rc8 `defaultPrometheusURL` fallback is
// gone; KEDA prometheus-trigger autoscale requires explicit prometheus_url.
func TestKedaPrometheusTrigger_PromURLEmpty_SkipsCreation(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	out, _, err := runReleaseTemplateServicesEvents(t, setupKedaPrometheusTest(t))
	require.NoError(t, err, "release template render must succeed: %v", err)
	assert.NotContains(t, string(out), "ScaledObject",
		"no ScaledObject should be rendered when PROMETHEUS_URL is empty and trigger needs Prometheus")
	assert.NotContains(t, string(out), "convox-gpu-utilization",
		"the gpu-utilization KEDA trigger name must NOT appear in rendered YAML")
}

// TestKedaPrometheusTrigger_PromURLSet_CreatesTrigger pins the happy path:
// when PROMETHEUS_URL is set, the KEDA ScaledObject for a Prometheus-driven
// trigger is created with the explicit URL plumbed into the trigger spec
// (serverAddress metadata field).
func TestKedaPrometheusTrigger_PromURLSet_CreatesTrigger(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "http://prom.example:9090")
	out, _, err := runReleaseTemplateServicesEvents(t, setupKedaPrometheusTest(t))
	require.NoError(t, err, "release template render must succeed: %v", err)
	assert.Contains(t, string(out), "ScaledObject",
		"ScaledObject must be created when PROMETHEUS_URL is set")
	assert.Contains(t, string(out), "http://prom.example:9090",
		"explicit URL must be plumbed into the trigger spec serverAddress")
	assert.Contains(t, string(out), "convox-gpu-utilization",
		"the gpu-utilization KEDA trigger name must appear in rendered YAML")
}

// TestKedaPrometheusTrigger_NeedsPrometheusEvent_EmittedOnSkip pins the
// observability contract: when PROMETHEUS_URL is empty AND the service's
// autoscale config NeedsPrometheus(), a release:prometheus-skipped event is
// emitted with Data.reason populated so users can diagnose the missing
// trigger from audit log / event stream.
func TestKedaPrometheusTrigger_NeedsPrometheusEvent_EmittedOnSkip(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	_, events, err := runReleaseTemplateServicesEvents(t, setupKedaPrometheusTest(t))
	require.NoError(t, err, "release template render must succeed: %v", err)

	skipped := findAllByAction(events, "release:prometheus-skipped")
	require.Len(t, skipped, 1, "exactly one release:prometheus-skipped event expected")
	data, _ := skipped[0]["data"].(map[string]any)
	require.NotNil(t, data, "event payload must include data block")
	assert.NotEmpty(t, data["reason"], "Data.reason must be populated to aid user diagnosis")
	assert.Equal(t, "worker", data["service"], "Data.service must name the affected service")
	assert.Equal(t, "app1", data["app"], "Data.app must name the affected app")
	assert.Equal(t, "system", data["actor"], "Data.actor must be 'system' for system-emitted events")
}

// kedaCpuOnlyManifestYaml returns a manifest with cpu-only autoscale —
// NeedsPrometheus()=false. The KEDA fallback rewrite must NOT skip
// ScaledObject creation for this shape even when PROMETHEUS_URL is empty,
// because cpu triggers don't query Prometheus at all.
func kedaCpuOnlyManifestYaml() string {
	return `services:
  api:
    image: docker.io/library/nginx
    port: 5000
    scale:
      min: 1
      max: 4
      autoscale:
        cpu:
          threshold: 70
`
}

// kedaManualTriggersManifestYaml returns a manifest with raw scale.keda.triggers
// (aws-sqs-queue) — NeedsPrometheus()=false on the autoscale struct because
// scale.autoscale is unset. The KEDA fallback rewrite must NOT skip
// ScaledObject creation; SQS triggers don't query Prometheus.
func kedaManualTriggersManifestYaml() string {
	return `services:
  worker:
    image: docker.io/library/nginx
    port: 5000
    scale:
      min: 1
      max: 5
      keda:
        triggers:
        - type: aws-sqs-queue
          metadata:
            queueURL: https://sqs.us-east-1.amazonaws.com/123/jobs
            queueLength: "5"
            awsRegion: us-east-1
`
}

// kedaPrometheusWithVpaManifestYaml returns a manifest with both
// gpuUtilization autoscale (NeedsPrometheus=true) AND scale.vpa enabled.
// The KEDA fallback rewrite must skip the ScaledObject for the prometheus-
// dependent autoscale (since PROMETHEUS_URL is empty in the test) but VPA
// rendering must still proceed for the same service.
func kedaPrometheusWithVpaManifestYaml() string {
	return `services:
  worker:
    image: docker.io/library/nginx
    port: 5000
    scale:
      min: 1
      max: 3
      gpu:
        count: 1
      autoscale:
        gpuUtilization:
          threshold: 75
      vpa:
        updateMode: Recreate
`
}

// setupKedaTestWithManifest is a generic helper that wires up the Provider
// fixtures (app + release CRD + atom Status mock) for a given inline manifest
// YAML and service name. Returns the setup func for runReleaseTemplateServicesEvents.
func setupKedaTestWithManifest(t *testing.T, manifestYaml string, vpaEnabled bool) func(*k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
	t.Helper()
	return func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		p.IsKedaEnabled = true
		p.IsVpaEnabled = vpaEnabled

		kk, _ := p.Cluster.(*fake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cc, _ := p.Convox.(*cvfake.Clientset)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", manifestYaml))
		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		m, err := manifest.Load([]byte(manifestYaml), structs.Environment{})
		require.NoError(t, err)
		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			m.Services
	}
}

// TestKedaScaledObject_CpuOnly_PromURLEmpty_RendersScaledObject pins that
// CPU-based KEDA autoscale renders correctly even when PROMETHEUS_URL is
// empty. The KEDA fallback rewrite gates `continue` on
// `s.Scale.Autoscale.NeedsPrometheus()`; cpu triggers don't need Prometheus
// so the ScaledObject must still build. Catches the over-skip regression
// where `continue` is unconditional on empty PROMETHEUS_URL.
func TestKedaScaledObject_CpuOnly_PromURLEmpty_RendersScaledObject(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	out, events, err := runReleaseTemplateServicesEvents(t, setupKedaTestWithManifest(t, kedaCpuOnlyManifestYaml(), false))
	require.NoError(t, err, "release template render must succeed: %v", err)
	assert.Contains(t, string(out), "ScaledObject",
		"ScaledObject MUST render for cpu-only autoscale even with empty PROMETHEUS_URL")
	assert.Contains(t, string(out), "convox-cpu",
		"the cpu KEDA trigger name MUST appear in rendered YAML")

	skipped := findAllByAction(events, "release:prometheus-skipped")
	assert.Empty(t, skipped, "no release:prometheus-skipped event for cpu-only autoscale (NeedsPrometheus=false)")
}

// TestKedaScaledObject_ManualKedaTriggers_PromURLEmpty_RendersScaledObject
// pins manual `scale.keda.triggers` (e.g., aws-sqs-queue) render correctly
// even when PROMETHEUS_URL is empty — these triggers don't need Prometheus.
// Without this guard, users using SQS-driven autoscale silently lose
// their autoscaler on upgrade.
func TestKedaScaledObject_ManualKedaTriggers_PromURLEmpty_RendersScaledObject(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	out, events, err := runReleaseTemplateServicesEvents(t, setupKedaTestWithManifest(t, kedaManualTriggersManifestYaml(), false))
	require.NoError(t, err, "release template render must succeed: %v", err)
	assert.Contains(t, string(out), "ScaledObject",
		"ScaledObject MUST render for manual scale.keda.triggers (e.g. aws-sqs-queue) with empty PROMETHEUS_URL")
	assert.Contains(t, string(out), "aws-sqs-queue",
		"the aws-sqs-queue trigger type MUST appear in rendered YAML")

	skipped := findAllByAction(events, "release:prometheus-skipped")
	assert.Empty(t, skipped, "no release:prometheus-skipped event for manual keda triggers (NeedsPrometheus=false)")
}

// TestVPA_RendersWhenPrometheusAutoscaleSkipped pins VPA rendering survives
// the prometheus-skipped autoscale fallback. A service with both
// gpuUtilization autoscale (NeedsPrometheus=true) AND vpa enabled must still
// get a VPA rendered when PROMETHEUS_URL is empty — the autoscale skip is
// scoped via `continue` on the autoscale-block path; VPA rendering happens
// later in releaseTemplateServices and must not be affected.
func TestVPA_RendersWhenPrometheusAutoscaleSkipped(t *testing.T) {
	t.Setenv("PROMETHEUS_URL", "")
	out, _, err := runReleaseTemplateServicesEvents(t, setupKedaTestWithManifest(t, kedaPrometheusWithVpaManifestYaml(), true))
	require.NoError(t, err, "release template render must succeed: %v", err)
	assert.NotContains(t, string(out), "ScaledObject",
		"ScaledObject correctly skipped for gpuUtilization autoscale with empty PROMETHEUS_URL")
	assert.Contains(t, string(out), "VerticalPodAutoscaler",
		"VPA MUST render despite skipped autoscale — `continue` must be scoped to the autoscale block, not the whole loop iteration")
}
