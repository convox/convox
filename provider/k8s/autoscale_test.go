package k8s_test

import (
	"context"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

var testScaledObjectGVR = schema.GroupVersionResource{Group: "keda.sh", Version: "v1alpha1", Resource: "scaledobjects"}

func scaledObjectUnstructured(namespace, name string, minReplica, maxReplica int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"minReplicaCount": minReplica,
				"maxReplicaCount": maxReplica,
			},
		},
	}
}

func newDynamicScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(testScaledObjectGVR.GroupVersion().WithKind("ScaledObject"), &unstructured.Unstructured{})
	s.AddKnownTypeWithName(testScaledObjectGVR.GroupVersion().WithKind("ScaledObjectList"), &unstructured.UnstructuredList{})
	return s
}

func seedDeployment(t *testing.T, kk *kfake.Clientset, ns, name string, replicas int32) {
	t.Helper()
	r := replicas
	_, err := kk.AppsV1().Deployments(ns).Create(context.TODO(), &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": "app1", "type": "service", "service": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func TestServiceUpdateKedaSync(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 2)

		dyn := fake.NewSimpleDynamicClient(newDynamicScheme(), scaledObjectUnstructured("rack1-app1", "web", 1, 5))
		p.DynamicClient = dyn

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{
			Min: options.Int(0),
			Max: options.Int(10),
		})
		require.NoError(t, err)

		got, err := dyn.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		spec, _ := got.Object["spec"].(map[string]interface{})
		require.Equal(t, int64(0), toInt64(spec["minReplicaCount"]))
		require.Equal(t, int64(10), toInt64(spec["maxReplicaCount"]))

		// Verify patch strategy is JSON-merge (RFC 7396). Strategic-merge
		// would require server-side strategy metadata that CRDs don't register.
		var sawPatch bool
		for _, action := range dyn.Actions() {
			p, ok := action.(k8stesting.PatchAction)
			if !ok {
				continue
			}
			if p.GetResource() != testScaledObjectGVR {
				continue
			}
			require.Equal(t, ktypes.MergePatchType, p.GetPatchType(), "scaledobject patch must use JSON-merge")
			sawPatch = true
		}
		require.True(t, sawPatch, "expected at least one patch action on scaledobjects")
	})
}

func TestServiceUpdateDeadPodsGuard(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)

		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Maybe()

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(0)})
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "no autoscale mechanism is configured"), err.Error())
	})
}

func TestServiceUpdateMinAllowsNonZeroWithoutScaledObject(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(5)})
		require.NoError(t, err)
	})
}

func TestServiceUpdateCountPatchesScaledObject(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 2)

		dyn := fake.NewSimpleDynamicClient(newDynamicScheme(), scaledObjectUnstructured("rack1-app1", "web", 1, 5))
		p.DynamicClient = dyn

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(3)})
		require.NoError(t, err)

		got, err := dyn.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		spec, _ := got.Object["spec"].(map[string]interface{})
		require.Equal(t, int64(3), toInt64(spec["minReplicaCount"]))
		require.Equal(t, int64(3), toInt64(spec["maxReplicaCount"]))

		// Count-only path on a ScaledObject-owned service must not mutate the
		// Deployment's Replicas field — that would fight KEDA's reconciler.
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(2), *d.Spec.Replicas, "deployment replicas must stay at original value when ScaledObject owns replicas")
	})
}

func TestKedaScaledObjectFromAutoscale(t *testing.T) {
	svc := manifest.Service{Name: "vllm"}
	svc.Scale.Autoscale = &manifest.ServiceAutoscale{
		GpuUtilization: &manifest.AutoscaleThreshold{Threshold: 70},
	}
	triggers := svc.Scale.Autoscale.BuildTriggers("myapp", "vllm", "http://prom/")

	obj := svc.KedaScaledObject(manifest.KedaScaledObjectParameters{
		ServiceName: "vllm",
		Namespace:   "myapp-ns",
		MinCount:    0,
		MaxCount:    10,
		Triggers:    triggers,
	})
	require.NotNil(t, obj)

	data, err := k8s.SerializeK8sObjToYaml(obj)
	require.NoError(t, err)

	var parsed kedav1alpha1.ScaledObject
	require.NoError(t, yaml.Unmarshal(data, &parsed))
	require.Equal(t, "ScaledObject", parsed.TypeMeta.Kind)
	require.Equal(t, "keda.sh/v1alpha1", parsed.TypeMeta.APIVersion)
	require.Equal(t, int32(0), *parsed.Spec.MinReplicaCount)
	require.Equal(t, int32(10), *parsed.Spec.MaxReplicaCount)
	require.Len(t, parsed.Spec.Triggers, 1)
	require.Equal(t, "prometheus", parsed.Spec.Triggers[0].Type)
	require.Equal(t, "convox-gpu-utilization", parsed.Spec.Triggers[0].Name)
	require.Equal(t, "70", parsed.Spec.Triggers[0].Metadata["threshold"])
	require.Equal(t, "35", parsed.Spec.Triggers[0].Metadata["activationThreshold"])
	require.Equal(t, "vllm", parsed.Spec.ScaleTargetRef.Name)
	require.Equal(t, "Deployment", parsed.Spec.ScaleTargetRef.Kind)
}

func TestAwsAuthAttachFilter(t *testing.T) {
	t.Setenv("PROVIDER", "aws")
	svc := manifest.Service{Name: "worker"}
	svc.Scale.Keda = &manifest.ServiceScaleKeda{
		Triggers: []kedav1alpha1.ScaleTriggers{
			{Type: "aws-sqs-queue", Name: "raw-sqs", Metadata: map[string]string{"queueURL": "http://q/", "queueLength": "1", "awsRegion": "us-east-1"}},
			{Type: "prometheus", Name: "raw-prom", Metadata: map[string]string{"serverAddress": "http://p/", "metricName": "m", "threshold": "1", "query": "up"}},
			{Type: "cron", Name: "raw-cron", Metadata: map[string]string{"timezone": "UTC", "start": "* * * * *", "end": "* * * * *", "desiredReplicas": "1"}},
		},
	}

	obj := svc.KedaScaledObject(manifest.KedaScaledObjectParameters{
		ServiceName: "worker",
		Namespace:   "myapp-ns",
		MinCount:    0,
		MaxCount:    5,
		Triggers:    svc.Scale.Keda.Triggers,
	})
	require.NotNil(t, obj)
	require.Len(t, obj.Spec.Triggers, 3)

	var sqs, prom, cron *kedav1alpha1.ScaleTriggers
	for i := range obj.Spec.Triggers {
		switch obj.Spec.Triggers[i].Type {
		case "aws-sqs-queue":
			sqs = &obj.Spec.Triggers[i]
		case "prometheus":
			prom = &obj.Spec.Triggers[i]
		case "cron":
			cron = &obj.Spec.Triggers[i]
		}
	}
	require.NotNil(t, sqs)
	require.NotNil(t, prom)
	require.NotNil(t, cron)

	require.NotNil(t, sqs.AuthenticationRef, "aws-sqs-queue MUST get default AWS IRSA auth")
	require.Equal(t, "keda-aws-auth-default", sqs.AuthenticationRef.Name)

	require.Nil(t, prom.AuthenticationRef, "prometheus trigger MUST NOT inherit AWS auth")
	require.Nil(t, cron.AuthenticationRef, "cron trigger MUST NOT inherit AWS auth")
}

func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case float64:
		return int64(t)
	case int:
		return int64(t)
	}
	panic("unexpected type in toInt64")
}
