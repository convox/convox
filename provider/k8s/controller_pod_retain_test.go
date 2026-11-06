package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func retainPod(annotation string, finished time.Time) *ac.Pod {
	p := &ac.Pod{ObjectMeta: am.ObjectMeta{Name: "web-abc", Namespace: "ns1"}}

	if annotation != "" {
		p.Annotations = map[string]string{AnnotationProcessRetain: annotation}
	}

	if !finished.IsZero() {
		p.Status.ContainerStatuses = []ac.ContainerStatus{
			{
				Name:  "app1",
				State: ac.ContainerState{Terminated: &ac.ContainerStateTerminated{FinishedAt: am.NewTime(finished)}},
			},
		}
	}

	return p
}

func TestCleanupDelay(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		annotation string
		finished   time.Time
		want       time.Duration
	}{
		{name: "no annotation keeps today's grace", want: podCleanupDelay},
		{name: "unparseable falls back", annotation: "soon", finished: now, want: podCleanupDelay},
		{name: "zero falls back", annotation: "0", finished: now, want: podCleanupDelay},
		{name: "negative falls back", annotation: "-30", finished: now, want: podCleanupDelay},
		{name: "overflow falls back", annotation: "999999999999999999999", finished: now, want: podCleanupDelay},
		{name: "in range runs from the container exit", annotation: "120", finished: now.Add(-30 * time.Second), want: 90 * time.Second},
		{name: "clamped to the ceiling", annotation: "99999", finished: now, want: podRetainMaxSeconds * time.Second},
		{name: "elapsed window still gets the grace", annotation: "60", finished: now.Add(-10 * time.Minute), want: podCleanupDelay},
		{name: "no container status runs from now", annotation: "120", want: 120 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, cleanupDelay(retainPod(tt.annotation, tt.finished), now))
		})
	}
}
