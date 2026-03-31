package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
)

const (
	reconcileInterval       = 5 * time.Minute
	runningPodGracePeriod   = 5 * time.Minute
	createdBuildGracePeriod = 30 * time.Minute
)

type buildReconciler struct {
	provider *Provider
	cluster  kubernetes.Interface
	convox   cv.Interface
	ctx      context.Context
	logger   *logger.Logger
}

func (r *buildReconciler) Run() error {
	r.logger.Logf("starting build status reconciliation...")

	apps, err := r.provider.AppList()
	if err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	for _, app := range apps {
		if err := r.reconcileApp(app.Name); err != nil {
			r.logger.Logf("build reconcile error for app %s: %s", app.Name, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	r.logger.Logf("finished build status reconciliation")
	return nil
}

func (r *buildReconciler) reconcileApp(app string) error {
	ns := r.provider.AppNamespace(app)

	buildList, err := r.convox.ConvoxV1().Builds(ns).List(am.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list builds in %s: %w", ns, err)
	}

	now := time.Now().UTC()

	for i := range buildList.Items {
		kb := &buildList.Items[i]

		b, err := r.provider.buildUnmarshal(kb)
		if err != nil {
			r.logger.Logf("failed to unmarshal build %s/%s: %s", ns, kb.Name, err)
			continue
		}

		switch b.Status {
		case "running":
			r.reconcileRunningBuild(app, b, now)
		case "created":
			r.reconcileCreatedBuild(app, b, now)
		}

		time.Sleep(50 * time.Millisecond)
	}

	return nil
}

func (r *buildReconciler) reconcileRunningBuild(app string, b *structs.Build, now time.Time) {
	if b.Process == "" {
		if now.Sub(b.Started) > createdBuildGracePeriod {
			r.markBuildFailed(app, b, "build marked as running but has no process ID")
		}
		return
	}

	ns := r.provider.AppNamespace(app)
	pod, err := r.cluster.CoreV1().Pods(ns).Get(r.ctx, b.Process, am.GetOptions{})

	if ae.IsNotFound(err) {
		if now.Sub(b.Started) > runningPodGracePeriod {
			r.markBuildFailed(app, b, "build pod no longer exists")
		}
		return
	}

	if err != nil {
		r.logger.Logf("failed to get pod %s for build %s: %s", b.Process, b.Id, err)
		return
	}

	switch pod.Status.Phase {
	case ac.PodRunning, ac.PodPending:
		return
	case ac.PodSucceeded, ac.PodFailed:
		terminated := podTerminationTime(pod)
		if terminated.IsZero() || now.Sub(terminated) > runningPodGracePeriod {
			reason := fmt.Sprintf("build pod terminated with phase %s", pod.Status.Phase)
			r.markBuildFailed(app, b, reason)
		}
	default:
		r.logger.Logf("build %s pod %s has unknown phase: %s", b.Id, b.Process, pod.Status.Phase)
	}
}

func (r *buildReconciler) reconcileCreatedBuild(app string, b *structs.Build, now time.Time) {
	if now.Sub(b.Started) > createdBuildGracePeriod {
		r.markBuildFailed(app, b, "build did not start within expected time")
	}
}

func (r *buildReconciler) markBuildFailed(app string, b *structs.Build, reason string) {
	r.logger.Logf("reconciling orphaned build %s (app: %s, status: %s): %s", b.Id, app, b.Status, reason)

	ended := time.Now().UTC()
	_, err := r.provider.BuildUpdate(app, b.Id, structs.BuildUpdateOptions{
		Status: options.String("failed"),
		Ended:  &ended,
	})
	if err != nil {
		r.logger.Logf("failed to update build %s status: %s", b.Id, err)
	}
}

func podTerminationTime(pod *ac.Pod) time.Time {
	var latest time.Time

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			t := cs.State.Terminated.FinishedAt.Time
			if t.After(latest) {
				latest = t
			}
		}
	}

	if latest.IsZero() {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == ac.PodReady && cond.Status == ac.ConditionFalse {
				t := cond.LastTransitionTime.Time
				if t.After(latest) {
					latest = t
				}
			}
		}
	}

	return latest
}
