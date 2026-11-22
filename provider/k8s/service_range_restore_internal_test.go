package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/atom"
	cxmock "github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	metricfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestServiceUpdateRangeRestoreLeavesInformerCacheAlone(t *testing.T) {
	t.Setenv("TEST", "true")

	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      "web",
			Namespace: "rack1-app1",
			Annotations: map[string]string{
				ServiceTriggersOverrideAnnotation:    ServiceTriggersOverrideValueOn,
				ServiceTriggersOverrideCRDAnnotation: TriggersCRDHPA,
			},
		},
		Spec: appsv1.DeploymentSpec{Replicas: &zero},
	}

	c := fake.NewSimpleClientset(
		&ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "ns1"}},
		&ac.Namespace{ObjectMeta: am.ObjectMeta{Name: "rack1-app1"}},
		dep.DeepCopy(),
	)

	p := &Provider{
		Atom:          &atom.MockInterface{},
		Cluster:       c,
		Convox:        cvfake.NewSimpleClientset(),
		MetricsClient: metricfake.NewSimpleClientset(),
		Engine:        &cxmock.TestEngine{},
		Name:          "rack1",
		Namespace:     "ns1",
		Provider:      "test",
	}
	require.NoError(t, p.Initialize(structs.ProviderOptions{}))
	p.DynamicClient = dynfake.NewSimpleDynamicClient(runtime.NewScheme())

	di := informers.NewSharedInformerFactory(c, 0).Apps().V1().Deployments()
	require.NoError(t, di.Informer().GetIndexer().Add(dep))
	p.deploymentInformer = di

	err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)})
	require.ErrorContains(t, err, "no HPA found")

	live, err := c.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, int32(0), *live.Spec.Replicas)

	cached, err := di.Lister().Deployments("rack1-app1").Get("web")
	require.NoError(t, err)
	require.Equal(t, int32(0), *cached.Spec.Replicas)
}
