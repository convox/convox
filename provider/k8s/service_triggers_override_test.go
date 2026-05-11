package k8s_test

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

// installManifestServiceHook injects a deterministic manifest service
// definition into the Provider so GPU preflight does not have to spin
// up the full release-storage machinery. Returns a *manifest.Service
// whose Scale.Gpu.Count matches the supplied gpuCount.
func installManifestServiceHook(p *k8s.Provider, app, service string, gpuCount int) {
	p.TriggersOverrideManifestServiceHook = func(a, s string) (*manifest.Service, error) {
		if a != app || s != service {
			return nil, structs.ErrNotFound("service not found: %s/%s", a, s)
		}
		ms := &manifest.Service{Name: service}
		ms.Scale.Gpu.Count = gpuCount
		return ms, nil
	}
}

func TestTriggersCRDChoice(t *testing.T) {
	cases := []struct {
		name string
		in   []structs.TriggerSpec
		want string
	}{
		{"cpu only", []structs.TriggerSpec{{Type: "cpu", Threshold: 70}}, "hpa"},
		{"memory only", []structs.TriggerSpec{{Type: "memory", Threshold: 80}}, "hpa"},
		{"cpu + memory", []structs.TriggerSpec{{Type: "cpu", Threshold: 70}, {Type: "memory", Threshold: 80}}, "hpa"},
		{"gpu only", []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}}, "keda"},
		{"queue only", []structs.TriggerSpec{{Type: "queueDepth", Threshold: 100}}, "keda"},
		{"cpu + gpu", []structs.TriggerSpec{{Type: "cpu", Threshold: 70}, {Type: "gpuUtilization", Threshold: 75}}, "keda"},
		{"cpu + queue", []structs.TriggerSpec{{Type: "cpu", Threshold: 70}, {Type: "queueDepth", Threshold: 50}}, "keda"},
		{"empty", []structs.TriggerSpec{}, "hpa"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, k8s.TriggersCRDChoiceForTest(tc.in))
		})
	}
}

func TestServiceTriggersDisable_NoOverride_Idempotent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err := p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err, "disable on no-override service must succeed")
	})
}

func TestServiceTriggersDisable_HPAPath(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 1)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
		}
		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "HPA must be deleted")

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		_, hasActive := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		_, hasCRD := dep.Annotations[k8s.ServiceTriggersOverrideCRDAnnotation]
		require.False(t, hasActive, "active annotation must be cleared")
		require.False(t, hasCRD, "crd annotation must be cleared")
	})
}

func TestServiceTriggersEnable_HPAPath_CPUOnly(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice@example.com")
		require.NoError(t, err)

		hpa, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(1), *hpa.Spec.MinReplicas)
		require.Equal(t, int32(5), hpa.Spec.MaxReplicas)
		require.Len(t, hpa.Spec.Metrics, 1)
		require.Equal(t, autoscalingv2.ResourceMetricSourceType, hpa.Spec.Metrics[0].Type)
		require.Equal(t, "cpu", string(hpa.Spec.Metrics[0].Resource.Name))
		require.Equal(t, int32(70), *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization)
		require.Equal(t, "Deployment", hpa.Spec.ScaleTargetRef.Kind)
		require.Equal(t, "web", hpa.Spec.ScaleTargetRef.Name)

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, "true", dep.Annotations[k8s.ServiceTriggersOverrideAnnotation])
		require.Equal(t, "hpa", dep.Annotations[k8s.ServiceTriggersOverrideCRDAnnotation])
	})
}

func TestServiceTriggersEnable_HPAPath_CPUAndMemory(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 2, Max: 10,
			Triggers: []structs.TriggerSpec{
				{Type: "cpu", Threshold: 70},
				{Type: "memory", Threshold: 80},
			},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		hpa, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(2), *hpa.Spec.MinReplicas)
		require.Equal(t, int32(10), hpa.Spec.MaxReplicas)
		require.Len(t, hpa.Spec.Metrics, 2)
	})
}

func TestServiceTriggersEnable_NoTriggers_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err := p.ServiceTriggersEnable("app1", "web", structs.ServiceTriggersOptions{Min: 1, Max: 5}, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "at least one trigger")
	})
}

func TestServiceTriggersEnable_MaxLessThanMin_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 5, Max: 1,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "max must be >= min")
	})
}

func TestServiceTriggersEnable_HPAPath_Idempotent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		opts.Triggers[0].Threshold = 80
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		hpa, _ := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, int32(80), *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization)
	})
}

func TestServiceTriggersEnable_KedaPath_GPU(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		obj, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers")
		require.Len(t, triggers, 1)
		tr := triggers[0].(map[string]interface{})
		require.Equal(t, "prometheus", tr["type"])
		require.Equal(t, "convox-gpu-utilization", tr["name"])
		md := tr["metadata"].(map[string]interface{})
		require.Equal(t, "75", md["threshold"])
		require.Contains(t, md["query"], "DCGM_FI_DEV_GPU_UTIL")

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, "true", dep.Annotations[k8s.ServiceTriggersOverrideAnnotation])
		require.Equal(t, "keda", dep.Annotations[k8s.ServiceTriggersOverrideCRDAnnotation])
	})
}

func TestServiceTriggersEnable_KedaPath_CPUOnKEDA(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		// Trigger mix: CPU + Queue → KEDA path. CPU trigger should be
		// the built-in cpu Type, not Prometheus.
		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{
				{Type: "cpu", Threshold: 70},
				{Type: "queueDepth", Threshold: 50},
			},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		obj, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers")
		require.Len(t, triggers, 2)

		types := []string{
			triggers[0].(map[string]interface{})["type"].(string),
			triggers[1].(map[string]interface{})["type"].(string),
		}
		require.Contains(t, types, "cpu")
		require.Contains(t, types, "prometheus")
	})
}

func TestServiceTriggersEnable_KedaOff_RejectsGPUTrigger(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = false
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "require KEDA")
	})
}

func TestServiceTriggersEnable_KedaOff_RejectsQueueTrigger(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = false
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "queueDepth", Threshold: 50}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "require KEDA")
	})
}

func TestServiceTriggersEnable_HPAtoKEDASwitch(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true

		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "old HPA must be deleted")

		_, err = p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "new ScaledObject must exist")

		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, "keda", dep.Annotations[k8s.ServiceTriggersOverrideCRDAnnotation])
	})
}

func TestServiceTriggersEnable_KEDAtoHPASwitch(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 5))

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		_, err = p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "old ScaledObject must be deleted")

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "new HPA must exist")

		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, "hpa", dep.Annotations[k8s.ServiceTriggersOverrideCRDAnnotation])
	})
}

func TestServiceTriggersEnable_ManifestHPAOwnershipTransfer(t *testing.T) {
	// Service that started with `count: 1-5` shorthand had a
	// manifest-materialized HPA. User enables override with same trigger
	// types (CPU) — no prior override annotation, but the HPA exists.
	// Expected: in-place update, no duplicate HPA. With no prior override
	// + HPA-path choice, we land on the same HPA (Update path in
	// applyTriggersHPA picks it up).
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		// Pre-existing manifest-materialized HPA with old thresholds.
		oldThresh := int32(50)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				Metrics: []autoscalingv2.MetricSpec{{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &oldThresh,
						},
					},
				}},
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		hpas, _ := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").List(context.TODO(), am.ListOptions{})
		require.Len(t, hpas.Items, 1, "must not duplicate HPA")
		require.Equal(t, int32(70), *hpas.Items[0].Spec.Metrics[0].Resource.Target.AverageUtilization)
	})
}

func TestServiceTriggersEnable_NoPriorOverride_ManifestSO_HPAOverride_DeletesSO(t *testing.T) {
	// Service had `scale.autoscale.cpu: 70` materialized as a KEDA SO.
	// No prior override annotation. User enables HPA-only override
	// (CPU). The manifest SO must be deleted so the new HPA is the
	// sole autoscaler.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 5))

		opts := structs.ServiceTriggersOptions{
			Min: 2, Max: 6,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		_, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "manifest SO must be deleted on cross-type transfer")

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "new HPA must exist")
	})
}

func TestServiceTriggersEnable_ManifestSOOwnershipTransfer(t *testing.T) {
	// Service with `scale.autoscale.cpu` has manifest-materialized SO.
	// User enables override with KEDA-path trigger — no prior override
	// annotation. Expected: SO updated in place, no duplicate.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 3))

		opts := structs.ServiceTriggersOptions{
			Min: 2, Max: 7,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 80}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		list, _ := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").List(context.TODO(), am.ListOptions{})
		require.Len(t, list.Items, 1, "must not duplicate ScaledObject")
		obj := list.Items[0]
		min, _, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicaCount")
		max, _, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicaCount")
		require.Equal(t, int64(2), min)
		require.Equal(t, int64(7), max)
	})
}

func TestServiceTriggersEnable_GPUWithoutScaleGPU_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 0)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "scale.gpu.count >= 1")
	})
}

func TestServiceTriggersThresholdSet_HPA_CPU(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		// Seed override + HPA at threshold=70.
		old := int32(70)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				Metrics: []autoscalingv2.MetricSpec{{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &old,
						},
					},
				}},
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)
		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		require.NoError(t, p.ServiceTriggersThresholdSet("app1", "web", "cpu", 85, "alice"))

		got, _ := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, int32(85), *got.Spec.Metrics[0].Resource.Target.AverageUtilization)
	})
}

func TestServiceTriggersThresholdSet_KEDA_GPU(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))
		require.NoError(t, p.ServiceTriggersThresholdSet("app1", "web", "gpuUtilization", 90, "alice"))

		obj, _ := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers")
		tr := triggers[0].(map[string]interface{})
		md := tr["metadata"].(map[string]interface{})
		require.Equal(t, "90", md["threshold"])
		require.Equal(t, "45", md["activationThreshold"], "activation must follow threshold/2 with floor 1")
	})
}

func TestServiceTriggersThresholdSet_NoOverride_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		err := p.ServiceTriggersThresholdSet("app1", "web", "cpu", 85, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "active triggers override")
	})
}

func TestServiceTriggersThresholdSet_HPA_TriggerNotPresent_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		err := p.ServiceTriggersThresholdSet("app1", "web", "memory", 80, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not present")
	})
}

func TestServiceTriggersDisable_KedaPath(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 1)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 5))

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		_, err = p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "ScaledObject must be deleted")

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		_, hasActive := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		require.False(t, hasActive)
	})
}

func TestServiceTriggers_Orthogonality_ScaleOverridePreserved(t *testing.T) {
	// A service can have both scale-override-active and
	// triggers-override-active set. Disabling triggers override must
	// leave scale-override-active intact (the two surfaces are
	// independent and have separate lifecycles).
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			"convox.com/scale-override-active":       "true",
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
		}
		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, p.ServiceTriggersDisable("app1", "web", "alice"))

		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		_, hasTriggers := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		require.False(t, hasTriggers, "triggers annotation must be cleared")
		require.Equal(t, "true", dep.Annotations["convox.com/scale-override-active"], "scale-override annotation must be untouched")

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err))
	})
}

func TestReleaseTemplateServices_TriggersOverrideActive_SkipsHPARender(t *testing.T) {
	// A service with `scale.count: 1-5` would normally render an HPA on
	// the classic-autoscale-block path. With the triggers-override-active
	// annotation set on its Deployment, the deploy controller must skip
	// HPA template rendering so the user's Console-driven autoscaler is
	// not overwritten.
	out, _, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		// Seed Deployment WITH triggers-override annotation set.
		seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 1)

		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})
		ss := scaleManifestServices(t, map[string]int{"web": 1})

		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			ss
	})
	require.NoError(t, err)
	require.NotContains(t, string(out), "HorizontalPodAutoscaler",
		"HPA template must not render when triggers-override-active is set")
}

func TestReleaseTemplateServices_NoTriggersOverride_RendersHPABackCompat(t *testing.T) {
	// Same fixture as above, but no annotation set — HPA template MUST
	// still render. Guards against accidental suppression of the
	// manifest-driven autoscaler.
	out, _, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")

		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})
		ss := scaleManifestServices(t, map[string]int{"web": 1})

		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			ss
	})
	require.NoError(t, err)
	require.Contains(t, string(out), "HorizontalPodAutoscaler",
		"HPA template must still render for services without triggers-override annotation")
}

// seedDeploymentWithTriggersOverride mirrors scaleSeedDeployment but with
// the triggers-override-active annotation pre-set. Used by deploy-
// controller integration tests that exercise the per-service skip path.
func seedDeploymentWithTriggersOverride(t *testing.T, c *kfake.Clientset, ns, name string, replicas int32) {
	t.Helper()
	r := replicas
	_, err := c.AppsV1().Deployments(ns).Create(context.TODO(), &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    map[string]string{"app": "app1", "type": "service", "service": name},
			Annotations: map[string]string{
				k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
				k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &r,
			Template: ac.PodTemplateSpec{Spec: ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}}},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func TestServiceUpdateRange_TriggersOverrideHPA_PatchesHPABounds(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)

		minR := int32(1)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: &minR,
				MaxReplicas: 3,
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{
			Min: options.Int(2),
			Max: options.Int(7),
		}))

		got, _ := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, int32(2), *got.Spec.MinReplicas)
		require.Equal(t, int32(7), got.Spec.MaxReplicas)

		// Deployment replicas must NOT be touched.
		d, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, int32(1), *d.Spec.Replicas)
	})
}

func TestServiceUpdateRange_TriggersOverrideKEDA_PatchesSOBounds(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 3))

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{
			Min: options.Int(2),
			Max: options.Int(7),
		}))

		obj, _ := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		min, _, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicaCount")
		max, _, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicaCount")
		require.Equal(t, int64(2), min)
		require.Equal(t, int64(7), max)
	})
}

func TestServiceProjection_TriggersOverrideActive_TrueWhenAnnotated(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 1)
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].TriggersOverrideActive, "3.24.6+ rack must always populate the pointer")
		require.True(t, *services[0].TriggersOverrideActive, "override annotation must drive the wire-projected field")
	})
}

func TestServiceProjection_TriggersOverrideActive_FalseWhenAbsent(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].TriggersOverrideActive)
		require.False(t, *services[0].TriggersOverrideActive, "absent annotation must serialize as *false, not nil, on 3.24.6+ racks")
	})
}

func TestServiceProjection_ClassicHPA_WidensAutoscaleEnabled(t *testing.T) {
	// Service with `scale.count: 1-6` (no explicit autoscale block)
	// materializes a native HPA. The resolver must populate
	// autoscale.enabled=true so Console pages reading that field
	// (ServiceOverviewPanel, ServiceScalingPanel) reflect reality.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Autoscale, "classic HPA path must surface autoscale state")
		require.True(t, services[0].Autoscale.Enabled, "autoscale.enabled must be true for count:1-N services")
	})
}

func TestServiceProjection_LiveHPAThresholdRead(t *testing.T) {
	// When an HPA exists for the service, the resolver projection
	// must surface its threshold(s) on autoscale.cpu_threshold /
	// autoscale.mem_threshold.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		threshold := int32(82)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				Metrics: []autoscalingv2.MetricSpec{{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: "cpu",
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &threshold,
						},
					},
				}},
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Autoscale)
		require.NotNil(t, services[0].Autoscale.CpuThreshold)
		require.Equal(t, 82, *services[0].Autoscale.CpuThreshold, "live HPA threshold must override manifest fallback")
	})
}

func TestServiceProjection_LiveSOThresholdRead(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		// Seed a KEDA ScaledObject with a convox-cpu trigger at value=88.
		so := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "keda.sh/v1alpha1",
				"kind":       "ScaledObject",
				"metadata": map[string]interface{}{
					"name":      "web",
					"namespace": "rack1-app1",
				},
				"spec": map[string]interface{}{
					"minReplicaCount": int64(1),
					"maxReplicaCount": int64(5),
					"triggers": []interface{}{
						map[string]interface{}{
							"type":     "cpu",
							"name":     "convox-cpu",
							"metadata": map[string]interface{}{"value": "88"},
						},
					},
				},
			},
		}
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), so)

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Autoscale)
		require.NotNil(t, services[0].Autoscale.CpuThreshold)
		require.Equal(t, 88, *services[0].Autoscale.CpuThreshold)
	})
}

func TestServiceTriggersDisable_CorruptionRecovery_NoCRDAnnotation(t *testing.T) {
	// Active annotation present but the triggers-override-crd annotation
	// is missing (e.g. partial write, hand-edited Deployment). Disable
	// must still clear the active annotation and try both delete paths.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation: k8s.ServiceTriggersOverrideValueOn,
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
		}
		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		require.NoError(t, p.ServiceTriggersDisable("app1", "web", "alice"))

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "corruption-recovery path must delete the HPA")
		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		_, hasActive := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		require.False(t, hasActive)
	})
}
