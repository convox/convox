package k8s

import (
	"context"
	"errors"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/structs"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	tmock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var manualPauseAnnotations = map[string]string{
	structs.KedaPausedReplicasAnnotation: "0",
	structs.KedaManualPauseAnnotation:    "true",
}

func pausePromoteProvider(t *testing.T, applyErr error) (*Provider, *[]byte) {
	t.Helper()
	p := releasePromoteNodePoolProvider(t, "rel2")
	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)
	kc, ok := p.Convox.(*cvfake.Clientset)
	require.True(t, ok)
	createRelease(t, kc, "rack1-app1", "rel3", "services:\n  web:\n    build: .\n    port: 5000\n")

	zero := int32(0)
	_, err := kk.AppsV1().Deployments("rack1-app1").Create(context.TODO(), &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1", Labels: map[string]string{"app": "app1", "type": "service", "service": "web"}},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zero,
			Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	so := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "keda.sh/v1alpha1",
		"kind":       "ScaledObject",
		"metadata":   map[string]interface{}{"name": "web", "namespace": "rack1-app1"},
		"spec":       map[string]interface{}{"minReplicaCount": int64(1), "maxReplicaCount": int64(3)},
	}}
	so.SetAnnotations(manualPauseAnnotations)
	p.DynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		nodePoolGVR:     "NodePoolList",
		scaledObjectGVR: "ScaledObjectList",
	}, nodePoolObject("workload", "workload"), so)
	p.IsKedaEnabled = true

	var template []byte
	aa := &atom.MockInterface{}
	aa.On("Status", "rack1-app1", "app").Return("Running", "rel1", nil).Maybe()
	aa.On("Apply", "rack1-app1", "app", tmock.Anything).Run(func(args tmock.Arguments) {
		if cfg, ok := args.Get(2).(*atom.ApplyConfig); ok {
			template = cfg.Template
		}
	}).Return(applyErr)
	p.Atom = aa

	return p, &template
}

func pausePromoteAnnotations(t *testing.T, p *Provider) map[string]string {
	t.Helper()
	so, err := p.DynamicClient.Resource(scaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
	require.NoError(t, err)
	return so.GetAnnotations()
}

func TestReleasePromoteKeepsManualPauseWhenApplyFails(t *testing.T) {
	p, _ := pausePromoteProvider(t, errors.New("apply failed"))

	require.ErrorContains(t, p.ReleasePromote("app1", "rel3", structs.ReleasePromoteOptions{}), "apply failed")

	require.Equal(t, manualPauseAnnotations, pausePromoteAnnotations(t, p))
}

func TestReleasePromoteOfCurrentReleaseKeepsManualPause(t *testing.T) {
	p, template := pausePromoteProvider(t, nil)

	require.NoError(t, p.ReleasePromote("app1", "rel1", structs.ReleasePromoteOptions{}))

	require.Equal(t, manualPauseAnnotations, pausePromoteAnnotations(t, p))
	require.Equal(t, int32(0), RenderedReplicasForTest(t, *template, "web"))
}

func TestReleasePromoteOfCurrentReleaseInOtherCaseKeepsManualPause(t *testing.T) {
	p, template := pausePromoteProvider(t, nil)
	kk, ok := p.Cluster.(*fake.Clientset)
	require.True(t, ok)
	setAppRelease(t, kk, "rack1-app1", "rel3")

	require.NoError(t, p.ReleasePromote("app1", "REL3", structs.ReleasePromoteOptions{}))

	require.Equal(t, manualPauseAnnotations, pausePromoteAnnotations(t, p))
	require.Equal(t, int32(0), RenderedReplicasForTest(t, *template, "web"))
}

func TestPromoteKeepsPauseWhenScaledObjectRead(t *testing.T) {
	cases := []struct {
		name   string
		getErr error
		want   bool
	}{
		{"read error", kerr.NewInternalError(errors.New("etcd timeout")), true},
		{"not found", kerr.NewNotFound(schema.GroupResource{Group: "keda.sh", Resource: "scaledobjects"}, "web"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := pausePromoteProvider(t, nil)
			dyn, ok := p.DynamicClient.(*dynamicfake.FakeDynamicClient)
			require.True(t, ok)
			dyn.PrependReactor("get", "scaledobjects", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, tc.getErr
			})

			got := p.promoteKeepsPause(&structs.App{Name: "app1", Release: "rel1"}, &structs.Release{Id: "REL1", App: "app1"}, "web")

			require.Equal(t, tc.want, got)
		})
	}
}

func TestReleasePromoteOfNewReleaseResumesManualPause(t *testing.T) {
	p, template := pausePromoteProvider(t, nil)

	require.NoError(t, p.ReleasePromote("app1", "rel3", structs.ReleasePromoteOptions{}))

	require.Empty(t, pausePromoteAnnotations(t, p))
	require.Equal(t, int32(1), RenderedReplicasForTest(t, *template, "web"))
}
