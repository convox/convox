package k8s

import (
	"context"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

func TestAppBuildArch(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"amd64", "amd64"},
		{"arm64", "arm64"},
		{"", ""},
		{"x86", ""},
		{"AMD64", ""},
		{"aarch64", ""},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := appBuildArch(map[string]string{structs.AppParamBuildArch: tc.in})
			require.Equal(t, tc.want, got)
		})
	}
	require.Equal(t, "", appBuildArch(map[string]string{}))
}

func TestAppParametersRegistersBuildArch(t *testing.T) {
	// Regression: PR #964 defined AppParamBuildArch but never registered it
	// here, so AppGet stripped it and apps params set rejected it, leaving the
	// whole BuildArch feature inert.
	_, ok := (&Provider{}).AppParameters()[structs.AppParamBuildArch]
	require.True(t, ok, "BuildArch must be a recognized app parameter")
}

func archExpr(exprs []ac.NodeSelectorRequirement) *ac.NodeSelectorRequirement {
	for i := range exprs {
		if exprs[i].Key == "kubernetes.io/arch" {
			return &exprs[i]
		}
	}
	return nil
}

func deploymentDoc(t *testing.T, rendered string) *appsv1.Deployment {
	t.Helper()
	for _, doc := range strings.Split(rendered, "\n---\n") {
		var meta struct {
			Kind string `json:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
			continue
		}
		if strings.TrimSpace(meta.Kind) != "Deployment" {
			continue
		}
		var d appsv1.Deployment
		require.NoError(t, yaml.Unmarshal([]byte(doc), &d))
		return &d
	}
	t.Fatalf("no Deployment doc in render:\n%s", rendered)
	return nil
}

func cronJobDoc(t *testing.T, rendered string) *batchv1.CronJob {
	t.Helper()
	for _, doc := range strings.Split(rendered, "\n---\n") {
		var meta struct {
			Kind string `json:"kind"`
		}
		if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
			continue
		}
		if strings.TrimSpace(meta.Kind) != "CronJob" {
			continue
		}
		var c batchv1.CronJob
		require.NoError(t, yaml.Unmarshal([]byte(doc), &c))
		return &c
	}
	t.Fatalf("no CronJob doc in render:\n%s", rendered)
	return nil
}

func TestRenderServiceBuildArchPin(t *testing.T) {
	base := `services:
  web:
    build: .
    port: 3000
`

	t.Run("arch set adds required kubernetes.io/arch nodeSelector", func(t *testing.T) {
		p, params := gpuTemplateFixture(t, base)
		params["BuildArch"] = "arm64"
		data, err := p.RenderTemplate("app/service", params)
		require.NoError(t, err)

		d := deploymentDoc(t, string(data))
		req := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.NotNil(t, req)
		require.Len(t, req.NodeSelectorTerms, 1)
		ae := archExpr(req.NodeSelectorTerms[0].MatchExpressions)
		require.NotNil(t, ae)
		require.Equal(t, ac.NodeSelectorOpIn, ae.Operator)
		require.Equal(t, []string{"arm64"}, ae.Values)
	})

	t.Run("arch unset renders no affinity (no behavior change)", func(t *testing.T) {
		p, params := gpuTemplateFixture(t, base)
		data, err := p.RenderTemplate("app/service", params)
		require.NoError(t, err)
		out := string(data)
		require.NotContains(t, out, "kubernetes.io/arch")
		require.NotContains(t, out, "affinity:")
	})

	t.Run("arch merges with nodeSelectorLabels in one term", func(t *testing.T) {
		src := `services:
  web:
    build: .
    port: 3000
    nodeSelectorLabels:
      convox.io/nodepool: gpu
`
		p, params := gpuTemplateFixture(t, src)
		params["BuildArch"] = "amd64"
		data, err := p.RenderTemplate("app/service", params)
		require.NoError(t, err)

		d := deploymentDoc(t, string(data))
		req := d.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.NotNil(t, req)
		require.Len(t, req.NodeSelectorTerms, 1, "arch must merge into the existing term, not add a new one")

		exprs := req.NodeSelectorTerms[0].MatchExpressions
		require.Len(t, exprs, 2)
		ae := archExpr(exprs)
		require.NotNil(t, ae)
		require.Equal(t, []string{"amd64"}, ae.Values)

		// The dedicated-node toleration from convox.io/nodepool must survive.
		require.Contains(t, string(data), "key: dedicated-node")
	})
}

func TestRenderTimerBuildArchPin(t *testing.T) {
	base := `services:
  worker:
    build: .
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`

	t.Run("arch set pins the cronjob pod", func(t *testing.T) {
		p, params := gpuTemplateFixture(t, base)
		params["BuildArch"] = "arm64"
		data, err := p.RenderTemplate("app/timer", params)
		require.NoError(t, err)

		c := cronJobDoc(t, string(data))
		req := c.Spec.JobTemplate.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.NotNil(t, req)
		require.Len(t, req.NodeSelectorTerms, 1)
		ae := archExpr(req.NodeSelectorTerms[0].MatchExpressions)
		require.NotNil(t, ae)
		require.Equal(t, []string{"arm64"}, ae.Values)
	})

	t.Run("arch unset renders no affinity", func(t *testing.T) {
		p, params := gpuTemplateFixture(t, base)
		data, err := p.RenderTemplate("app/timer", params)
		require.NoError(t, err)
		require.NotContains(t, string(data), "kubernetes.io/arch")
	})

	t.Run("arch merges with nodeSelectorLabels", func(t *testing.T) {
		src := `services:
  worker:
    build: .
    nodeSelectorLabels:
      team: ml
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`
		p, params := gpuTemplateFixture(t, src)
		params["BuildArch"] = "amd64"
		data, err := p.RenderTemplate("app/timer", params)
		require.NoError(t, err)

		c := cronJobDoc(t, string(data))
		exprs := c.Spec.JobTemplate.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions
		require.Len(t, exprs, 2)
		require.NotNil(t, archExpr(exprs))
	})
}

type archEngine struct {
	*mock.TestEngine
}

func (archEngine) AppParameters() map[string]string {
	return map[string]string{
		structs.AppParamBuildArch:   "",
		structs.AppParamBuildCpu:    "",
		structs.AppParamBuildMem:    "",
		structs.AppParamBuildLabels: "",
	}
}

func createAppNamespaceWithParams(t *testing.T, kk *fake.Clientset, rack, app, paramsJSON string) {
	t.Helper()
	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: rack + "-" + app,
			Annotations: map[string]string{
				"convox.com/lock":   "false",
				"convox.com/params": paramsJSON,
			},
			Labels: map[string]string{
				"app": app, "name": app, "rack": rack, "system": "convox", "type": "app",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func TestAppGetPreservesBuildArch(t *testing.T) {
	p, kk, _ := minimalProvider(t)
	p.Engine = archEngine{&mock.TestEngine{}}
	createAppNamespaceWithParams(t, kk, "rack1", "app1", `{"BuildArch":"arm64"}`)

	a, err := p.AppGet("app1")
	require.NoError(t, err)
	require.Equal(t, "arm64", a.Parameters[structs.AppParamBuildArch])
}

func TestProcessRun_BuildArchPinsRuntimePod(t *testing.T) {
	manifest := `services:
  web:
    build: .
    port: 5000
  gpu-worker:
    build: .
    nodeSelectorLabels:
      convox.io/nodepool: gpu
`

	setup := func(t *testing.T, paramsJSON string) (*Provider, *fake.Clientset) {
		p, kk, kc := minimalProvider(t)
		p.Engine = archEngine{&mock.TestEngine{}}
		createAppNamespaceWithParams(t, kk, "rack1", "app1", paramsJSON)
		createBuild(t, kc, "rack1-app1", "build1")
		createRelease(t, kc, "rack1-app1", "rel1", manifest)
		return p, kk
	}

	archReq := func(t *testing.T, spec *ac.PodSpec) *ac.NodeSelectorRequirement {
		t.Helper()
		require.NotNil(t, spec.Affinity)
		require.NotNil(t, spec.Affinity.NodeAffinity)
		req := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		require.NotNil(t, req)
		require.Len(t, req.NodeSelectorTerms, 1)
		return archExpr(req.NodeSelectorTerms[0].MatchExpressions)
	}

	t.Run("arch-only app pins run pod, no toleration", func(t *testing.T) {
		p, kk := setup(t, `{"BuildArch":"arm64"}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{Release: options.String("rel1")})
		ae := archReq(t, spec)
		require.NotNil(t, ae)
		require.Equal(t, []string{"arm64"}, ae.Values)
		require.Empty(t, spec.Tolerations)
	})

	t.Run("arch merges with nodeSelectorLabels and keeps toleration", func(t *testing.T) {
		p, kk := setup(t, `{"BuildArch":"amd64"}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "gpu-worker", structs.ProcessRunOptions{Release: options.String("rel1")})
		exprs := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions
		require.Len(t, exprs, 2)
		require.Equal(t, []string{"amd64"}, archExpr(exprs).Values)
		require.Len(t, spec.Tolerations, 1)
		require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	})

	t.Run("no BuildArch means no arch pin", func(t *testing.T) {
		p, kk := setup(t, `{}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{Release: options.String("rel1")})
		require.Nil(t, spec.Affinity)
	})

	t.Run("invalid BuildArch is ignored", func(t *testing.T) {
		p, kk := setup(t, `{"BuildArch":"x86"}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{Release: options.String("rel1")})
		require.Nil(t, spec.Affinity)
	})

	t.Run("build pods do not inherit the app arch param", func(t *testing.T) {
		p, kk := setup(t, `{"BuildArch":"arm64"}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{
			Release: options.String("rel1"),
			IsBuild: true,
		})
		require.Nil(t, spec.Affinity)
	})

	t.Run("--node-labels override keeps the arch guard via nodeSelector", func(t *testing.T) {
		p, kk := setup(t, `{"BuildArch":"arm64"}`)
		spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{
			Release:    options.String("rel1"),
			NodeLabels: options.String("team=ml"),
		})
		require.Nil(t, spec.Affinity)
		require.Equal(t, "ml", spec.NodeSelector["team"])
		require.Equal(t, "arm64", spec.NodeSelector["kubernetes.io/arch"])
	})
}

func TestReleaseTemplateTimerBuildArchWiring(t *testing.T) {
	p, _, _ := minimalProvider(t)
	p.templater = templater.New(template.TemplatesFS)
	src := `services:
  worker:
    build: .
timers:
  nightly:
    schedule: "0 0 * * ? *"
    command: "echo hi"
    service: worker
`
	m, err := manifest.Load([]byte(src), map[string]string{})
	require.NoError(t, err)
	svc := &m.Services[0]
	tm := m.Timers[0]
	r := &structs.Release{Id: "r1"}

	render := func(arch string) string {
		a := &structs.App{Name: "app1", Parameters: map[string]string{structs.AppParamBuildArch: arch}}
		data, err := p.releaseTemplateTimer(a, structs.Environment{}, r, svc, tm)
		require.NoError(t, err)
		return string(data)
	}

	require.Contains(t, render("arm64"), "kubernetes.io/arch")
	require.Contains(t, render("arm64"), "arm64")
	require.NotContains(t, render("x86"), "kubernetes.io/arch")
	require.NotContains(t, render(""), "kubernetes.io/arch")
}
