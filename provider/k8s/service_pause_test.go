package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	budgetPause = map[string]string{structs.KedaPausedReplicasAnnotation: "0"}
	manualPause = map[string]string{structs.KedaPausedReplicasAnnotation: "0", structs.KedaManualPauseAnnotation: "true"}
	markerOnly  = map[string]string{structs.KedaManualPauseAnnotation: "true"}
)

func pausedScaledObject(name string, minReplicas, maxReplicas int64, annotations map[string]string) *unstructured.Unstructured {
	so := scaledObjectUnstructured("rack1-app1", name, minReplicas, maxReplicas)
	so.SetAnnotations(annotations)
	return so
}

func pauseShutdownState(services ...string) *structs.AppBudgetShutdownState {
	armed := time.Now().UTC().Add(-10 * time.Minute)
	state := &structs.AppBudgetShutdownState{
		SchemaVersion:        1,
		ArmedAt:              &armed,
		RecoveryMode:         "auto-on-reset",
		ShutdownOrder:        "largest-cost",
		ShutdownTickId:       "tick-pause",
		EligibleServiceCount: len(services),
	}
	for _, name := range services {
		state.Services = append(state.Services, structs.AppBudgetShutdownStateService{Name: name})
	}
	return state
}

func scaledObjectPatchCount(dyn *fake.FakeDynamicClient) int {
	n := 0
	for _, action := range dyn.Actions() {
		if action.GetVerb() == "patch" && action.GetResource() == testScaledObjectGVR {
			n++
		}
	}
	return n
}

func firstScaledObjectPatchAnnotations(t *testing.T, dyn *fake.FakeDynamicClient) map[string]interface{} {
	t.Helper()
	for _, action := range dyn.Actions() {
		patch, ok := action.(k8stesting.PatchAction)
		if !ok || patch.GetResource() != testScaledObjectGVR {
			continue
		}
		var body struct {
			Metadata struct {
				Annotations map[string]interface{} `json:"annotations"`
			} `json:"metadata"`
		}
		require.NoError(t, json.Unmarshal(patch.GetPatch(), &body))
		return body.Metadata.Annotations
	}
	require.FailNow(t, "no ScaledObject patch sent")
	return nil
}

func getScaledObject(t *testing.T, p *k8s.Provider, name string) *unstructured.Unstructured {
	t.Helper()
	so, err := p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	return so
}

func scaledObjectBounds(so *unstructured.Unstructured) (minReplicas, maxReplicas int64) {
	spec, _ := so.Object["spec"].(map[string]interface{})
	return toInt64(spec["minReplicaCount"]), toInt64(spec["maxReplicaCount"])
}

func deploymentReplicas(t *testing.T, kk *kfake.Clientset, name string) int32 {
	t.Helper()
	d, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), name, am.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, d.Spec.Replicas)
	return *d.Spec.Replicas
}

func pauseTestProvider(t *testing.T, so *unstructured.Unstructured, fn func(*k8s.Provider, *kfake.Clientset, *fake.FakeDynamicClient)) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		seedDeployment(t, kk, "rack1-app1", so.GetName(), 2)
		dyn := fake.NewSimpleDynamicClient(newDynamicScheme(), so)
		p.DynamicClient = dyn
		fn(p, kk, dyn)
	})
}

func TestServiceUpdateCountZeroPausesScaledObject(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, nil), func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(0)}))

		so := getScaledObject(t, p, "web")
		require.Equal(t, manualPause, so.GetAnnotations())
		lo, hi := scaledObjectBounds(so)
		require.Equal(t, int64(1), lo)
		require.Equal(t, int64(5), hi)
		require.Equal(t, int32(2), deploymentReplicas(t, kk, "web"))
		require.Equal(t, 1, scaledObjectPatchCount(dyn))

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(0)}))

		require.Equal(t, 1, scaledObjectPatchCount(dyn))
		require.Equal(t, int32(0), deploymentReplicas(t, kk, "web"))
		require.Equal(t, manualPause, getScaledObject(t, p, "web").GetAnnotations())
	})
}

func TestServiceUpdateCountZeroSetsLongHPAName(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject(longServiceName, 1, 5, nil), func(p *k8s.Provider, _ *kfake.Clientset, _ *fake.FakeDynamicClient) {
		require.NoError(t, p.ServiceUpdate("app1", longServiceName, structs.ServiceUpdateOptions{Count: options.Int(0)}))

		hpaName, _, _ := unstructured.NestedString(scaledObjectAdvanced(t, p, "rack1-app1", longServiceName), "horizontalPodAutoscalerConfig", "name")
		require.Equal(t, manifest.KedaHPAName(longServiceName), hpaName)
		require.Equal(t, manualPause, getScaledObject(t, p, longServiceName).GetAnnotations())
	})
}

func TestServiceUpdateCountResumesManualPause(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, manualPause), func(p *k8s.Provider, _ *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(3)}))

		so := getScaledObject(t, p, "web")
		require.Empty(t, so.GetAnnotations())
		lo, hi := scaledObjectBounds(so)
		require.Equal(t, int64(3), lo)
		require.Equal(t, int64(3), hi)
		require.Equal(t, 1, scaledObjectPatchCount(dyn))
	})
}

func TestServiceUpdateRangeResumesManualPause(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, manualPause), func(p *k8s.Provider, _ *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Min: options.Int(2), Max: options.Int(6)}))

		so := getScaledObject(t, p, "web")
		require.Empty(t, so.GetAnnotations())
		lo, hi := scaledObjectBounds(so)
		require.Equal(t, int64(2), lo)
		require.Equal(t, int64(6), hi)
		require.Equal(t, 1, scaledObjectPatchCount(dyn))
	})
}

func TestServiceUpdateCountNullsOnlyTheAnnotationsItRead(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		want        map[string]interface{}
	}{
		{"marker only", markerOnly, map[string]interface{}{structs.KedaManualPauseAnnotation: nil}},
		{"budget pause", budgetPause, map[string]interface{}{structs.KedaPausedReplicasAnnotation: nil}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pauseTestProvider(t, pausedScaledObject("web", 1, 5, tc.annotations), func(p *k8s.Provider, _ *kfake.Clientset, dyn *fake.FakeDynamicClient) {
				require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(2)}))

				require.Equal(t, tc.want, firstScaledObjectPatchAnnotations(t, dyn))
			})
		})
	}
}

func TestServiceUpdatePauseAndResumeEventReasons(t *testing.T) {
	const (
		boundsReason = "KEDA ScaledObject owns replicas; patched scaledobject spec.minReplicaCount / spec.maxReplicaCount instead of deployment replicas"
		pauseReason  = "KEDA ScaledObject owns replicas; paused the scaledobject instead of setting deployment replicas to 0"
	)
	var (
		mu       sync.Mutex
		captured []map[string]any
	)
	srv := webhookCaptureServer(&mu, &captured)
	t.Cleanup(srv.Close)
	reasons := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := []string{}
		for _, e := range findAllByAction(captured, "release:imperative-patch-note") {
			data, _ := e["data"].(map[string]any)
			reason, _ := data["reason"].(string)
			out = append(out, reason)
		}
		return out
	}

	pauseTestProvider(t, pausedScaledObject("web", 1, 5, nil), func(p *k8s.Provider, _ *kfake.Clientset, _ *fake.FakeDynamicClient) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		for _, count := range []int{0, 0, 3, 2} {
			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(count)}))
		}

		require.Eventually(t, func() bool { return len(reasons()) >= 3 }, 5*time.Second, 20*time.Millisecond)
		drainPendingDispatches()
	})

	require.ElementsMatch(t, []string{pauseReason, boundsReason + " and removed the pause", boundsReason}, reasons())
}

func TestServiceUpdateMarkerWithoutPausedReplicas(t *testing.T) {
	t.Run("count 0 writes both annotations", func(t *testing.T) {
		pauseTestProvider(t, pausedScaledObject("web", 1, 5, markerOnly), func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(0)}))

			require.Equal(t, manualPause, getScaledObject(t, p, "web").GetAnnotations())
			require.Equal(t, 1, scaledObjectPatchCount(dyn))
			require.Equal(t, int32(2), deploymentReplicas(t, kk, "web"))
		})
	})

	t.Run("count 2 removes the marker", func(t *testing.T) {
		pauseTestProvider(t, pausedScaledObject("web", 1, 5, markerOnly), func(p *k8s.Provider, _ *kfake.Clientset, _ *fake.FakeDynamicClient) {
			require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(2)}))

			so := getScaledObject(t, p, "web")
			require.Empty(t, so.GetAnnotations())
			lo, hi := scaledObjectBounds(so)
			require.Equal(t, int64(2), lo)
			require.Equal(t, int64(2), hi)
		})
	})
}

func TestServiceUpdateRefusesResumeWhileBudgetClaimsService(t *testing.T) {
	shutdownAt := time.Now().UTC().Add(-time.Minute)
	states := map[string]func(*structs.AppBudgetShutdownState){
		"armed":    func(*structs.AppBudgetShutdownState) {},
		"in force": func(s *structs.AppBudgetShutdownState) { s.ShutdownAt = &shutdownAt },
	}
	pauses := map[string]map[string]string{"budget pause": budgetPause, "manual pause": manualPause}
	commands := map[string]structs.ServiceUpdateOptions{
		"count":   {Count: options.Int(2)},
		"min max": {Min: options.Int(2), Max: options.Int(6)},
	}

	for stateName, mutate := range states {
		for pauseName, annotations := range pauses {
			for commandName, opts := range commands {
				t.Run(stateName+"/"+pauseName+"/"+commandName, func(t *testing.T) {
					pauseTestProvider(t, pausedScaledObject("web", 1, 5, annotations), func(p *k8s.Provider, kk *kfake.Clientset, dyn *fake.FakeDynamicClient) {
						state := pauseShutdownState("web")
						mutate(state)
						require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
						before, err := json.Marshal(getScaledObject(t, p, "web").Object)
						require.NoError(t, err)

						err = p.ServiceUpdate("app1", "web", opts)

						var herr *structs.HttpError
						require.True(t, errors.As(err, &herr), "want a conflict error, got %v", err)
						require.Equal(t, 409, herr.Code())
						require.Contains(t, herr.Error(), "convox budget reset app1")
						after, err := json.Marshal(getScaledObject(t, p, "web").Object)
						require.NoError(t, err)
						require.Equal(t, string(before), string(after))
						require.Zero(t, scaledObjectPatchCount(dyn))
						require.Equal(t, int32(2), deploymentReplicas(t, kk, "web"))
					})
				})
			}
		}
	}
}

func TestServiceUpdateResumesWhenBudgetDoesNotClaimService(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name  string
		state func() *structs.AppBudgetShutdownState
	}{
		{"expired", func() *structs.AppBudgetShutdownState {
			s := pauseShutdownState("web")
			s.ShutdownAt = &now
			s.ExpiredAt = &now
			return s
		}},
		{"restored", func() *structs.AppBudgetShutdownState {
			s := pauseShutdownState("web")
			s.ShutdownAt = &now
			s.RestoredAt = &now
			return s
		}},
		{"service not listed", func() *structs.AppBudgetShutdownState {
			s := pauseShutdownState("worker")
			s.ShutdownAt = &now
			return s
		}},
		{"no state", func() *structs.AppBudgetShutdownState { return nil }},
	}
	pauses := map[string]map[string]string{"budget pause": budgetPause, "manual pause": manualPause}

	for _, tc := range cases {
		for pauseName, annotations := range pauses {
			t.Run(tc.name+"/"+pauseName, func(t *testing.T) {
				pauseTestProvider(t, pausedScaledObject("web", 1, 5, annotations), func(p *k8s.Provider, _ *kfake.Clientset, _ *fake.FakeDynamicClient) {
					if state := tc.state(); state != nil {
						require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))
					}

					require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(2)}))

					so := getScaledObject(t, p, "web")
					require.Empty(t, so.GetAnnotations())
					lo, hi := scaledObjectBounds(so)
					require.Equal(t, int64(2), lo)
					require.Equal(t, int64(2), hi)
				})
			})
		}
	}
}

func TestServiceUpdateClaimedServiceNotPausedScalesAsBefore(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, nil), func(p *k8s.Provider, _ *kfake.Clientset, dyn *fake.FakeDynamicClient) {
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", pauseShutdownState("web")))

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(2)}))

		so := getScaledObject(t, p, "web")
		lo, hi := scaledObjectBounds(so)
		require.Equal(t, int64(2), lo)
		require.Equal(t, int64(2), hi)
		require.Empty(t, so.GetAnnotations())
		require.Equal(t, 1, scaledObjectPatchCount(dyn))
	})
}

func TestServiceUpdateCountZeroWhileClaimedMarksBudgetPause(t *testing.T) {
	pauseTestProvider(t, pausedScaledObject("web", 1, 5, budgetPause), func(p *k8s.Provider, _ *kfake.Clientset, _ *fake.FakeDynamicClient) {
		shutdownAt := time.Now().UTC()
		state := pauseShutdownState("web")
		state.ShutdownAt = &shutdownAt
		require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", state))

		require.NoError(t, p.ServiceUpdate("app1", "web", structs.ServiceUpdateOptions{Count: options.Int(0)}))

		require.Equal(t, manualPause, getScaledObject(t, p, "web").GetAnnotations())
	})
}

func TestServiceListReportsPausedScaledObject(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		paused      *bool
		coldStart   *bool
	}{
		{"manual pause", manualPause, options.Bool(true), nil},
		{"budget pause", budgetPause, options.Bool(true), nil},
		{"not paused", nil, nil, options.Bool(true)},
		{"marker only", markerOnly, nil, options.Bool(true)},
		{"paused at 2", map[string]string{structs.KedaPausedReplicasAnnotation: "2"}, options.Bool(true), nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*kfake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				scaleSeedDeployment(t, kk, "rack1-app1", "web", 0, "")
				scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 0})
				p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), pausedScaledObject("web", 0, 5, tc.annotations))

				ss, err := p.ServiceList("app1")

				require.NoError(t, err)
				require.Len(t, ss, 1)
				require.Equal(t, tc.paused, ss[0].Paused)
				require.Equal(t, tc.coldStart, ss[0].ColdStart)
			})
		})
	}
}
