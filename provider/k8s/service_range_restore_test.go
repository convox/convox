package k8s_test

import (
	"context"
	"errors"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

const rangeRestorePrefix = "service web was at 0 replicas and is now scaled to "

func rangeRestoreProvider(t *testing.T, fn func(*k8s.Provider, *kfake.Clientset, *fake.FakeDynamicClient)) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		dyn := fake.NewSimpleDynamicClient(newDynamicScheme())
		p.DynamicClient = dyn
		fn(p, kk, dyn)
	})
}

func rangeRestoreSeedHPA(t *testing.T, kk *kfake.Clientset, minReplicas, maxReplicas int32) {
	t.Helper()
	_, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Create(context.TODO(), &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: am.ObjectMeta{Name: "web", Namespace: "rack1-app1"},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
		},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func rangeRestoreHPABounds(t *testing.T, kk *kfake.Clientset) (int32, int32) {
	t.Helper()
	hpa, err := kk.AutoscalingV2().HorizontalPodAutoscalers("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, hpa.Spec.MinReplicas)
	return *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas
}

func rangeRestoreScaledObjectGets(dyn *fake.FakeDynamicClient) int {
	n := 0
	for _, action := range dyn.Actions() {
		if action.GetVerb() == "get" && action.GetResource() == testScaledObjectGVR {
			n++
		}
	}
	return n
}

func rangeRestoreSeedRelease(t *testing.T, p *k8s.Provider, manifestYaml string) {
	t.Helper()
	aa, _ := p.Atom.(*atom.MockInterface)
	cc, _ := p.Convox.(*cvfake.Clientset)
	aa.On("Status", "rack1-app1", "app").Return("Running", "release1", nil)
	require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release1", manifestYaml))
}

func rangeRestoreRequireConflict(t *testing.T, err error) {
	t.Helper()
	var herr *structs.HttpError
	require.True(t, errors.As(err, &herr), "want a conflict error, got %v", err)
	require.Equal(t, 409, herr.Code())
	require.Contains(t, herr.Error(), "convox budget reset app1")
}

func TestServiceUpdateRangeRestoreTriggersOverrideHPA(t *testing.T) {
	t.Run("min and max", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 0)
			rangeRestoreSeedHPA(t, kk, 1, 3)

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2), Max: options.Int(7)}))

			require.Equal(t, int32(2), pauseDeploymentReplicas(t, kk, "web"))
			lo, hi := rangeRestoreHPABounds(t, kk)
			require.Equal(t, int32(2), lo)
			require.Equal(t, int32(7), hi)
		})
	})

	t.Run("max only", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 0)
			rangeRestoreSeedHPA(t, kk, 1, 3)

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Max: options.Int(5)}))

			require.Equal(t, int32(1), pauseDeploymentReplicas(t, kk, "web"))
			lo, hi := rangeRestoreHPABounds(t, kk)
			require.Equal(t, int32(1), lo)
			require.Equal(t, int32(5), hi)
		})
	})
}

func TestServiceUpdateRangeRestoreAfterCountZero(t *testing.T) {
	rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
		seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 2)
		rangeRestoreSeedHPA(t, kk, 1, 3)

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(0)}))
		require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)}))
		require.Equal(t, int32(1), pauseDeploymentReplicas(t, kk, "web"))
	})
}

func TestServiceUpdateRangeRestoreWithoutAutoscaler(t *testing.T) {
	cases := []struct {
		name string
		keda bool
		want string
	}{
		{"keda enabled", true, "requires an autoscale block in convox.yml"},
		{"keda disabled", false, "requires KEDA on this rack"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
				seedDeployment(t, kk, "rack1-app1", "web", 0)
				p.IsKedaEnabled = tc.keda

				err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)})

				require.Error(t, err)
				require.Contains(t, err.Error(), rangeRestorePrefix+"1; range scaling (min/max) "+tc.want)
				require.Equal(t, int32(1), pauseDeploymentReplicas(t, kk, "web"))
			})
		})
	}
}

func TestServiceUpdateRangeRestoreKedaCRDAbsent(t *testing.T) {
	rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		seedDeployment(t, kk, "rack1-app1", "web", 0)
		dyn.PrependReactor("get", "scaledobjects", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, &meta.NoKindMatchError{
				GroupKind:        schema.GroupKind{Group: "keda.sh", Kind: "ScaledObject"},
				SearchedVersions: []string{"v1alpha1"},
			}
		})

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(4), Max: options.Int(6)})

		require.Error(t, err)
		require.Contains(t, err.Error(), rangeRestorePrefix+"4; range scaling (min/max) requires KEDA on this rack")
		require.Equal(t, int32(4), pauseDeploymentReplicas(t, kk, "web"))
	})
}

func TestServiceUpdateRangeRestoreMinOnly(t *testing.T) {
	t.Run("min 2", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 0)
			p.IsKedaEnabled = true

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2)}))

			require.Equal(t, int32(2), pauseDeploymentReplicas(t, kk, "web"))
		})
	})

	t.Run("min 0", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 0)
			p.IsKedaEnabled = true
			rangeRestoreSeedRelease(t, p, "services:\n  web:\n    image: docker.io/library/nginx\n    scale:\n      min: 0\n      max: 3\n      autoscale:\n        queueDepth:\n          threshold: 3\n")

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(0)}))

			require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
		})
	})
}

func TestServiceUpdateRangeRestoreScaledObjectReadError(t *testing.T) {
	rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 0)
		rangeRestoreSeedHPA(t, kk, 1, 3)
		dyn.PrependReactor("get", "scaledobjects", func(_ k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, kerr.NewForbidden(schema.GroupResource{Group: "keda.sh", Resource: "scaledobjects"}, "web", errors.New("denied"))
		})

		err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)})

		require.True(t, kerr.IsForbidden(err), "want the read error, got %v", err)
		require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
		lo, hi := rangeRestoreHPABounds(t, kk)
		require.Equal(t, int32(1), lo)
		require.Equal(t, int32(3), hi)
	})
}

func TestServiceUpdateRangeRestoreMinZero(t *testing.T) {
	t.Run("autoscale block", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 0)
			p.IsKedaEnabled = true
			rangeRestoreSeedRelease(t, p, "services:\n  web:\n    image: docker.io/library/nginx\n    scale:\n      min: 0\n      max: 3\n      autoscale:\n        queueDepth:\n          threshold: 3\n")

			err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(0), Max: options.Int(3)})

			require.Error(t, err)
			require.Contains(t, err.Error(), rangeRestorePrefix+"1; range scaling (min/max) requires an autoscale block")
			require.Equal(t, int32(1), pauseDeploymentReplicas(t, kk, "web"))
		})
	})

	t.Run("no autoscale block", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 0)
			p.IsKedaEnabled = true
			rangeRestoreSeedRelease(t, p, "services:\n  web:\n    image: docker.io/library/nginx\n")

			err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(0), Max: options.Int(3)})

			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot set --min 0 on service web")
			require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
		})
	})
}

func TestServiceUpdateRangeRestoreRefusedWhileBudgetClaims(t *testing.T) {
	t.Run("triggers override", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 0)
			rangeRestoreSeedHPA(t, kk, 1, 3)
			require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", pauseShutdownState("web")))

			rangeRestoreRequireConflict(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2), Max: options.Int(7)}))

			require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
			lo, hi := rangeRestoreHPABounds(t, kk)
			require.Equal(t, int32(1), lo)
			require.Equal(t, int32(3), hi)
		})
	})

	t.Run("no autoscaler", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 0)
			p.IsKedaEnabled = true
			require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", pauseShutdownState("web")))

			rangeRestoreRequireConflict(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)}))

			require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
		})
	})

	t.Run("running service", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
			seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 2)
			rangeRestoreSeedHPA(t, kk, 1, 3)
			require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", pauseShutdownState("web")))

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2), Max: options.Int(7)}))

			require.Equal(t, int32(2), pauseDeploymentReplicas(t, kk, "web"))
			lo, hi := rangeRestoreHPABounds(t, kk)
			require.Equal(t, int32(2), lo)
			require.Equal(t, int32(7), hi)
		})
	})
}

func TestServiceUpdateRangeLeavesRunningServiceAlone(t *testing.T) {
	t.Run("triggers override", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
			seedDeploymentWithTriggersOverride(t, kk, "rack1-app1", "web", 2)
			rangeRestoreSeedHPA(t, kk, 1, 3)

			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)}))

			require.Equal(t, int32(2), pauseDeploymentReplicas(t, kk, "web"))
			require.Zero(t, rangeRestoreScaledObjectGets(dyn))
		})
	})

	t.Run("no autoscaler", func(t *testing.T) {
		rangeRestoreProvider(t, func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
			seedDeployment(t, kk, "rack1-app1", "web", 2)
			p.IsKedaEnabled = true

			err := p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)})

			require.Error(t, err)
			require.NotContains(t, err.Error(), "was at 0 replicas")
			require.Equal(t, int32(2), pauseDeploymentReplicas(t, kk, "web"))
			require.Equal(t, 1, rangeRestoreScaledObjectGets(dyn))
		})
	})
}

func TestServiceUpdateRangeScaledObjectPathUnchanged(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, pauseAnnosManual), func(p *k8s.Provider, kk *kfake.Clientset, _ *fake.FakeDynamicClient) {
		d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "web", am.GetOptions{})
		require.NoError(t, err)
		zero := int32(0)
		d.Spec.Replicas = &zero
		_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), d, am.UpdateOptions{})
		require.NoError(t, err)

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(1), Max: options.Int(3)}))

		so := pauseGetScaledObject(t, p, "web")
		require.Empty(t, so.GetAnnotations())
		lo, hi := pauseScaledObjectBounds(so)
		require.Equal(t, int64(1), lo)
		require.Equal(t, int64(3), hi)
		require.Equal(t, int32(0), pauseDeploymentReplicas(t, kk, "web"))
	})
}
