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

	var resultStatus, resultError string
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=release_watcher at=panic app=%s id=%s recover=%q stack=%q\n",
				app, state.ReleaseID, r, debug.Stack())
			resultStatus = "error"
			resultError = fmt.Sprintf("watcher-panic: %v", r)
		}
		p.emitReleasePromoteResult(app, state, resultStatus, resultError)
		if hook := releasePromoteCleanupDeferPanicHookForTest; hook != nil {
			hook(app, state.ReleaseID)
		}
		if err := p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID); err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=annotation_delete app=%s err=%q\n", app, err)
		}
		release()
	}()

	deadline := state.ExpiresAt.Add(releasePromoteWatchGracePeriod)
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
			if err != nil {
				if kerr.IsNotFound(err) {
					resultStatus = ""
					return
				}
				continue
			}
			currentRelease := ns.Annotations["convox.com/app-release"]
			if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion {
				resultStatus = "cancelled"
				resultError = "superseded-by-newer-promote"
				return
			}
			atomStatus := ns.Annotations["convox.com/app-status"]
			if status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus); terminal {
				resultStatus = status
				resultError = errMsg
				return
			}
			if time.Now().UTC().After(deadline) {
				resultStatus = "error"
				resultError = "watcher-timeout"
				return
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

func (p *Provider) emitReleasePromoteResult(app string, state *structs.ReleasePromoteWatchState, status, errMsg string) {
	if status == "" {
		return // ctx cancelled or namespace gone — silent
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
func (p *Provider) deleteReleasePromoteWatchAnnotationIfMatches(ctx context.Context, app, expectedReleaseID string) error {
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, p.AppNamespace(app), am.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	raw := ns.Annotations[structs.ReleasePromoteWatchAnnotation]
	if raw == "" {
		return nil
	}
	var state structs.ReleasePromoteWatchState
	if jerr := json.Unmarshal([]byte(raw), &state); jerr != nil {
		fmt.Printf("ns=release_watcher at=warn kind=cleanup_corrupt_json app=%s err=%q\n", app, jerr)
		return nil
	}
	if state.ReleaseID != expectedReleaseID {
		fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip app=%s mine=%s current=%s\n",
			app, expectedReleaseID, state.ReleaseID)
		return nil
	}
	rawJSON, jerr := json.Marshal(raw)
	if jerr != nil {
		return errors.WithStack(jerr)
	}
	patch := []byte(fmt.Sprintf(
		`[{"op":"test","path":"/metadata/annotations/convox.com~1release-promote-watch","value":%s},{"op":"remove","path":"/metadata/annotations/convox.com~1release-promote-watch"}]`,
		rawJSON,
	))
	_, perr := p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.JSONPatchType, patch, am.PatchOptions{})
	if perr != nil {
		if kerr.IsNotFound(perr) {
			return nil
		}
		if kerr.IsConflict(perr) || kerr.IsInvalid(perr) {
			// test op failed — annotation was overwritten concurrently.
			fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip_toctou app=%s mine=%s err=%q\n",
				app, expectedReleaseID, perr)
			return nil
		}
		return errors.WithStack(perr)
	}
	return nil
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
		var state structs.ReleasePromoteWatchState
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			fmt.Printf("ns=release_watcher at=warn kind=corrupt_json app=%s err=%q\n", app, err)
			_ = p.deleteReleasePromoteWatchAnnotation(ctx, app)
			continue
		}
		if state.SchemaVersion != 1 {
			// unknown schemaVersion — skip, don't delete (may belong to a newer api-pod during rolling upgrade).
			fmt.Printf("ns=release_watcher at=warn kind=unknown_schema_version app=%s schemaVersion=%d release_id=%s actor=%s expires_at=%q recovery=\"kubectl annotate ns %s convox.com/release-promote-watch-\"\n",
				app, state.SchemaVersion, state.ReleaseID, state.Actor, state.ExpiresAt.Format(time.RFC3339), p.AppNamespace(app))
			continue
		}
		if state.ExpiresAt.Before(now) {
			// consult app-status before assuming timeout — AtomController may have already written a terminal status.
			atomStatus := ns.Annotations["convox.com/app-status"]
			if status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus); terminal {
				p.emitReleasePromoteResult(app, &state, status, errMsg)
			} else {
				p.emitReleasePromoteResult(app, &state, "error", "watcher-timeout")
			}
			_ = p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID)
			continue
		}
		currentRelease := ns.Annotations["convox.com/app-release"]
		if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion {
			p.emitReleasePromoteResult(app, &state, "cancelled", "superseded-by-newer-promote")
			_ = p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID)
			continue
		}
		acquired, release := tryAcquireWatchSlot(app, state.ReleaseID)
		if !acquired {
			continue
		}
		s := state
		go p.runReleasePromoteWatcher(ctx, app, &s, release)
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
