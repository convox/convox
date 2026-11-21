package k8s_test

import (
	"testing"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestReleaseTemplateServicesPausedReplicas(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		releaseID   string
		claimed     bool
		want        int32
	}{
		{"manual pause, new release", pauseAnnosManual, "RELEASE2", false, 1},
		{"manual pause, same release", pauseAnnosManual, "RELEASE1", false, 0},
		{"manual pause, new release, budget claims service", pauseAnnosManual, "RELEASE2", true, 0},
		{"budget pause, new release", pauseAnnosBudget, "RELEASE2", false, 0},
		{"not paused, new release", nil, "RELEASE2", false, 1},
		{"not paused, same release", nil, "RELEASE1", false, 1},
		{"marker only, same release", pauseAnnosMarker, "RELEASE1", false, 1},
		{"paused at 2, same release", map[string]string{structs.KedaPausedReplicasAnnotation: "2"}, "RELEASE1", false, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*kfake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				scaleSeedDeployment(t, kk, "rack1-app1", "web", 0, "")
				scaleSeedAppRelease(t, p, "rack1-app1", "release1", map[string]int{"web": 1})
				p.IsKedaEnabled = true
				p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), pausedScaledObject("web", 1, 6, tc.annotations))
				if tc.claimed {
					require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", pauseShutdownState("web")))
				}

				out, err := k8s.ReleaseTemplateServicesForTest(p,
					&structs.App{Name: "app1", Release: "release1"},
					structs.Environment{},
					&structs.Release{Id: tc.releaseID, App: "app1"},
					scaleManifestServices(t, map[string]int{"web": 1}),
					structs.ReleasePromoteOptions{})

				require.NoError(t, err)
				require.Equal(t, tc.want, k8s.RenderedReplicasForTest(t, out, "web"))
			})
		})
	}
}

func TestResumeManualPauses(t *testing.T) {
	shutdownAt := time.Now().UTC()
	cases := []struct {
		name        string
		annotations map[string]string
		override    string
		state       *structs.AppBudgetShutdownState
		want        map[string]string
	}{
		{"manual pause", pauseAnnosManual, "", nil, map[string]string{}},
		{"manual pause, budget restored", pauseAnnosManual, "", func() *structs.AppBudgetShutdownState {
			s := pauseShutdownState("web")
			s.ShutdownAt = &shutdownAt
			s.RestoredAt = &shutdownAt
			return s
		}(), map[string]string{}},
		{"manual pause, scale override on", pauseAnnosManual, k8s.ServiceScaleOverrideValueOn, nil, pauseAnnosManual},
		{"manual pause, budget armed", pauseAnnosManual, "", pauseShutdownState("web"), pauseAnnosManual},
		{"budget pause", pauseAnnosBudget, "", nil, pauseAnnosBudget},
		{"marker only", pauseAnnosMarker, "", nil, map[string]string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*kfake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				scaleSeedDeployment(t, kk, "rack1-app1", "web", 0, tc.override)
				p.IsKedaEnabled = true
				p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), pausedScaledObject("web", 1, 6, tc.annotations))
				if tc.state != nil {
					require.NoError(t, k8s.WriteBudgetShutdownStateAnnotationForTest(p, "app1", tc.state))
				}

				k8s.ResumeManualPausesForTest(p, "app1")

				got := pauseGetScaledObject(t, p, "web").GetAnnotations()
				if len(tc.want) == 0 {
					require.Empty(t, got)
				} else {
					require.Equal(t, tc.want, got)
				}
			})
		})
	}
}

func TestResumeManualPausesSetsLongHPAName(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk, _ := p.Cluster.(*kfake.Clientset)
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		scaleSeedDeployment(t, kk, "rack1-app1", longServiceName, 0, "")
		p.IsKedaEnabled = true
		p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), pausedScaledObject(longServiceName, 1, 6, pauseAnnosManual))

		k8s.ResumeManualPausesForTest(p, "app1")

		require.Empty(t, pauseGetScaledObject(t, p, longServiceName).GetAnnotations())
		hpaName, _, _ := unstructured.NestedString(scaledObjectAdvanced(t, p, "rack1-app1", longServiceName), "horizontalPodAutoscalerConfig", "name")
		require.Equal(t, manifest.KedaHPAName(longServiceName), hpaName)
	})
}
