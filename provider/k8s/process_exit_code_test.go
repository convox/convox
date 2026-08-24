package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func podWithContainerState(phase ac.PodPhase, state ac.ContainerState) ac.Pod {
	return ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:   "web-abc",
			Labels: map[string]string{"app": "app1", "service": "web"},
		},
		Spec:   ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}},
		Status: ac.PodStatus{Phase: phase, ContainerStatuses: []ac.ContainerStatus{{Name: "app1", State: state}}},
	}
}

func TestProcessFromPod_ExitCode(t *testing.T) {
	p := &Provider{Name: "rack1"}

	tests := []struct {
		name  string
		pod   ac.Pod
		want  *int
		state string
	}{
		{
			name:  "terminated container reports its code",
			pod:   podWithContainerState(ac.PodFailed, ac.ContainerState{Terminated: &ac.ContainerStateTerminated{ExitCode: 3}}),
			want:  intp(3),
			state: "failed",
		},
		{
			name:  "clean exit reports zero rather than nil",
			pod:   podWithContainerState(ac.PodSucceeded, ac.ContainerState{Terminated: &ac.ContainerStateTerminated{ExitCode: 0}}),
			want:  intp(0),
			state: "complete",
		},
		{
			name:  "running container has none",
			pod:   podWithContainerState(ac.PodRunning, ac.ContainerState{Running: &ac.ContainerStateRunning{}}),
			state: "running",
		},
		{
			name:  "crash looping container has none",
			pod:   podWithContainerState(ac.PodRunning, ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}),
			state: "crashed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := p.processFromPod(tt.pod)
			require.NoError(t, err)
			require.Equal(t, tt.state, ps.Status)

			if tt.want == nil {
				require.Nil(t, ps.ExitCode)
				return
			}

			require.NotNil(t, ps.ExitCode)
			require.Equal(t, *tt.want, *ps.ExitCode)
		})
	}
}

func TestProcessFromPod_ExitCodeAbsentWithoutContainerStatus(t *testing.T) {
	p := &Provider{Name: "rack1"}

	pod := ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: "web-abc", Labels: map[string]string{"app": "app1", "service": "web"}},
		Spec:       ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}},
		Status:     ac.PodStatus{Phase: ac.PodFailed},
	}

	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "failed", ps.Status)
	require.Nil(t, ps.ExitCode)
}

func intp(i int) *int {
	return &i
}
