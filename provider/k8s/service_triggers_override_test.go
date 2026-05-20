package k8s_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
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

		seedDeployment(t, kk, "rack1-app1", "web", 5)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		minReplicas := int32(3)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: &minReplicas,
			},
		}
		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())
		installManifestServiceHook(p, "app1", "web", 0)

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
		require.Equal(t, int32(1), *dep.Spec.Replicas, "replicas must reset to manifest min")
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

func TestServiceTriggersEnable_ResetsReplicasToMin(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 3)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 9,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(1), *dep.Spec.Replicas, "Enable must reset replicas to opts.Min")
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
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		obj, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers") //nolint:errcheck
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
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
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
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers") //nolint:errcheck
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

func TestServiceTriggersEnable_HPAPath_MinZero_Rejected(t *testing.T) {
	// HPA-backed autoscale requires min >= 1 on standard K8s (the
	// HPAScaleToZero feature gate is alpha and not enabled on EKS).
	// Reject Min=0 with a friendly error pointing users at the KEDA
	// path instead of letting the K8s API server surface a bare
	// HorizontalPodAutoscaler validation error in the toast.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 0, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "do not support min=0")
	})
}

func TestServiceTriggersEnable_KedaPath_MinZero_Allowed(t *testing.T) {
	// KEDA's ScaledObject supports scale-to-zero natively. Min=0 on
	// the KEDA path must NOT be gated by the HPA-specific check.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 0, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))
	})
}

func TestServiceTriggersThresholdSet_OverPercentCap_Rejects(t *testing.T) {
	// ThresholdSet on a percent-bound trigger (cpu / memory / gpu)
	// must reject values > 100 — same validation rule as Enable's
	// TriggerSpec.Validate. The earlier implementation only checked
	// for `threshold <= 0`, letting nonsensical values through.
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

		err := p.ServiceTriggersThresholdSet("app1", "web", "cpu", 150, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "percent triggers must be <= 100")
	})
}

func TestServiceTriggersEnable_HPAUpdate_PreservesLabelsAndAnnotations(t *testing.T) {
	// When the user takes ownership of an existing HPA (manifest-
	// materialized OR Console-driven re-enable), the Update path
	// must preserve labels, annotations, and Spec.Behavior. Earlier
	// implementation Update'd a fresh struct, wiping all of those.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		oldThresh := int32(50)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{
				Name:        "web",
				Namespace:   "rack1-app1",
				Labels:      map[string]string{"app": "app1", "system": "convox", "custom-label": "keep-me"},
				Annotations: map[string]string{"convox.com/manifest-version": "v123"},
			},
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

		got, _ := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, int32(70), *got.Spec.Metrics[0].Resource.Target.AverageUtilization, "threshold should be replaced")
		require.Equal(t, "keep-me", got.Labels["custom-label"], "user-set labels must survive ownership transfer")
		require.Equal(t, "v123", got.Annotations["convox.com/manifest-version"], "manifest-set annotations must survive ownership transfer")
	})
}

func TestServiceTriggersEnable_KEDAUpdate_PreservesAdvancedFields(t *testing.T) {
	// Symmetric to the HPA preservation test: KEDA ScaledObject's
	// cooldownPeriod / pollingInterval / advanced behaviors are
	// yaml-only per the spec. Console-driven re-enable must preserve
	// those fields when patching the override.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")

		// Seed an SO with advanced fields the override must preserve.
		so := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "keda.sh/v1alpha1",
				"kind":       "ScaledObject",
				"metadata": map[string]interface{}{
					"name":      "web",
					"namespace": "rack1-app1",
					"labels":    map[string]interface{}{"custom-label": "keep-me"},
				},
				"spec": map[string]interface{}{
					"scaleTargetRef":  map[string]interface{}{"name": "web"},
					"minReplicaCount": int64(1),
					"maxReplicaCount": int64(3),
					"cooldownPeriod":  int64(120),
					"pollingInterval": int64(15),
					"triggers":        []interface{}{},
				},
			},
		}
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), so)

		opts := structs.ServiceTriggersOptions{
			Min: 2, Max: 7,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 80}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		got, _ := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		cooldown, _, _ := unstructured.NestedInt64(got.Object, "spec", "cooldownPeriod") //nolint:errcheck
		polling, _, _ := unstructured.NestedInt64(got.Object, "spec", "pollingInterval") //nolint:errcheck
		require.Equal(t, int64(120), cooldown, "cooldownPeriod must survive update")
		require.Equal(t, int64(15), polling, "pollingInterval must survive update")
		labels, _, _ := unstructured.NestedStringMap(got.Object, "metadata", "labels") //nolint:errcheck
		require.Equal(t, "keep-me", labels["custom-label"], "user labels must survive update")
	})
}

func TestServiceTriggersEnable_KedaPath_AnnotationPatchFails_RollsBackSO(t *testing.T) {
	// Symmetric to the HPA-path rollback test: KEDA path must
	// best-effort delete the just-created SO when annotation patch
	// fails. Mirrors the HPA-path invariant on the dynamic-client
	// surface.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		kk.PrependReactor("patch", "deployments", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated patch failure")
		})

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err, "annotation patch failure must surface")

		_, getErr := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(getErr), "SO must be rolled back when annotation patch fails")
	})
}

func TestServiceTriggersEnable_AnnotationPatchFails_RollsBackCRD(t *testing.T) {
	// Spec at line 271 promises: "if the new CRD create succeeds but
	// the subsequent annotation write fails, the handler attempts a
	// best-effort delete of the just-created CRD before returning the
	// original error." Inject a patch-deployments reactor that errors
	// AFTER the HPA is created, then verify the HPA is rolled back.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		// Patch the deployment Patch call to fail unconditionally — this
		// simulates the annotation-write failure post-CRD-create.
		kk.PrependReactor("patch", "deployments", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf("simulated patch failure")
		})

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err, "annotation patch failure must surface")

		// CRD must be rolled back.
		_, getErr := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(getErr), "HPA must be rolled back when annotation patch fails")
	})
}

func TestReleaseTemplateServices_TriggersOverrideActive_SkipsKEDABuild(t *testing.T) {
	// Symmetric to TestReleaseTemplateServices_TriggersOverrideActive_SkipsHPARender
	// but for the KEDA-build branch. A service with `scale.autoscale.cpu`
	// + the triggers-override annotation must NOT have a ScaledObject
	// rebuilt by the deploy controller — the Console-driven override
	// keeps its own CRD across deploys.
	out, _, err := runReleaseTemplateServicesEvents(t, func(p *k8s.Provider) (*structs.App, *structs.Release, manifest.Services) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true

		yaml := `services:
  web:
    image: docker.io/library/nginx
    port: 5000
    scale:
      min: 1
      max: 5
      autoscale:
        cpu:
          threshold: 70
`
		cc, _ := p.Convox.(*cvfake.Clientset)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", yaml))
		aa, _ := p.Atom.(*atom.MockInterface)
		aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)

		m, err := manifest.Load([]byte(yaml), structs.Environment{})
		require.NoError(t, err)
		return &structs.App{Name: "app1", Release: "release1"},
			&structs.Release{Id: "release1", App: "app1"},
			m.Services
	})
	require.NoError(t, err)
	require.NotContains(t, string(out), "ScaledObject",
		"deploy controller must not rebuild ScaledObject when triggers-override is active")
}

func TestServiceProjection_DualCRDDetected_EmitsLog(t *testing.T) {
	// Operator-introduced corruption (both HPA and KEDA SO exist for
	// same service) must surface a structured log line on every
	// ServiceList projection invocation. Captures stdout to verify.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		// Seed both an HPA and a ScaledObject for the same service.
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: int32Ptr(1),
				MaxReplicas: 5,
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 1, 5))

		stop := captureStdout(t)
		_, err = p.ServiceList("app1")
		require.NoError(t, err)
		stdout := stop()
		require.Contains(t, stdout, "at=dual-crd-detected",
			"dual-CRD state must emit operator-corruption log line")
		require.Contains(t, stdout, `service="web"`)
	})
}

// int32Ptr returns a pointer to the value. Used by test fixtures that
// construct HPA structs with int32 replica bounds; declared once here
// so the trigger-override test suite stays self-contained.
func int32Ptr(v int32) *int32 { return &v }

func TestServiceTriggersEnable_HPAtoKEDASwitch(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")

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
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
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
		min, _, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicaCount") //nolint:errcheck
		max, _, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicaCount") //nolint:errcheck
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
		t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))
		require.NoError(t, p.ServiceTriggersThresholdSet("app1", "web", "gpuUtilization", 90, "alice"))

		obj, _ := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		triggers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "triggers") //nolint:errcheck
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

func TestServiceTriggersThresholdSet_HPA_UpsertMemory(t *testing.T) {
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
		require.NoError(t, err)

		hpa, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Len(t, hpa.Spec.Metrics, 2)

		var foundCPU, foundMem bool
		for _, m := range hpa.Spec.Metrics {
			if m.Resource != nil && string(m.Resource.Name) == "cpu" {
				foundCPU = true
				require.Equal(t, int32(70), *m.Resource.Target.AverageUtilization)
			}
			if m.Resource != nil && string(m.Resource.Name) == "memory" {
				foundMem = true
				require.Equal(t, int32(80), *m.Resource.Target.AverageUtilization)
			}
		}
		require.True(t, foundCPU, "CPU metric must be present")
		require.True(t, foundMem, "memory metric must be upserted")
	})
}

func TestServiceTriggersThresholdSet_HPA_UpsertCPU(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "memory", Threshold: 80}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		err := p.ServiceTriggersThresholdSet("app1", "web", "cpu", 65, "alice")
		require.NoError(t, err)

		hpa, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Len(t, hpa.Spec.Metrics, 2)

		var foundCPU, foundMem bool
		for _, m := range hpa.Spec.Metrics {
			if m.Resource != nil && string(m.Resource.Name) == "cpu" {
				foundCPU = true
				require.Equal(t, int32(65), *m.Resource.Target.AverageUtilization)
			}
			if m.Resource != nil && string(m.Resource.Name) == "memory" {
				foundMem = true
				require.Equal(t, int32(80), *m.Resource.Target.AverageUtilization)
			}
		}
		require.True(t, foundCPU, "CPU metric must be upserted")
		require.True(t, foundMem, "memory metric must be present")
	})
}

func TestServiceTriggersDisable_KedaPath(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 4)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 2, 8))
		installManifestServiceHook(p, "app1", "web", 0)

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		_, err = p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "ScaledObject must be deleted")

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		_, hasActive := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		require.False(t, hasActive)
		require.Equal(t, int32(1), *dep.Spec.Replicas, "replicas must reset to manifest min")
	})
}

func TestServiceTriggersDisable_ReinstatesManifestHPA(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 4)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDHPA,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		minReplicas := int32(3)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: &minReplicas,
			},
		}
		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())
		p.TriggersOverrideManifestServiceHook = func(a, s string) (*manifest.Service, error) {
			ms := &manifest.Service{Name: s}
			ms.Scale.Count.Min = 1
			ms.Scale.Count.Max = 4
			ms.Scale.Targets.Cpu = 70
			ms.Scale.Targets.Memory = 80
			return ms, nil
		}

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		reinstated, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "manifest HPA must be reinstated after disable")
		require.Equal(t, int32(4), reinstated.Spec.MaxReplicas)
		require.Equal(t, int32(1), *reinstated.Spec.MinReplicas)
		require.Len(t, reinstated.Spec.Metrics, 2)

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(1), *dep.Spec.Replicas, "replicas must reset to manifest min")
	})
}

func TestServiceTriggersDisable_ReinstatesManifestKEDA(t *testing.T) {
	// Manifest autoscale block with cpu+memory routes to HPA via
	// triggersCRDChoice — only gpu/queue forces KEDA. This test uses
	// cpu+queueDepth to exercise the KEDA reinstatement path.
	t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 3)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 2, 8))

		p.TriggersOverrideManifestServiceHook = func(a, s string) (*manifest.Service, error) {
			ms := &manifest.Service{Name: s}
			ms.Scale.Count.Min = 1
			ms.Scale.Count.Max = 6
			ms.Scale.Autoscale = &manifest.ServiceAutoscale{
				Cpu:        &manifest.AutoscaleMode{Threshold: 70},
				QueueDepth: &manifest.AutoscaleMode{Threshold: 25},
			}
			return ms, nil
		}

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		reinstated, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "manifest ScaledObject must be reinstated after disable")

		maxReplica, _, _ := unstructured.NestedInt64(reinstated.Object, "spec", "maxReplicaCount") //nolint:errcheck
		require.Equal(t, int64(6), maxReplica)

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(1), *dep.Spec.Replicas, "replicas must reset to manifest min")
	})
}

func TestServiceTriggersEnable_Orthogonality_ScaleOverridePreserved(t *testing.T) {
	// Symmetric to the Disable orthogonality test: Enable must NOT
	// touch the scale-override annotation. Both override surfaces are
	// independent and a service can have both active. The
	// StrategicMergePatch on the triggers annotation pair must leave
	// the scale-override key intact.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 5)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		// Pre-seed scale-override annotation; no triggers annotation yet.
		dep, _ := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		dep.Annotations = map[string]string{
			"convox.com/scale-override-active": "true",
		}
		_, err := kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))

		// Both annotations must be present: scale-override (existing) +
		// triggers-override (just written).
		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.Equal(t, "true", dep.Annotations["convox.com/scale-override-active"], "scale-override must be preserved across Enable")
		require.Equal(t, "true", dep.Annotations[k8s.ServiceTriggersOverrideAnnotation], "triggers-override must be set by Enable")
	})
}

func TestServiceTriggersEnable_EmitsBothActorAndAckBy(t *testing.T) {
	// Wire contract: the triggers-override audit events must include
	// both `actor` (legacy field) and `ack_by` (canonical 3.24.6+ field)
	// in the EventSend Data payload. Webhook receivers depending on
	// either key must work. Mirrors the scale-override precedent.
	var (
		mu       sync.Mutex
		captured []map[string]any
	)
	srv := webhookCaptureServer(&mu, &captured)
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice@example.com"))
		drainPendingDispatches()
	})

	mu.Lock()
	defer mu.Unlock()
	toggled := findAllByAction(captured, "app:triggers-override:toggled")
	require.Len(t, toggled, 1, "exactly one app:triggers-override:toggled event")
	data, _ := toggled[0]["data"].(map[string]any)
	require.NotNil(t, data, "event payload must include data block")
	require.Equal(t, "alice@example.com", data["actor"], "legacy actor key carried")
	require.Equal(t, "alice@example.com", data["ack_by"], "canonical ack_by key carried")
	require.Equal(t, "on", data["state"])
	require.Equal(t, "hpa", data["crd"])
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
		installManifestServiceHook(p, "app1", "web", 0)

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
		min, _, _ := unstructured.NestedInt64(obj.Object, "spec", "minReplicaCount") //nolint:errcheck
		max, _, _ := unstructured.NestedInt64(obj.Object, "spec", "maxReplicaCount") //nolint:errcheck
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

func TestServiceProjection_LiveHPABoundsOverride(t *testing.T) {
	// `convox scale --min --max` and the Console Range Apply both
	// patch the live HPA's MinReplicas/MaxReplicas. The manifest
	// values stay unchanged. Without this overlay the bounds card
	// would show the manifest-time min/max forever, drifting from
	// live cluster state. Verify the live HPA bounds win when
	// present.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		// Manifest says count: 1 → fixed; live HPA was patched to
		// min=2, max=7 by an out-of-band scale call.
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		liveMin := int32(2)
		hpa := &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
			Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
				MinReplicas: &liveMin,
				MaxReplicas: 7,
			},
		}
		_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), hpa, am.CreateOptions{})
		require.NoError(t, err)

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Min)
		require.NotNil(t, services[0].Max)
		require.Equal(t, 2, *services[0].Min, "bounds card must reflect live HPA Min, not manifest")
		require.Equal(t, 7, *services[0].Max, "bounds card must reflect live HPA Max, not manifest")
	})
}

func TestServiceProjection_LiveSOBoundsOverride(t *testing.T) {
	// Symmetric to LiveHPABoundsOverride: when the service is owned
	// by a KEDA ScaledObject (manifest scale.autoscale.*), live
	// minReplicaCount / maxReplicaCount win over the manifest's
	// scale.count.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 3, 9))

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Min)
		require.NotNil(t, services[0].Max)
		require.Equal(t, 3, *services[0].Min, "bounds card must reflect live SO minReplicaCount")
		require.Equal(t, 9, *services[0].Max, "bounds card must reflect live SO maxReplicaCount")
	})
}

func TestServiceProjection_NoCRD_FallsBackToManifest(t *testing.T) {
	// Without any autoscaler CRD, the bounds card shows manifest
	// values. Default state for fixed-count services.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", "web", 1, "")
		// Manifest seeded with count: 1 (via scaleSeedAppRelease default).
		scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		services, err := p.ServiceList("app1")
		require.NoError(t, err)
		require.Len(t, services, 1)
		require.NotNil(t, services[0].Min)
		require.NotNil(t, services[0].Max)
		// scaleSeedAppRelease seeds count: N-N+5 shorthand, so manifest
		// min=1, max=6. The fixture detail; the contract is: no CRD →
		// manifest values flow through.
		require.Equal(t, 1, *services[0].Min)
		require.Equal(t, 6, *services[0].Max)
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
		installManifestServiceHook(p, "app1", "web", 0)

		require.NoError(t, p.ServiceTriggersDisable("app1", "web", "alice"))

		_, err = kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.True(t, kerr.IsNotFound(err), "corruption-recovery path must delete the HPA")
		dep, _ = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		_, hasActive := dep.Annotations[k8s.ServiceTriggersOverrideAnnotation]
		require.False(t, hasActive)
	})
}

func TestServiceTriggersEnable_GPU_PrometheusEmpty_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "gpuUtilization", Threshold: 75}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "GPU Telemetry")
	})
}

func TestServiceTriggersEnable_Queue_PrometheusEmpty_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "queueDepth", Threshold: 50}},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "GPU Telemetry")
	})
}

func TestServiceTriggersEnable_MixedGPUCPU_PrometheusEmpty_Rejects(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		installManifestServiceHook(p, "app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{
				{Type: "cpu", Threshold: 70},
				{Type: "gpuUtilization", Threshold: 75},
			},
		}
		err := p.ServiceTriggersEnable("app1", "web", opts, "alice")
		require.Error(t, err)
		require.Contains(t, err.Error(), "GPU Telemetry")
	})
}

func TestServiceTriggersEnable_CPUOnly_PrometheusEmpty_Succeeds(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", "web", 1)
		p.IsKedaEnabled = true
		t.Setenv("PROMETHEUS_URL", "")
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme())

		opts := structs.ServiceTriggersOptions{
			Min: 1, Max: 5,
			Triggers: []structs.TriggerSpec{{Type: "cpu", Threshold: 70}},
		}
		require.NoError(t, p.ServiceTriggersEnable("app1", "web", opts, "alice"))
	})
}

func TestServiceTriggersDisable_ReinstatesAutoscaleCustom(t *testing.T) {
	// scale.autoscale.custom contains raw kedav1alpha1.ScaleTriggers that
	// can't round-trip through ServiceTriggersOptions/TriggerSpec. The
	// reinstatement must detect custom triggers and route to
	// reinstateKedaScaledObject instead of manifestToTriggersOptions.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 4)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 2, 8))

		p.TriggersOverrideManifestServiceHook = func(a, s string) (*manifest.Service, error) {
			ms := &manifest.Service{Name: s}
			ms.Scale.Count.Min = 1
			ms.Scale.Count.Max = 5
			ms.Scale.Autoscale = &manifest.ServiceAutoscale{
				Custom: []kedav1alpha1.ScaleTriggers{
					{
						Type: "aws-sqs-queue",
						Name: "my-sqs-trigger",
						Metadata: map[string]string{
							"queueURL":    "https://sqs.us-east-1.amazonaws.com/123456789/my-queue",
							"queueLength": "5",
						},
					},
				},
			}
			return ms, nil
		}

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		reinstated, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "autoscale.custom ScaledObject must be reinstated after disable")

		minReplica, _, _ := unstructured.NestedInt64(reinstated.Object, "spec", "minReplicaCount") //nolint:errcheck
		maxReplica, _, _ := unstructured.NestedInt64(reinstated.Object, "spec", "maxReplicaCount") //nolint:errcheck
		require.Equal(t, int64(1), minReplica)
		require.Equal(t, int64(5), maxReplica)

		triggers, _, _ := unstructured.NestedSlice(reinstated.Object, "spec", "triggers") //nolint:errcheck
		require.Len(t, triggers, 1, "reinstated SO must carry the custom trigger")
		tr := triggers[0].(map[string]interface{})
		require.Equal(t, "aws-sqs-queue", tr["type"])

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(1), *dep.Spec.Replicas, "replicas must reset to manifest min")
	})
}

func TestServiceTriggersDisable_ReinstatesScaleKeda(t *testing.T) {
	// scale.keda is a power-user path with raw kedav1alpha1.ScaleTriggers.
	// After disabling a KEDA-type override, reinstateManifestAutoscaler
	// must detect IsKedaEnabled() and call reinstateKedaScaledObject to
	// rebuild the ScaledObject from the manifest's Keda config.
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		seedDeployment(t, kk, "rack1-app1", "web", 5)
		dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		dep.Annotations = map[string]string{
			k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
			k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
		}
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
		require.NoError(t, err)

		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(),
			scaledObjectUnstructured("rack1-app1", "web", 3, 10))

		p.TriggersOverrideManifestServiceHook = func(a, s string) (*manifest.Service, error) {
			ms := &manifest.Service{Name: s}
			ms.Scale.Count.Min = 2
			ms.Scale.Count.Max = 8
			ms.Scale.Keda = &manifest.ServiceScaleKeda{
				Triggers: []kedav1alpha1.ScaleTriggers{
					{
						Type: "prometheus",
						Name: "custom-metric",
						Metadata: map[string]string{
							"serverAddress": "http://prom:9090",
							"metricName":    "custom_requests_total",
							"threshold":     "100",
							"query":         "sum(rate(custom_requests_total[5m]))",
						},
					},
				},
			}
			return ms, nil
		}

		err = p.ServiceTriggersDisable("app1", "web", "alice@example.com")
		require.NoError(t, err)

		reinstated, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err, "scale.keda ScaledObject must be reinstated after disable")

		minReplica, _, _ := unstructured.NestedInt64(reinstated.Object, "spec", "minReplicaCount") //nolint:errcheck
		maxReplica, _, _ := unstructured.NestedInt64(reinstated.Object, "spec", "maxReplicaCount") //nolint:errcheck
		require.Equal(t, int64(2), minReplica, "reinstated SO must use manifest min")
		require.Equal(t, int64(8), maxReplica, "reinstated SO must use manifest max")

		triggers, _, _ := unstructured.NestedSlice(reinstated.Object, "spec", "triggers") //nolint:errcheck
		require.Len(t, triggers, 1, "reinstated SO must carry the raw keda trigger")
		tr := triggers[0].(map[string]interface{})
		require.Equal(t, "prometheus", tr["type"])
		require.Equal(t, "custom-metric", tr["name"])

		dep, err = kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, int32(2), *dep.Spec.Replicas, "replicas must reset to manifest min")
	})
}
