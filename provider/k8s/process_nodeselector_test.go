package k8s

import (
	"context"
	"sort"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// minimalProvider creates a Provider with fake clients and no informers.
// Informer-based lookups fall back to direct API calls against the fakes.
func minimalProvider(t *testing.T) (*Provider, *fake.Clientset, *cvfake.Clientset) {
	t.Helper()
	kk := fake.NewSimpleClientset()
	kc := cvfake.NewSimpleClientset()
	mc := metricfake.NewSimpleClientset()
	aa := &atom.MockInterface{}
	aa.On("Status", "rack1-app1", "app").Return("Running", "rel1", nil)

	p := &Provider{
		Atom:          aa,
		Cluster:       kk,
		Convox:        kc,
		Engine:        &mock.TestEngine{},
		MetricsClient: mc,
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
		logger:        logger.New("ns=k8s-test"),
	}

	return p, kk, kc
}

func createAppNamespace(t *testing.T, kk *fake.Clientset, rack, app string) {
	t.Helper()
	_, err := kk.CoreV1().Namespaces().Create(context.TODO(), &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name:        rack + "-" + app,
			Annotations: map[string]string{"convox.com/lock": "false"},
			Labels: map[string]string{
				"app":    app,
				"name":   app,
				"rack":   rack,
				"system": "convox",
				"type":   "app",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func createBuild(t *testing.T, kc *cvfake.Clientset, ns, id string) {
	t.Helper()
	_, err := kc.ConvoxV1().Builds(ns).Create(&ca.Build{
		ObjectMeta: am.ObjectMeta{
			Name:   id,
			Labels: map[string]string{"app": "app1"},
		},
		Spec: ca.BuildSpec{
			Description: "test build",
			Started:     "20200101.000000.000000000",
			Ended:       "20200101.000000.000000000",
			Manifest:    "services:\n  web:\n    build: .\n",
		},
	})
	require.NoError(t, err)
}

func createRelease(t *testing.T, kc *cvfake.Clientset, ns, id, manifest string) {
	t.Helper()
	_, err := kc.ConvoxV1().Releases(ns).Create(&ca.Release{
		ObjectMeta: am.ObjectMeta{Name: id},
		Spec: ca.ReleaseSpec{
			Build:    "build1",
			Created:  "20200101.000000.000000000",
			Manifest: manifest,
		},
	})
	require.NoError(t, err)
}

// runAndGetPodSpec calls ProcessRun then retrieves the pod from the fake clientset.
func runAndGetPodSpec(t *testing.T, p *Provider, kk *fake.Clientset, app, service string, opts structs.ProcessRunOptions) *ac.PodSpec {
	t.Helper()
	ps, err := p.ProcessRun(app, service, opts)
	require.NoError(t, err)
	require.NotNil(t, ps)

	pod, err := kk.CoreV1().Pods(p.AppNamespace(app)).Get(context.TODO(), ps.Id, am.GetOptions{})
	require.NoError(t, err)
	return &pod.Spec
}

const manifestWithNodeSelectors = `services:
  web:
    build: .
    port: 5000
  gpu-worker:
    build: .
    nodeSelectorLabels:
      convox.io/nodepool: gpu
  labeled-worker:
    build: .
    nodeSelectorLabels:
      convox.io/label: special
  custom-label-worker:
    build: .
    nodeSelectorLabels:
      team: ml
`

func TestProcessRun_InheritsNodeSelectorLabelsAffinity(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "gpu-worker", structs.ProcessRunOptions{
		Release: options.String("rel1"),
	})

	// Should have required node affinity for convox.io/nodepool=gpu
	require.NotNil(t, spec.Affinity, "expected Affinity to be set")
	require.NotNil(t, spec.Affinity.NodeAffinity)
	req := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	require.NotNil(t, req)
	require.Len(t, req.NodeSelectorTerms, 1)
	require.Len(t, req.NodeSelectorTerms[0].MatchExpressions, 1)
	require.Equal(t, "convox.io/nodepool", req.NodeSelectorTerms[0].MatchExpressions[0].Key)
	require.Equal(t, ac.NodeSelectorOpIn, req.NodeSelectorTerms[0].MatchExpressions[0].Operator)
	require.Equal(t, []string{"gpu"}, req.NodeSelectorTerms[0].MatchExpressions[0].Values)

	// Should have dedicated-node toleration (Equal operator, exact value)
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	require.Equal(t, ac.TolerationOpEqual, spec.Tolerations[0].Operator)
	require.Equal(t, "gpu", spec.Tolerations[0].Value)
	require.Equal(t, ac.TaintEffectNoSchedule, spec.Tolerations[0].Effect)
}

func TestProcessRun_InheritsConvoxLabelToleration(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "labeled-worker", structs.ProcessRunOptions{
		Release: options.String("rel1"),
	})

	// convox.io/label should also trigger the dedicated-node toleration
	require.NotNil(t, spec.Affinity)
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	require.Equal(t, ac.TolerationOpEqual, spec.Tolerations[0].Operator)
	require.Equal(t, "special", spec.Tolerations[0].Value)
}

func TestProcessRun_CustomLabelGetsAffinityButNoToleration(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "custom-label-worker", structs.ProcessRunOptions{
		Release: options.String("rel1"),
	})

	// Custom labels should get affinity
	require.NotNil(t, spec.Affinity)
	require.NotNil(t, spec.Affinity.NodeAffinity)
	terms := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	require.Len(t, terms, 1)
	require.Equal(t, "team", terms[0].MatchExpressions[0].Key)
	require.Equal(t, []string{"ml"}, terms[0].MatchExpressions[0].Values)

	// But NOT dedicated-node tolerations (only convox.io/label and convox.io/nodepool trigger those)
	require.Empty(t, spec.Tolerations)
}

func TestProcessRun_NoNodeSelectorLabelsNoAffinityNoTolerations(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release: options.String("rel1"),
	})

	// Service without nodeSelectorLabels: no affinity, no tolerations
	require.Nil(t, spec.Affinity)
	require.Empty(t, spec.Tolerations)
}

func TestProcessRun_NodeLabelsOverridesInheritedAffinity(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "gpu-worker", structs.ProcessRunOptions{
		Release:    options.String("rel1"),
		NodeLabels: options.String("custom-pool=debug"),
	})

	// --node-labels should clear inherited affinity and use nodeSelector instead
	require.Nil(t, spec.Affinity, "explicit --node-labels should clear inherited Affinity")
	require.Equal(t, map[string]string{"custom-pool": "debug"}, spec.NodeSelector)

	// Should have broad dedicated-node toleration (Exists operator, not Equal)
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	require.Equal(t, ac.TolerationOpExists, spec.Tolerations[0].Operator)
	require.Equal(t, ac.TaintEffectNoSchedule, spec.Tolerations[0].Effect)
}

func TestProcessRun_NodeLabelsOverridesForServiceWithoutNodeSelector(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "web", structs.ProcessRunOptions{
		Release:    options.String("rel1"),
		NodeLabels: options.String("convox.io/nodepool=other"),
	})

	// --node-labels on a service without nodeSelectorLabels should still work
	require.Nil(t, spec.Affinity)
	require.Equal(t, map[string]string{"convox.io/nodepool": "other"}, spec.NodeSelector)
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	require.Equal(t, ac.TolerationOpExists, spec.Tolerations[0].Operator)
}

func TestProcessRun_MultipleNodeSelectorLabels(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")

	manifest := `services:
  multi:
    build: .
    nodeSelectorLabels:
      convox.io/nodepool: gpu
      team: ml
`
	createRelease(t, kc, "rack1-app1", "rel1", manifest)

	spec := runAndGetPodSpec(t, p, kk, "app1", "multi", structs.ProcessRunOptions{
		Release: options.String("rel1"),
	})

	// Should have affinity with both match expressions
	require.NotNil(t, spec.Affinity)
	terms := spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	require.Len(t, terms, 1)
	require.Len(t, terms[0].MatchExpressions, 2)

	// Sort for deterministic comparison (Go map iteration is non-deterministic)
	exprs := terms[0].MatchExpressions
	sort.Slice(exprs, func(i, j int) bool { return exprs[i].Key < exprs[j].Key })
	require.Equal(t, "convox.io/nodepool", exprs[0].Key)
	require.Equal(t, []string{"gpu"}, exprs[0].Values)
	require.Equal(t, "team", exprs[1].Key)
	require.Equal(t, []string{"ml"}, exprs[1].Values)

	// Only convox.io/nodepool triggers a toleration, not "team"
	require.Len(t, spec.Tolerations, 1)
	require.Equal(t, "dedicated-node", spec.Tolerations[0].Key)
	require.Equal(t, "gpu", spec.Tolerations[0].Value)
}

func TestProcessRun_BuildIgnoresNodeSelectorLabels(t *testing.T) {
	p, kk, kc := minimalProvider(t)
	createAppNamespace(t, kk, "rack1", "app1")
	createBuild(t, kc, "rack1-app1", "build1")
	createRelease(t, kc, "rack1-app1", "rel1", manifestWithNodeSelectors)

	spec := runAndGetPodSpec(t, p, kk, "app1", "gpu-worker", structs.ProcessRunOptions{
		Release: options.String("rel1"),
		IsBuild: true,
	})

	// Build pods should NOT inherit nodeSelectorLabels affinity
	// (the isBuild check in podSpecFromService skips the manifest lookup)
	require.Nil(t, spec.Affinity)
	require.Empty(t, spec.Tolerations)
}
