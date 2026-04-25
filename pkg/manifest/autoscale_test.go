package manifest_test

import (
	"math"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestAutoscaleIsEnabled(t *testing.T) {
	var a *manifest.ServiceAutoscale
	require.False(t, a.IsEnabled(), "nil receiver returns false")

	a = &manifest.ServiceAutoscale{}
	require.False(t, a.IsEnabled(), "zero value returns false")

	a = &manifest.ServiceAutoscale{Cpu: &manifest.AutoscaleThreshold{Threshold: 70}}
	require.True(t, a.IsEnabled())

	a = &manifest.ServiceAutoscale{Custom: []kedav1alpha1.ScaleTriggers{{Type: "cron", Name: "nightly"}}}
	require.True(t, a.IsEnabled())
}

func TestAutoscaleNeedsPrometheus(t *testing.T) {
	var a *manifest.ServiceAutoscale
	require.False(t, a.NeedsPrometheus())

	a = &manifest.ServiceAutoscale{GpuUtilization: &manifest.AutoscaleThreshold{Threshold: 80}}
	require.True(t, a.NeedsPrometheus(), "gpu util without explicit URL needs Prometheus")

	a = &manifest.ServiceAutoscale{GpuUtilization: &manifest.AutoscaleThreshold{Threshold: 80, PrometheusUrl: "http://prom:9090"}}
	require.False(t, a.NeedsPrometheus(), "explicit URL satisfies")

	a = &manifest.ServiceAutoscale{QueueDepth: &manifest.AutoscaleQueueDepth{Threshold: 5}}
	require.True(t, a.NeedsPrometheus())

	a = &manifest.ServiceAutoscale{Cpu: &manifest.AutoscaleThreshold{Threshold: 70}}
	require.False(t, a.NeedsPrometheus(), "cpu-only never queries Prometheus")
}

func TestScaleMinMaxYaml_GpuUtilization(t *testing.T) {
	m, err := testdataManifest("autoscale-gpu", map[string]string{})
	require.NoError(t, err)
	require.Len(t, m.Services, 1)

	s := m.Services[0]
	require.NotNil(t, s.Scale.Min)
	require.NotNil(t, s.Scale.Max)
	require.Equal(t, 0, *s.Scale.Min)
	require.Equal(t, 10, *s.Scale.Max)

	require.True(t, s.Scale.Autoscale.IsEnabled())
	require.NotNil(t, s.Scale.Autoscale.GpuUtilization)
	require.Equal(t, float64(70), s.Scale.Autoscale.GpuUtilization.Threshold)

	require.Equal(t, 0, s.Scale.Count.Min)
	require.Equal(t, 10, s.Scale.Count.Max)
}

func TestScaleMinMaxYaml_QueueDepth(t *testing.T) {
	m, err := testdataManifest("autoscale-queue", map[string]string{})
	require.NoError(t, err)
	require.Len(t, m.Services, 1)

	s := m.Services[0]
	require.NotNil(t, s.Scale.Min)
	require.NotNil(t, s.Scale.Max)
	require.Equal(t, 0, *s.Scale.Min)
	require.Equal(t, 5, *s.Scale.Max)

	require.True(t, s.Scale.Autoscale.IsEnabled())
	require.NotNil(t, s.Scale.Autoscale.QueueDepth)
	require.Equal(t, float64(5), s.Scale.Autoscale.QueueDepth.Threshold)
	require.Equal(t, "vllm:num_requests_waiting", s.Scale.Autoscale.QueueDepth.MetricName)

	require.Equal(t, 0, s.Scale.Count.Min)
	require.Equal(t, 5, s.Scale.Count.Max)
}

func TestScaleMinMaxYaml_MinZeroNoAutoscale(t *testing.T) {
	m, err := testdataManifest("autoscale-min-zero", map[string]string{})
	require.NoError(t, err)
	require.Len(t, m.Services, 1)
	s := m.Services[0]
	require.NotNil(t, s.Scale.Min)
	require.Equal(t, 0, *s.Scale.Min)
	require.Nil(t, s.Scale.Max)
	require.False(t, s.Scale.Autoscale.IsEnabled())
	require.Equal(t, 0, s.Scale.Count.Min)
	require.Equal(t, 0, s.Scale.Count.Max)
}

func TestScaleMinMaxYaml_Combined(t *testing.T) {
	m, err := testdataManifest("autoscale-combined", map[string]string{})
	require.NoError(t, err)
	require.Len(t, m.Services, 1)

	s := m.Services[0]
	require.Equal(t, 1, *s.Scale.Min)
	require.Equal(t, 8, *s.Scale.Max)

	a := s.Scale.Autoscale
	require.True(t, a.IsEnabled())
	require.NotNil(t, a.Cpu)
	require.Equal(t, float64(70), a.Cpu.Threshold)
	require.NotNil(t, a.GpuUtilization)
	require.Equal(t, float64(75), a.GpuUtilization.Threshold)
	require.NotNil(t, a.QueueDepth)
	require.Equal(t, float64(3), a.QueueDepth.Threshold)
	require.NotNil(t, a.CooldownPeriod)
	require.Equal(t, int32(300), *a.CooldownPeriod)
	require.NotNil(t, a.PollingInterval)
	require.Equal(t, int32(15), *a.PollingInterval)

	require.Equal(t, 1, s.Scale.Count.Min)
	require.Equal(t, 8, s.Scale.Count.Max)
}

func TestScaleMinMaxCountResolution(t *testing.T) {
	cases := []struct {
		name          string
		yaml          string
		expectedMin   int
		expectedMax   int
		wantAutoscale bool
	}{
		{
			name: "autoscale only defaults to min 0 max 10",
			yaml: `services:
  svc:
    build: .
    port: 3000
    scale:
      gpu:
        count: 1
      autoscale:
        gpu_utilization:
          threshold: 70
`,
			expectedMin:   0,
			expectedMax:   10,
			wantAutoscale: true,
		},
		{
			name: "min only without autoscale pins max=min",
			yaml: `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 3
`,
			expectedMin: 3,
			expectedMax: 3,
		},
		{
			name: "min only with autoscale defaults max=10",
			yaml: `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 2
      autoscale:
        cpu:
          threshold: 80
        queue_depth:
          threshold: 3
`,
			expectedMin:   2,
			expectedMax:   10,
			wantAutoscale: true,
		},
		{
			name: "max only without autoscale pins min=max",
			yaml: `services:
  svc:
    build: .
    port: 3000
    scale:
      max: 4
`,
			expectedMin: 4,
			expectedMax: 4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := manifest.Load([]byte(tc.yaml), map[string]string{})
			require.NoError(t, err, "load")
			require.NoError(t, m.Validate(), "validate")
			require.Len(t, m.Services, 1)
			s := m.Services[0]
			require.Equal(t, tc.expectedMin, s.Scale.Count.Min, "Count.Min")
			require.Equal(t, tc.expectedMax, s.Scale.Count.Max, "Count.Max")
			require.Equal(t, tc.wantAutoscale, s.Scale.Autoscale.IsEnabled())
		})
	}
}

func TestValidateScaleMinZeroCpuOnly(t *testing.T) {
	y := `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 0
      max: 5
      autoscale:
        cpu:
          threshold: 70
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale.min: 0 combined with only always-active autoscale triggers")
}

func TestValidateScaleMinZeroCpuPlusQueueOk(t *testing.T) {
	y := `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 0
      max: 5
      autoscale:
        cpu:
          threshold: 70
        queue_depth:
          threshold: 3
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}

func TestValidateReservedTriggerName(t *testing.T) {
	y := `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 0
      max: 5
      autoscale:
        custom:
        - type: cron
          name: convox-my-cron
          metadata:
            timezone: UTC
            start: 0 9 * * 1-5
            end: 0 17 * * 1-5
            desiredReplicas: "1"
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "uses reserved prefix 'convox-'")
}

func TestValidateAutoscaleBounds(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "cpu threshold above 100",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 1
      max: 5
      autoscale:
        cpu:
          threshold: 150
`,
			want: "scale.autoscale.cpu.threshold must be between 1 and 100",
		},
		{
			name: "gpu threshold above 100",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      gpu:
        count: 1
      autoscale:
        gpu_utilization:
          threshold: 120
`,
			want: "scale.autoscale.gpu_utilization.threshold must be > 0 and <= 100",
		},
		{
			name: "queue threshold zero",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        queue_depth:
          threshold: 0
`,
			want: "scale.autoscale.queue_depth.threshold must be > 0",
		},
		{
			name: "gpu util requires gpu count",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        gpu_utilization:
          threshold: 70
`,
			want: "scale.autoscale.gpu_utilization requires scale.gpu.count >= 1",
		},
		{
			name: "invalid prometheus URL",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        queue_depth:
          threshold: 3
          prometheus_url: "://bad"
`,
			want: "scale.autoscale.queue_depth.prometheus_url is not a valid URL",
		},
		{
			name: "invalid metric name",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        queue_depth:
          threshold: 3
          metric_name: "1-bad-name"
`,
			want: "metric_name",
		},
		{
			name: "negative scale.min",
			yaml: `services:
  svc:
    build: .
    scale:
      min: -1
      max: 3
`,
			want: "scale.min must be >= 0",
		},
		{
			name: "max less than min",
			yaml: `services:
  svc:
    build: .
    scale:
      min: 5
      max: 3
`,
			want: "scale.max must be >= scale.min",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := manifest.Load([]byte(tc.yaml), map[string]string{})
			require.NoError(t, err, "load")
			err = m.Validate()
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), tc.want), "want %q in %s", tc.want, err.Error())
		})
	}
}

func TestBuildTriggers_Shapes(t *testing.T) {
	a := &manifest.ServiceAutoscale{
		Cpu:    &manifest.AutoscaleThreshold{Threshold: 70},
		Memory: &manifest.AutoscaleThreshold{Threshold: 80},
		GpuUtilization: &manifest.AutoscaleThreshold{
			Threshold: 75,
		},
		QueueDepth: &manifest.AutoscaleQueueDepth{
			Threshold: 4,
		},
	}
	triggers := a.BuildTriggers("myapp", "mysvc", "http://prom/")
	require.Len(t, triggers, 4)

	require.Equal(t, "cpu", triggers[0].Type)
	require.Equal(t, "convox-cpu", triggers[0].Name)
	require.Equal(t, "70", triggers[0].Metadata["value"])

	require.Equal(t, "memory", triggers[1].Type)
	require.Equal(t, "convox-memory", triggers[1].Name)

	require.Equal(t, "prometheus", triggers[2].Type)
	require.Equal(t, "convox-gpu-utilization", triggers[2].Name)
	require.Equal(t, "75", triggers[2].Metadata["threshold"])
	require.Equal(t, "DCGM_FI_DEV_GPU_UTIL", triggers[2].Metadata["metricName"])
	require.Equal(t, `max(DCGM_FI_DEV_GPU_UTIL{app="myapp",service="mysvc"})`, triggers[2].Metadata["query"])
	require.Equal(t, "37.5", triggers[2].Metadata["activationThreshold"])
	require.Equal(t, "http://prom/", triggers[2].Metadata["serverAddress"])

	require.Equal(t, "prometheus", triggers[3].Type)
	require.Equal(t, "convox-queue-depth", triggers[3].Name)
	require.Equal(t, "4", triggers[3].Metadata["threshold"])
	require.Equal(t, "2", triggers[3].Metadata["activationThreshold"])
}

func TestBuildTriggers_ActivationThresholdFloorOne(t *testing.T) {
	a := &manifest.ServiceAutoscale{
		QueueDepth: &manifest.AutoscaleQueueDepth{Threshold: 1},
	}
	triggers := a.BuildTriggers("a", "s", "http://prom/")
	require.Equal(t, "1", triggers[0].Metadata["activationThreshold"], "floor of 1 applies")
}

func TestBuildTriggers_CustomPassthrough(t *testing.T) {
	a := &manifest.ServiceAutoscale{
		Custom: []kedav1alpha1.ScaleTriggers{
			{Type: "cron", Name: "after-hours", Metadata: map[string]string{"timezone": "UTC"}},
		},
	}
	triggers := a.BuildTriggers("a", "s", "")
	require.Len(t, triggers, 1)
	require.Equal(t, "cron", triggers[0].Type)
	require.Equal(t, "after-hours", triggers[0].Name)
}

func TestBackwardCompatExistingFixtures(t *testing.T) {
	// Existing fixtures (no autoscale fields) must still parse cleanly and
	// not grow unexpected Count{Min:1,Max:1} overrides.
	env := map[string]string{"REQUIRED": "x", "OTHERGLOBAL": "y", "SECRET": "z"}
	for _, name := range []string{"simple", "full", "keda", "startup-probe", "startup-probe-gpu"} {
		t.Run(name, func(t *testing.T) {
			m, err := testdataManifest(name, env)
			require.NoError(t, err, name)
			require.NotNil(t, m)
			for _, s := range m.Services {
				require.Nil(t, s.Scale.Min, "%s.%s", name, s.Name)
				require.Nil(t, s.Scale.Max, "%s.%s", name, s.Name)
				require.False(t, s.Scale.Autoscale.IsEnabled(), "%s.%s", name, s.Name)
			}
		})
	}
}

func TestKedaScaledObject_NilTriggersReturnsNil(t *testing.T) {
	svc := manifest.Service{Name: "svc"}
	obj := svc.KedaScaledObject(manifest.KedaScaledObjectParameters{
		MinCount:    0,
		MaxCount:    10,
		ServiceName: "svc",
		Namespace:   "ns",
	})
	require.Nil(t, obj, "no triggers + no Scale.Keda returns nil")
}

func TestKedaScaledObject_FromParamsTriggers(t *testing.T) {
	svc := manifest.Service{Name: "svc"}
	params := manifest.KedaScaledObjectParameters{
		MinCount:    0,
		MaxCount:    5,
		ServiceName: "svc",
		Namespace:   "myapp-ns",
		Triggers: []kedav1alpha1.ScaleTriggers{
			{Type: "prometheus", Name: "convox-queue-depth", Metadata: map[string]string{"threshold": "5"}},
		},
	}
	obj := svc.KedaScaledObject(params)
	require.NotNil(t, obj)
	require.Equal(t, "ScaledObject", obj.TypeMeta.Kind)
	require.Equal(t, "keda.sh/v1alpha1", obj.TypeMeta.APIVersion)
	require.Equal(t, "svc", obj.ObjectMeta.Name)
	require.Equal(t, "myapp-ns", obj.ObjectMeta.Namespace)
	require.Equal(t, int32(0), *obj.Spec.MinReplicaCount)
	require.Equal(t, int32(5), *obj.Spec.MaxReplicaCount)
	require.Len(t, obj.Spec.Triggers, 1)
	require.Nil(t, obj.Spec.Triggers[0].AuthenticationRef, "prometheus trigger not AWS - no auth attach")
}

func TestValidateRejectsNaNThreshold(t *testing.T) {
	m := &manifest.Manifest{
		Services: []manifest.Service{{
			Name: "svc",
			Scale: manifest.ServiceScale{
				Min: ptrInt(1), Max: ptrInt(5),
				Autoscale: &manifest.ServiceAutoscale{
					Cpu: &manifest.AutoscaleThreshold{Threshold: math.NaN()},
				},
			},
		}},
	}
	err := m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale.autoscale.cpu.threshold must be between 1 and 100")
}

func TestValidateRejectsPrometheusURLScheme(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"file scheme", "file:///etc/passwd", "must use http or https scheme"},
		{"empty host", "http:///foo", "must include a host"},
		{"javascript scheme", "javascript:alert(1)", "must use http or https scheme"},
		{"ftp scheme", "ftp://example.com/foo", "must use http or https scheme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			y := `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        queue_depth:
          threshold: 3
          prometheus_url: "` + tc.url + `"
`
			m, err := manifest.Load([]byte(y), map[string]string{})
			require.NoError(t, err)
			err = m.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestValidateRejectsMaxEqualsMinWithAutoscale(t *testing.T) {
	y := `services:
  svc:
    build: .
    scale:
      min: 3
      max: 3
      autoscale:
        queue_depth:
          threshold: 5
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale.max must be > scale.min when autoscale is enabled")
}

func TestValidateRejectsNegativeMax(t *testing.T) {
	y := `services:
  svc:
    build: .
    scale:
      max: -1
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale.max must be >= 0")
}

func TestValidateRejectsNegativeCountForm(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "bare int negative",
			yaml: `services:
  svc:
    build: .
    scale: -2
`,
		},
		{
			name: "count map negative min",
			yaml: `services:
  svc:
    build: .
    scale:
      count:
        min: -2
        max: 5
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := manifest.Load([]byte(tc.yaml), map[string]string{})
			require.NoError(t, err)
			err = m.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), "scale.min must be >= 0")
		})
	}
}

func TestValidateRejectsCronOnlyAtZero(t *testing.T) {
	y := `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        custom:
        - type: cron
          name: business-hours
          metadata:
            timezone: UTC
            start: 0 9 * * 1-5
            end: 0 17 * * 1-5
            desiredReplicas: "1"
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "always-active autoscale triggers")
}

func TestValidateReservedPrefixCaseInsensitive(t *testing.T) {
	y := `services:
  svc:
    build: .
    scale:
      min: 0
      max: 5
      autoscale:
        custom:
        - type: cron
          name: ok
          metadata: {timezone: UTC, start: "* * * * *", end: "* * * * *", desiredReplicas: "1"}
        - type: cron
          name: Convox-collision
          metadata: {timezone: UTC, start: "* * * * *", end: "* * * * *", desiredReplicas: "1"}
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "custom[1]", "error must reference the correct array index")
	require.Contains(t, err.Error(), "reserved prefix 'convox-'")
}

func TestValidateRejectsClusterTriggerAuthentication(t *testing.T) {
	y := `services:
  svc:
    build: .
    scale:
      min: 1
      max: 5
      autoscale:
        custom:
        - type: prometheus
          name: mine
          metadata: {serverAddress: "http://x/", metricName: y, threshold: "1", query: "up"}
          authenticationRef:
            name: tenant-cta
            kind: ClusterTriggerAuthentication
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "ClusterTriggerAuthentication is not permitted")
}

func TestYamlParseMinMaxAllowsInt64(t *testing.T) {
	// Smoke test: confirm our scale.min / scale.max bindings accept both
	// YAML-decoded int and int64 paths. yaml.v2 promotes unquoted numbers to
	// int when they fit and int64 otherwise; switching between versions of
	// the decoder would otherwise silently drop the field.
	y := `services:
  svc:
    build: .
    port: 3000
    scale:
      min: 0
      max: 10
      autoscale:
        queue_depth:
          threshold: 5
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	require.NotNil(t, m.Services[0].Scale.Min)
	require.NotNil(t, m.Services[0].Scale.Max)
	require.Equal(t, 0, *m.Services[0].Scale.Min)
	require.Equal(t, 10, *m.Services[0].Scale.Max)
}

func ptrInt(i int) *int { return &i }

func TestValidateRejectsAgentAutoscale(t *testing.T) {
	y := `services:
  collector:
    build: .
    agent: true
    scale:
      min: 0
      max: 5
      autoscale:
        queue_depth:
          threshold: 3
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent services render as DaemonSet")
}

func TestValidateRejectsAgentKeda(t *testing.T) {
	y := `services:
  collector:
    build: .
    agent: true
    scale:
      keda:
        triggers:
        - type: aws-sqs-queue
          metadata: {queueURL: "http://x/", queueLength: "1", awsRegion: "us-east-1"}
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent services render as DaemonSet")
}

func TestValidateCatchesMaxEqualsMinInCountForm(t *testing.T) {
	// Legacy scale.count form + autoscale used to bypass the max==min check
	// because validateServiceScale gated on pointer Min/Max only. Regression
	// guard: Count-form manifests must hit the same rule.
	y := `services:
  svc:
    build: .
    scale:
      count: 2-2
      autoscale:
        queue_depth:
          threshold: 3
`
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "scale.max must be > scale.min when autoscale is enabled")
}

func TestValidateRejectsUnicodeCustomName(t *testing.T) {
	y := "services:\n  svc:\n    build: .\n    scale:\n      min: 1\n      max: 3\n      autoscale:\n        custom:\n        - type: prometheus\n          name: \"Сonvox-cpu\"\n          metadata: {serverAddress: \"http://x/\", metricName: m, threshold: \"1\", query: \"up\"}\n"
	m, err := manifest.Load([]byte(y), map[string]string{})
	require.NoError(t, err)
	err = m.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "custom[0].name")
	require.Contains(t, err.Error(), "must contain only lowercase alphanumeric")
}
