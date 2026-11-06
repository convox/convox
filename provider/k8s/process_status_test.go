package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// processFromPod classifies a pod's runtime state into a single status
// string surfaced through `convox ps`. The Ready=False branch needs to
// distinguish two cases the kubelet conflates by setting Ready=False:
//
//   - Initial startup: the readiness probe has not had a chance to run
//     yet (initialDelaySeconds, container creating, init containers
//     pending). The pod is *not* unhealthy — it is in the normal
//     startup window. K8s exposes this via Condition.Reason values
//     like "ContainersNotReady" / "PodInitializing" / "ContainerCreating".
//
//   - Genuine probe failure: the readiness probe ran and failed, or
//     the container is crashing. This IS unhealthy.
//
// Conflating the two paints brand-new pods as "unhealthy" for the
// initial-delay window during a normal promote, which leaks into the
// CLI/Console as a spurious red blip. The classifier inspects
// Condition.Reason and routes startup reasons to "starting"; unknown
// or empty reasons stay on the conservative "unhealthy" verdict.

func newPodWithReady(name string, phase ac.PodPhase, ready ac.ConditionStatus, reason string) ac.Pod {
	return ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"app": "app1", "service": "web"},
		},
		Spec: ac.PodSpec{
			Containers: []ac.Container{{Name: "app1"}},
		},
		Status: ac.PodStatus{
			Phase: phase,
			Conditions: []ac.PodCondition{
				{Type: ac.PodReady, Status: ready, Reason: reason},
			},
		},
	}
}

func TestProcessFromPod_Status_StartupReasonsClassifiedAsStarting(t *testing.T) {
	p := &Provider{Name: "rack1"}

	cases := []struct {
		name   string
		reason string
	}{
		{"ContainersNotReady", "ContainersNotReady"},
		{"PodInitializing", "PodInitializing"},
		{"ContainerCreating", "ContainerCreating"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := newPodWithReady("web-abc", ac.PodRunning, ac.ConditionFalse, tc.reason)
			ps, err := p.processFromPod(pod)
			require.NoError(t, err)
			require.Equal(t, "starting", ps.Status,
				"Ready=False with startup reason %q must classify as starting, got %q", tc.reason, ps.Status)
		})
	}
}

func TestProcessFromPod_Status_ProbeFailureStillUnhealthy(t *testing.T) {
	p := &Provider{Name: "rack1"}

	// "" (empty reason) is the conservative case — kubelets prior to ~1.20
	// set Ready=False without populating Reason. We must NOT mask an
	// unknown negative state as "starting"; default to "unhealthy".
	cases := []struct {
		name   string
		reason string
	}{
		{"empty reason (older kubelet)", ""},
		{"ProbeFailure", "ProbeFailure"},
		{"ReadinessProbeFailed", "ReadinessProbeFailed"},
		{"unknown reason", "SomeFutureReason"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pod := newPodWithReady("web-abc", ac.PodRunning, ac.ConditionFalse, tc.reason)
			ps, err := p.processFromPod(pod)
			require.NoError(t, err)
			require.Equal(t, "unhealthy", ps.Status,
				"Ready=False with reason %q must classify as unhealthy, got %q", tc.reason, ps.Status)
		})
	}
}

func TestProcessFromPod_Status_ReadyTrueIsRunning(t *testing.T) {
	p := &Provider{Name: "rack1"}

	pod := newPodWithReady("web-abc", ac.PodRunning, ac.ConditionTrue, "")
	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "running", ps.Status)
}

func TestProcessFromPod_Status_PendingPhaseStaysPending(t *testing.T) {
	p := &Provider{Name: "rack1"}

	// A Pending-phase pod with no Ready condition stays "pending" —
	// the classifier's Phase switch handles this; the Conditions loop
	// only escalates to unhealthy/starting if an explicit Ready=False
	// is present. Pending without conditions is the canonical
	// "pod has not been admitted yet" shape.
	pod := ac.Pod{
		ObjectMeta: am.ObjectMeta{
			Name:   "web-abc",
			Labels: map[string]string{"app": "app1", "service": "web"},
		},
		Spec: ac.PodSpec{
			Containers: []ac.Container{{Name: "app1"}},
		},
		Status: ac.PodStatus{Phase: ac.PodPending},
	}
	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "pending", ps.Status)
}

func TestProcessFromPod_Status_PendingPhaseWithStartupReasonStillPending(t *testing.T) {
	p := &Provider{Name: "rack1"}

	// Defensive: a Pending-phase pod that already carries Ready=False
	// with a startup reason (e.g. ContainersNotReady) should remain
	// "pending" — the more specific Pending classification beats the
	// startup-reason refinement (the pod hasn't even been admitted yet,
	// nothing to "start" yet).
	pod := newPodWithReady("web-abc", ac.PodPending, ac.ConditionFalse, "ContainersNotReady")
	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "pending", ps.Status,
		"Pending-phase pod with startup reason must stay pending, got %q", ps.Status)
}

func TestProcessFromPod_Status_FailedPhaseStaysFailed(t *testing.T) {
	p := &Provider{Name: "rack1"}

	// A Failed pod with Ready=False (which the kubelet sets on Failed
	// pods) must stay "failed" — the existing skip-when-failed guard
	// in the Conditions loop must continue to hold for the new
	// startup-reason path too.
	pod := newPodWithReady("web-abc", ac.PodFailed, ac.ConditionFalse, "")
	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "failed", ps.Status)
}

func TestProcessFromPod_Status_CrashLoopBackoffStillCrashed(t *testing.T) {
	p := &Provider{Name: "rack1"}

	// CrashLoopBackOff is reported via ContainerStatuses[].State.Waiting.Reason,
	// which runs *after* the Ready-condition branch. The container-status
	// signal must still win — a pod stuck crashing must not be
	// re-classified as "starting" even if its Ready condition reason
	// happens to be a startup string.
	pod := newPodWithReady("web-abc", ac.PodRunning, ac.ConditionFalse, "ContainersNotReady")
	pod.Status.ContainerStatuses = []ac.ContainerStatus{
		{
			Name:  "app1",
			State: ac.ContainerState{Waiting: &ac.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		},
	}
	ps, err := p.processFromPod(pod)
	require.NoError(t, err)
	require.Equal(t, "crashed", ps.Status)
}
