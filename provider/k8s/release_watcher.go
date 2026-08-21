package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// vars (not const) so tests can override timing without real wall-clock waits.
var (
	releasePromoteWatchPollInterval        = 3 * time.Second
	releasePromoteWatchGracePeriod         = 30 * time.Second
	releasePromoteWatchGCTickInterval      = 5 * time.Minute // overridable via RELEASE_WATCHER_GC_INTERVAL env
	releasePromoteWatchInformerWarmupDelay = 15 * time.Second
)

// per-(app, release-id) singleton gate; self-clears on watcher exit.
var releasePromoteWatchInflight sync.Map

func tryAcquireWatchSlot(app, releaseID string) (bool, func()) {
	key := app + "/" + releaseID
	if _, loaded := releasePromoteWatchInflight.LoadOrStore(key, struct{}{}); loaded {
		return false, func() {}
	}
	return true, func() { releasePromoteWatchInflight.Delete(key) }
}

func releasePromoteWatchSlotHeldForTest(app, releaseID string) bool {
	_, ok := releasePromoteWatchInflight.Load(app + "/" + releaseID)
	return ok
}

// test-only panic hooks; nil in production.
var releasePromoteWatcherPanicHookForTest func(app, releaseID string)
var releasePromoteCleanupDeferPanicHookForTest func(app, releaseID string)

func (p *Provider) runReleasePromoteWatcher(
	ctx context.Context,
	app string,
	state *structs.ReleasePromoteWatchState,
	release func(),
) {
	// bare release runs last (LIFO) even if the cleanup defer panics.
	defer release()

	var resultStatus, resultError, bailReason string
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=release_watcher at=panic app=%s id=%s recover=%q stack=%q\n",
				app, state.ReleaseID, r, debug.Stack())
			resultStatus = "error"
			resultError = fmt.Sprintf("watcher-panic: %v", r)
		}
		// delete first: whoever wins the compare-and-delete owns the event
		claimed, err := p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID)
		if err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=annotation_delete app=%s err=%q\n", app, err)
		}
		if claimed || err != nil {
			p.emitReleasePromoteResult(app, state, resultStatus, resultError, bailReason)
		}
		if hook := releasePromoteCleanupDeferPanicHookForTest; hook != nil {
			hook(app, state.ReleaseID)
		}
		release()
	}()

	deadline := state.ExpiresAt.Add(releasePromoteWatchGracePeriod)
	// already past for a GC-adopted watcher, so a recovered rollback reports at once
	mirrorDeadline := state.StartedAt.Add(2 * releasePromoteWatchGracePeriod)
	detect := p.deployFastFailArmed(state)
	observedInFlight := false
	bailed := false

	tick := time.NewTicker(releasePromoteWatchPollInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			resultStatus = ""
			return
		case <-tick.C:
			if hook := releasePromoteWatcherPanicHookForTest; hook != nil {
				hook(app, state.ReleaseID)
			}
			ns, err := p.GetNamespaceFromInformer(p.AppNamespace(app))
			timedOut := time.Now().UTC().After(deadline)
			if err != nil {
				if kerr.IsNotFound(err) {
					resultStatus = ""
					return
				}
				if timedOut {
					resultStatus = "error"
					resultError = "watcher-timeout"
					return
				}
				continue
			}

			currentRelease := ns.Annotations["convox.com/app-release"]
			atomStatus := ns.Annotations["convox.com/app-status"]

			// updateNamespace patches all three annotations together, so once
			// app-release names this promote the whole triple is ours.
			if !observedInFlight {
				stale := currentRelease == "" || currentRelease == state.PriorRelease
				if currentRelease == state.AtomVersion || (stale && time.Now().UTC().After(mirrorDeadline)) {
					observedInFlight = true
				} else if stale {
					if timedOut {
						resultStatus = "error"
						resultError = "watcher-timeout"
						return
					}
					continue
				}
			}

			ownRollback := currentRelease == state.PriorRelease && isRollbackStatus(atomStatus)
			if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion && !ownRollback {
				resultStatus = "cancelled"
				resultError = "superseded-by-newer-promote"
				return
			}

			if status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus); terminal {
				resultStatus = status
				resultError = errMsg
				return
			}

			if timedOut {
				resultStatus = "error"
				resultError = "watcher-timeout"
				return
			}

			if !detect || bailed {
				continue
			}

			// a transient claim failure leaves bailed unset so the next tick retries
			if reason := p.deployFastFailReason(app, state); reason != "" {
				if p.bailReleasePromote(ctx, app, state, reason) {
					bailed = true
					bailReason = reason
				}
			}
		}
	}
}

func mapAppStatusToWatchResult(atomStatus string) (status, errMsg string, terminal bool) {
	switch atomStatus {
	case "Running", "Success":
		return "success", "", true
	case "Failure", "Reverted":
		return "error", "rollout-failed: " + atomStatus, true
	case "Cancelled":
		return "error", "cancelled", true
	case "Deadline":
		return "error", "deadline-exceeded", true
	case "Error", "Rollback":
		return "error", "rollback: " + atomStatus, true
	}
	return "", "", false
}

func (p *Provider) emitReleasePromoteResult(app string, state *structs.ReleasePromoteWatchState, status, errMsg, bailReason string) {
	if status == "" {
		return // ctx cancelled or namespace gone — silent
	}
	// Generic Atom-status messages only: a success routes into data["message"].
	if bailReason != "" && status == "error" && bailReasonOverridesError(errMsg) {
		errMsg = bailReason
	}
	var action string
	switch status {
	case "success":
		action = "app:promote:completed"
	case "error":
		action = "app:promote:errored"
	case "cancelled":
		action = "app:promote:cancelled"
	default:
		fmt.Printf("ns=release_watcher at=warn kind=unknown_status app=%s status=%q\n", app, status)
		action = "app:promote:errored"
	}
	data := map[string]string{"app": app, "id": state.ReleaseID, "actor": state.Actor}
	opts := structs.EventSendOptions{
		Data:   data,
		Status: options.String(status),
	}
	// opts.Error triggers EventSend's Status="error" rewrite; use data["message"] for non-error statuses.
	if errMsg != "" {
		if status == "error" {
			opts.Error = options.String(errMsg)
		} else {
			data["message"] = errMsg
		}
	}
	_ = p.EventSend(action, opts)
}

func (p *Provider) writeReleasePromoteWatchAnnotation(ctx context.Context, app string, state *structs.ReleasePromoteWatchState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return errors.WithStack(err)
	}
	patch, err := patchBytes(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				structs.ReleasePromoteWatchAnnotation: string(raw),
			},
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.MergePatchType, patch, am.PatchOptions{})
	return errors.WithStack(err)
}

// deleteReleasePromoteWatchAnnotationIfMatches only deletes if the annotation's
// release-id still matches ours. Uses JSON-Patch test op for atomic check-and-delete.
// The bool reports whether the compare-and-delete applied, which is what makes
// the caller the single owner of the outcome event.
func (p *Provider) deleteReleasePromoteWatchAnnotationIfMatches(ctx context.Context, app, expectedReleaseID string) (bool, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return false, nil
		}
		return false, errors.WithStack(err)
	}
	raw := ns.Annotations[structs.ReleasePromoteWatchAnnotation]
	if raw == "" {
		return false, nil
	}
	var state structs.ReleasePromoteWatchState
	if jerr := json.Unmarshal([]byte(raw), &state); jerr != nil {
		fmt.Printf("ns=release_watcher at=warn kind=cleanup_corrupt_json app=%s err=%q\n", app, jerr)
		return false, nil
	}
	if state.ReleaseID != expectedReleaseID {
		fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip app=%s mine=%s current=%s\n",
			app, expectedReleaseID, state.ReleaseID)
		return false, nil
	}
	rawJSON, jerr := json.Marshal(raw)
	if jerr != nil {
		return false, errors.WithStack(jerr)
	}
	patch := []byte(fmt.Sprintf(
		`[{"op":"test","path":"/metadata/annotations/convox.com~1release-promote-watch","value":%s},{"op":"remove","path":"/metadata/annotations/convox.com~1release-promote-watch"}]`,
		rawJSON,
	))
	_, perr := p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.JSONPatchType, patch, am.PatchOptions{})
	if perr != nil {
		if kerr.IsNotFound(perr) {
			return false, nil
		}
		if kerr.IsConflict(perr) || kerr.IsInvalid(perr) {
			// test op failed — annotation was overwritten concurrently.
			fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip_toctou app=%s mine=%s err=%q\n",
				app, expectedReleaseID, perr)
			return false, nil
		}
		return false, errors.WithStack(perr)
	}
	return true, nil
}

// Compare-and-set makes AppCancel run once cluster-wide. The read has to be
// live: a stale informer copy fails the test op and reads as a peer having won.
func (p *Provider) claimReleasePromoteBail(ctx context.Context, app string, state *structs.ReleasePromoteWatchState) (bool, error) {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		return false, errors.WithStack(err)
	}

	raw := ns.Annotations[structs.ReleasePromoteWatchAnnotation]
	if raw == "" {
		return false, nil
	}

	var stored structs.ReleasePromoteWatchState
	if jerr := json.Unmarshal([]byte(raw), &stored); jerr != nil {
		fmt.Printf("ns=release_watcher at=warn kind=bail_claim_corrupt_json app=%s err=%q\n", app, jerr)
		return false, nil
	}
	if stored.ReleaseID != state.ReleaseID || !stored.BailedAt.IsZero() {
		return false, nil
	}

	stored.BailedAt = time.Now().UTC()
	next, err := json.Marshal(&stored)
	if err != nil {
		return false, errors.WithStack(err)
	}
	oldJSON, err := json.Marshal(raw)
	if err != nil {
		return false, errors.WithStack(err)
	}
	newJSON, err := json.Marshal(string(next))
	if err != nil {
		return false, errors.WithStack(err)
	}

	patch := []byte(fmt.Sprintf(
		`[{"op":"test","path":"/metadata/annotations/convox.com~1release-promote-watch","value":%s},{"op":"replace","path":"/metadata/annotations/convox.com~1release-promote-watch","value":%s}]`,
		oldJSON, newJSON,
	))
	if _, err := p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.JSONPatchType, patch, am.PatchOptions{}); err != nil {
		if kerr.IsConflict(err) || kerr.IsInvalid(err) || kerr.IsNotFound(err) {
			return false, nil
		}
		return false, errors.WithStack(err)
	}

	return true, nil
}

func (p *Provider) deleteReleasePromoteWatchAnnotation(ctx context.Context, app string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:null}}}`, structs.ReleasePromoteWatchAnnotation))
	_, err := p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.MergePatchType, patch, am.PatchOptions{})
	if kerr.IsNotFound(err) {
		return nil
	}
	return errors.WithStack(err)
}

func (p *Provider) runReleasePromoteWatchGC(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(releasePromoteWatchInformerWarmupDelay):
	}
	p.scanReleasePromoteAnnotations(ctx)
	tick := time.NewTicker(releasePromoteWatchGCTickInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			p.scanReleasePromoteAnnotations(ctx)
		}
	}
}

func (p *Provider) scanReleasePromoteAnnotations(ctx context.Context) {
	selector := fmt.Sprintf("system=convox,rack=%s,type=app", p.Name)
	nsList, err := p.ListNamespacesFromInformer(selector)
	if err != nil {
		fmt.Printf("ns=release_watcher at=warn kind=gc_list_namespaces err=%q\n", err)
		return
	}
	now := time.Now().UTC()
	for i := range nsList.Items {
		ns := &nsList.Items[i]
		raw := ns.Annotations[structs.ReleasePromoteWatchAnnotation]
		if raw == "" {
			continue
		}
		app := ns.Labels["app"]
		if app == "" {
			app = ns.Labels["name"]
		}
		// The GC's background context carries no tid, so AppNamespace would
		// rebuild a tenant app's namespace without it.
		gp := p
		if tid := ns.Labels["tid"]; tid != "" {
			cp := *p
			cp.ctx = context.WithValue(p.ctx, structs.ConvoxTIDCtxKey, tid)
			gp = &cp
		}
		var state structs.ReleasePromoteWatchState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=corrupt_json app=%s err=%q\n", app, err)
			_ = gp.deleteReleasePromoteWatchAnnotation(gp.ctx, app)
			continue
		}
		if state.SchemaVersion != 1 {
			// unknown schemaVersion — skip, don't delete (may belong to a newer api-pod during rolling upgrade).
			fmt.Printf("ns=release_watcher at=warn kind=unknown_schema_version app=%s schemaVersion=%d release_id=%s actor=%s expires_at=%q recovery=\"kubectl annotate ns %s convox.com/release-promote-watch-\"\n",
				app, state.SchemaVersion, state.ReleaseID, state.Actor, state.ExpiresAt.Format(time.RFC3339), p.AppNamespace(app))
			continue
		}
		atomStatus := ns.Annotations["convox.com/app-status"]
		currentRelease := ns.Annotations["convox.com/app-release"]

		// the grace keeps this off the in-handler watcher's own expiry
		if state.ExpiresAt.Add(releasePromoteWatchGracePeriod).Before(now) {
			// consult app-status before assuming timeout — AtomController may have already written a terminal status.
			status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus)
			if !terminal {
				status, errMsg = "error", "watcher-timeout"
			}
			if claimed, err := gp.deleteReleasePromoteWatchAnnotationIfMatches(gp.ctx, app, state.ReleaseID); claimed || err != nil {
				gp.emitReleasePromoteResult(app, &state, status, errMsg, "")
			}
			continue
		}
		ownRollback := currentRelease == state.PriorRelease && isRollbackStatus(atomStatus)
		if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion && !ownRollback {
			if claimed, err := gp.deleteReleasePromoteWatchAnnotationIfMatches(gp.ctx, app, state.ReleaseID); claimed || err != nil {
				gp.emitReleasePromoteResult(app, &state, "cancelled", "superseded-by-newer-promote", "")
			}
			continue
		}
		acquired, release := tryAcquireWatchSlot(app, state.ReleaseID)
		if !acquired {
			continue
		}
		s := state
		go gp.runReleasePromoteWatcher(gp.ctx, app, &s, release)
	}
}

const (
	releaseWatcherGCIntervalLowerBound = 60 * time.Second
	releaseWatcherGCIntervalUpperBound = 1 * time.Hour
	releaseWatcherGCIntervalEnv        = "RELEASE_WATCHER_GC_INTERVAL"
)

// applyReleaseWatcherGCIntervalEnv reads the env var once at Initialize; clamps to [60s, 1h].
func applyReleaseWatcherGCIntervalEnv() bool {
	v := os.Getenv(releaseWatcherGCIntervalEnv)
	if v == "" {
		return false
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		fmt.Printf("ns=release_watcher at=warn kind=invalid_gc_interval value=%q err=%q falling_back=%s\n",
			v, err.Error(), releasePromoteWatchGCTickInterval)
		return true
	}
	switch {
	case d < releaseWatcherGCIntervalLowerBound:
		fmt.Printf("ns=release_watcher at=warn kind=gc_interval_below_min value=%q clamped_to=%s\n",
			v, releaseWatcherGCIntervalLowerBound)
		releasePromoteWatchGCTickInterval = releaseWatcherGCIntervalLowerBound
	case d > releaseWatcherGCIntervalUpperBound:
		fmt.Printf("ns=release_watcher at=warn kind=gc_interval_above_max value=%q clamped_to=%s\n",
			v, releaseWatcherGCIntervalUpperBound)
		releasePromoteWatchGCTickInterval = releaseWatcherGCIntervalUpperBound
	default:
		releasePromoteWatchGCTickInterval = d
	}
	return true
}
