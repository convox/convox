package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pendingPod(status ac.PodStatus) ac.Pod {
	return ac.Pod{
		ObjectMeta: am.ObjectMeta{Name: "p1", Labels: map[string]string{"app": "app1", "service": "web"}},
		Spec:       ac.PodSpec{Containers: []ac.Container{{Name: "app1"}}},
		Status:     status,
	}
}

func TestProcessFromPod_Reason(t *testing.T) {
	p := &Provider{}
	long := strings.Repeat("x", 400)
	tests := []struct {
		name   string
		status ac.PodStatus
		want   string
	}{
		{
			name: "unschedulable",
			status: ac.PodStatus{Phase: ac.PodPending, Conditions: []ac.PodCondition{
				{Type: ac.PodScheduled, Status: ac.ConditionFalse, Reason: "Unschedulable", Message: "0/3 nodes are available: 3 Insufficient memory."},
			}},
			want: "unschedulable: 0/3 nodes are available: 3 Insufficient memory.",
		},
		{
			name: "image pull back-off",
			status: ac.PodStatus{Phase: ac.PodPending, ContainerStatuses: []ac.ContainerStatus{
				{Name: "app1", State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: `Back-off pulling image "x:1"`}}},
			}},
			want: `ImagePullBackOff: Back-off pulling image "x:1"`,
		},
		{
			name: "init container image pull",
			status: ac.PodStatus{Phase: ac.PodPending, InitContainerStatuses: []ac.ContainerStatus{
				{Name: "seed", State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "ErrImagePull"}}},
			}},
			want: "ErrImagePull",
		},
		{
			name: "normal start has no reason",
			status: ac.PodStatus{Phase: ac.PodPending, ContainerStatuses: []ac.ContainerStatus{
				{Name: "app1", State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "ContainerCreating"}}},
			}},
			want: "",
		},
		{
			name: "running pods never carry a reason",
			status: ac.PodStatus{Phase: ac.PodRunning, Conditions: []ac.PodCondition{
				{Type: ac.PodScheduled, Status: ac.ConditionFalse, Message: "stale"},
			}},
			want: "",
		},
		{
			name: "long messages are cut",
			status: ac.PodStatus{Phase: ac.PodPending, Conditions: []ac.PodCondition{
				{Type: ac.PodScheduled, Status: ac.ConditionFalse, Message: long},
			}},
			want: ("unschedulable: " + long)[:pendingReasonMax],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps, err := p.processFromPod(pendingPod(tt.status))
			require.NoError(t, err)
			require.Equal(t, tt.want, ps.Reason)
		})
	}
}
