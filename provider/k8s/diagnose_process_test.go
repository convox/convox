package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func diagnosePod(t *testing.T, kk *fake.Clientset, name, podType string, phase ac.PodPhase) {
	t.Helper()

	_, err := kk.CoreV1().Pods("rack1-app1").Create(context.TODO(), &ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:      name,
			Namespace: "rack1-app1",
			Labels:    map[string]string{"system": "convox", "app": "app1", "service": "web", "type": podType},
		},
		Spec:   ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}},
		Status: ac.PodStatus{Phase: phase},
	}, am.CreateOptions{})
	require.NoError(t, err)
}

func TestDiagnosePodsSkipsCompletedRuns(t *testing.T) {
	p, kk, _ := minimalProvider(t)

	diagnosePod(t, kk, "web-done", "process", ac.PodSucceeded)

	pods, summary, err := p.diagnosePods("rack1-app1", structs.AppDiagnoseOptions{})
	require.NoError(t, err)
	require.Empty(t, pods)
	require.Equal(t, 0, summary.Total)
	require.Equal(t, 0, summary.Unhealthy)
}

func TestDiagnosePodsKeepsFailedRunsAndServices(t *testing.T) {
	p, kk, _ := minimalProvider(t)

	diagnosePod(t, kk, "web-failed", "process", ac.PodFailed)
	diagnosePod(t, kk, "web-stopped", "service", ac.PodSucceeded)

	pods, summary, err := p.diagnosePods("rack1-app1", structs.AppDiagnoseOptions{})
	require.NoError(t, err)
	require.Len(t, pods, 2)
	require.Equal(t, 2, summary.Total)
	require.Equal(t, 2, summary.Unhealthy)
}
