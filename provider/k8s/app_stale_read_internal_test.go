package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/atom"
	cxmock "github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// A namespace lister that is never synced, so it holds whatever the test seeded
// it with while the fake clientset moves on. This is what the rack API sees in
// the window between a write and the watch event that carries it.
type frozenNamespaceInformer struct {
	lister listersv1.NamespaceLister
}

func (f *frozenNamespaceInformer) Informer() cache.SharedIndexInformer {
	return nil
}

func (f *frozenNamespaceInformer) Lister() listersv1.NamespaceLister {
	return f.lister
}

func appNamespaceObject(params string, locked bool) *ac.Namespace {
	lock := "false"
	if locked {
		lock = "true"
	}

	return &ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: "rack1-app1",
			Annotations: map[string]string{
				"convox.com/lock":   lock,
				"convox.com/params": params,
			},
			Labels: map[string]string{
				"app":    "app1",
				"name":   "app1",
				"rack":   "rack1",
				"system": "convox",
				"type":   "app",
			},
		},
	}
}

// live is what the API server holds, frozen is what the informer still reports.
func staleReadProvider(t *testing.T, live, frozen *ac.Namespace) (*Provider, *atom.MockInterface) {
	t.Helper()
	t.Setenv("TEST", "true")

	c := fake.NewSimpleClientset()

	for _, ns := range []*ac.Namespace{
		{ObjectMeta: am.ObjectMeta{Name: "ns1", UID: "uid1"}},
		live,
	} {
		_, err := c.CoreV1().Namespaces().Create(context.TODO(), ns, am.CreateOptions{})
		require.NoError(t, err)
	}

	a := &atom.MockInterface{}

	p := &Provider{
		Atom:          a,
		Cluster:       c,
		Convox:        cvfake.NewSimpleClientset(),
		MetricsClient: metricfake.NewSimpleClientset(),
		Engine:        &cxmock.TestEngine{},
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
	}

	require.NoError(t, p.Initialize(structs.ProviderOptions{}))

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(frozen))
	p.namespaceInformer = &frozenNamespaceInformer{lister: listersv1.NewNamespaceLister(indexer)}

	a.On("Status", "rack1-app1", "app").Return("Running", "", nil)

	return p, a
}

func capturePromoteTemplate(t *testing.T, a *atom.MockInterface) *string {
	t.Helper()

	var rendered string

	a.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		cfg, ok := args.Get(2).(*atom.ApplyConfig)
		require.True(t, ok)
		rendered = string(cfg.Template)
	})

	return &rendered
}

func liveParams(t *testing.T, p *Provider) string {
	t.Helper()

	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
	require.NoError(t, err)

	return ns.Annotations["convox.com/params"]
}

func TestAppUpdateParametersSurviveStaleInformer(t *testing.T) {
	p, a := staleReadProvider(t,
		appNamespaceObject(`{"Test":"foo"}`, false),
		appNamespaceObject(`{"Test":"foo"}`, false),
	)
	rendered := capturePromoteTemplate(t, a)

	require.NoError(t, p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "bar"}}))

	require.Equal(t, `{"Test":"bar"}`, liveParams(t, p))
	require.Contains(t, *rendered, `{"Test":"bar"}`)
	require.NotContains(t, *rendered, `{"Test":"foo"}`)
}

func TestAppUpdateLockSurvivesStaleInformer(t *testing.T) {
	p, a := staleReadProvider(t,
		appNamespaceObject(`{"Test":"foo"}`, false),
		appNamespaceObject(`{"Test":"foo"}`, false),
	)
	rendered := capturePromoteTemplate(t, a)

	require.NoError(t, p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)}))

	ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "true", ns.Annotations["convox.com/lock"])
	require.Contains(t, *rendered, `convox.com/lock: "true"`)
}

func TestAppUpdateKeepsParameterSetSinceInformerSnapshot(t *testing.T) {
	p, a := staleReadProvider(t,
		appNamespaceObject(`{"Test":"bar","BuildArch":"arm64"}`, false),
		appNamespaceObject(`{"Test":"foo"}`, false),
	)
	capturePromoteTemplate(t, a)

	require.NoError(t, p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "baz"}}))

	params := liveParams(t, p)
	require.Contains(t, params, `"BuildArch":"arm64"`)
	require.Contains(t, params, `"Test":"baz"`)
}

func TestAppDeleteHonoursLockSetSinceInformerSnapshot(t *testing.T) {
	p, _ := staleReadProvider(t,
		appNamespaceObject(`{"Test":"foo"}`, true),
		appNamespaceObject(`{"Test":"foo"}`, false),
	)

	err := p.AppDelete("app1")
	require.EqualError(t, err, "app is locked: app1")

	_, err = p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
	require.NoError(t, err)
}

func TestBuildCreateReadsParametersSetSinceInformerSnapshot(t *testing.T) {
	p, _ := staleReadProvider(t,
		appNamespaceObject(`{"BuildArch":"arm64"}`, false),
		appNamespaceObject(`{}`, false),
	)

	_, err := p.BuildCreate("app1", "object://app1/object.tgz", structs.BuildCreateOptions{})
	require.NoError(t, err)

	pods, err := p.Cluster.CoreV1().Pods("rack1-app1").List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Len(t, pods.Items, 1)

	envs := map[string]string{}
	for _, e := range pods.Items[0].Spec.Containers[0].Env {
		envs[e.Name] = e.Value
	}
	require.Equal(t, "arm64", envs["BUILD_ARCHS"])
}
