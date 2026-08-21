package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const progressDeadlineExceededReason = "ProgressDeadlineExceeded"

func effectiveProgressDeadline(s *manifest.Service, rackDefault int) int {
	if s.Deployment.ProgressDeadline != 0 {
		return s.Deployment.ProgressDeadline
	}
	return rackDefault
}

func effectiveCrashRestartLimit(s *manifest.Service, rackDefault int) int {
	if s.Deployment.CrashRestartLimit != 0 {
		return s.Deployment.CrashRestartLimit
	}
	return rackDefault
}

// Zero stays zero: racks whose api module passes no tuning env read unset as 0.
func clampProgressDeadline(v int) int {
	switch {
	case v == 0:
		return 0
	case v < manifest.ProgressDeadlineFloor:
		return manifest.ProgressDeadlineFloor
	case v > manifest.ProgressDeadlineCeiling:
		return manifest.ProgressDeadlineCeiling
	}
	return v
}

// Only Deployments carry progressDeadlineSeconds, so agents and stateful services
// get no deadline entry. Stateful ones stay in maxDeadline: the Atom's deadline
// is the only clock their progressDeadline can move.
func (p *Provider) fastFailStateForPromote(ss manifest.Services) (deadlines, limits map[string]int, maxDeadline int) {
	for i := range ss {
		s := &ss[i]

		deadline := clampProgressDeadline(effectiveProgressDeadline(s, p.DeployProgressDeadline))
		if !s.Agent.Enabled {
			if deadline > maxDeadline {
				maxDeadline = deadline
			}
			if deadline > 0 && deadline < manifest.DefaultProgressDeadlineSeconds && !s.Stateful {
				if deadlines == nil {
					deadlines = map[string]int{}
				}
				deadlines[s.Name] = deadline
			}
		}

		if limit := effectiveCrashRestartLimit(s, p.DeployCrashRestartLimit); limit > 0 {
			if limits == nil {
				limits = map[string]int{}
			}
			limits[s.Name] = limit
		}
	}

	return deadlines, limits, maxDeadline
}

// The generation gate proves the status caught up with this promote's apply, the
// timestamp gate separates a fresh verdict from the previous rollout's.
func deadlineExceededService(deps []appsv1.Deployment, opted map[string]int, startedAt time.Time) string {
	for i := range deps {
		d := &deps[i]

		if d.Status.ObservedGeneration < d.Generation {
			continue
		}

		svc := d.Labels["service"]
		if _, ok := opted[svc]; !ok {
			continue
		}

		for j := range d.Status.Conditions {
			c := &d.Status.Conditions[j]
			switch {
			case c.Type != appsv1.DeploymentProgressing, c.Status != corev1.ConditionFalse:
			case c.Reason != progressDeadlineExceededReason:
			case c.LastUpdateTime.Time.Before(startedAt):
			default:
				return svc
			}
		}
	}

	return ""
}

func crashRestartExceeded(pods []corev1.Pod, limits map[string]int, startedAt time.Time) (svc string, restarts, limit int) {
	for i := range pods {
		pod := &pods[i]

		if pod.DeletionTimestamp != nil {
			continue
		}

		// unset until the kubelet admits the pod, which reports no restarts anyway
		if st := pod.Status.StartTime; st != nil && st.Time.Before(startedAt) {
			continue
		}

		limit = limits[pod.Labels["service"]]
		if limit <= 0 {
			continue
		}

		restarts = 0
		for _, css := range [][]corev1.ContainerStatus{pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses} {
			for j := range css {
				if r := int(css[j].RestartCount); r > restarts {
					restarts = r
				}
			}
		}

		if restarts > limit {
			return pod.Labels["service"], restarts, limit
		}
	}

	return "", 0, 0
}

// rollback() swaps CurrentVersion before the status is written, so these are the
// statuses the app-release mirror can pair with the previous release id.
func isRollbackStatus(atomStatus string) bool {
	status, _, terminal := mapAppStatusToWatchResult(atomStatus)
	return terminal && status == "error"
}

func bailReasonOverridesError(errMsg string) bool {
	switch errMsg {
	case "cancelled", "deadline-exceeded", "rollback: Error", "rollback: Rollback",
		"rollout-failed: Failure", "rollout-failed: Reverted":
		return true
	}
	return false
}

func (p *Provider) deployFastFailArmed(state *structs.ReleasePromoteWatchState) bool {
	if p.FeatureGates[options.FeatureGateDeployFastFailDisable] {
		return false
	}
	return len(state.ProgressDeadlines) > 0 || len(state.CrashRestartLimits) > 0
}

func (p *Provider) deployFastFailReason(app string, state *structs.ReleasePromoteWatchState) string {
	ns := p.AppNamespace(app)

	if len(state.ProgressDeadlines) > 0 {
		deps, err := p.fastFailDeployments(ns, fmt.Sprintf("app=%s,type=service,release=%s", app, state.ReleaseID))
		if err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=fast_fail_deployments app=%s err=%q\n", app, err)
		} else if svc := deadlineExceededService(deps, state.ProgressDeadlines, state.StartedAt); svc != "" {
			return "progress-deadline-exceeded: service=" + svc
		}
	}

	if len(state.CrashRestartLimits) > 0 {
		// the pod template renders the upper-cased release id
		selector := fmt.Sprintf("system=convox,rack=%s,app=%s,type=service,release=%s",
			p.Name, app, strings.ToUpper(state.ReleaseID))
		pods, err := p.fastFailPods(ns, selector)
		if err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=fast_fail_pods app=%s err=%q\n", app, err)
		} else if svc, restarts, limit := crashRestartExceeded(pods, state.CrashRestartLimits, state.StartedAt); svc != "" {
			return fmt.Sprintf("crash-restart-limit: service=%s restarts=%d limit=%d", svc, restarts, limit)
		}
	}

	return ""
}

// Read directly rather than through the shared helpers, whose unconditional log
// line would emit twice per tick for a whole rollout.
func (p *Provider) fastFailDeployments(ns, selector string) ([]appsv1.Deployment, error) {
	if p.deploymentInformer == nil {
		list, err := p.ListDeploymentsFromInformer(ns, selector)
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}
	sel, err := labels.Parse(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ds, err := p.deploymentInformer.Lister().Deployments(ns).List(sel)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out := make([]appsv1.Deployment, len(ds))
	for i := range ds {
		out[i] = *ds[i]
	}
	return out, nil
}

func (p *Provider) fastFailPods(ns, selector string) ([]corev1.Pod, error) {
	if p.podInformer == nil {
		list, err := p.ListPodsFromInformer(ns, selector)
		if err != nil {
			return nil, err
		}
		return list.Items, nil
	}
	sel, err := labels.Parse(selector)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ps, err := p.podInformer.Lister().Pods(ns).List(sel)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	out := make([]corev1.Pod, len(ps))
	for i := range ps {
		out[i] = *ps[i]
	}
	return out, nil
}

// Reports whether the bail is settled, by this process or a peer. A transient
// failure returns false so the next tick retries.
func (p *Provider) bailReleasePromote(ctx context.Context, app string, state *structs.ReleasePromoteWatchState, reason string) bool {
	claimed, err := p.claimReleasePromoteBail(ctx, app, state)
	if err != nil {
		fmt.Printf("ns=release_watcher at=warn kind=bail_claim_failed app=%s err=%q\n", app, err)
		return false
	}
	if !claimed {
		fmt.Printf("ns=release_watcher at=info kind=bail_claimed_by_peer app=%s id=%s\n", app, state.ReleaseID)
		return true
	}

	fmt.Printf("ns=release_watcher at=bail app=%s id=%s reason=%q\n", app, state.ReleaseID, reason)

	if err := p.AppCancel(app); err != nil {
		fmt.Printf("ns=release_watcher at=warn kind=bail_cancel app=%s err=%q\n", app, err)
	}

	return true
}
