package k8s

import (
	"context"
	"encoding/json"
	"fmt"
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

// release-watcher constants — see phase-i-post-rc4/phase-b/items/item-18-rollout-watcher.md
// for the rationale on each value.
//
// These are vars (not const) so unit tests can override timing without
// burning real wall-clock seconds. Production code MUST NOT mutate these
// at runtime; the test-only override hooks live in export_test.go.
var (
	// releasePromoteWatchPollInterval is how often the watcher reads the
	// namespace annotation `convox.com/app-status`. 3s balances UX latency
	// (Console events page repaints within ~3s of rollout completion)
	// against load (cached annotation read; effectively free).
	releasePromoteWatchPollInterval = 3 * time.Second

	// releasePromoteWatchGracePeriod extends the watcher's deadline past
	// state.ExpiresAt. Lets the AtomController's own `Deadline` transition
	// land before we declare watcher-timeout — preferring the customer-
	// truth (Atom Status) over our derived emit.
	releasePromoteWatchGracePeriod = 30 * time.Second

	// releasePromoteWatchGCTickInterval is how often the cold-start GC
	// re-sweeps every app namespace. 5min is cheap enough not to matter
	// and frequent enough that a multi-pod restart in a 30-min window
	// self-heals.
	releasePromoteWatchGCTickInterval = 5 * time.Minute

	// releasePromoteWatchInformerWarmupDelay is the wait before the FIRST
	// GC sweep on api-pod startup. Lets the namespace informer cache
	// populate so the sweep finds the apps that have in-flight watches.
	releasePromoteWatchInformerWarmupDelay = 15 * time.Second
)

// releasePromoteWatchInflight is the per-(app, release-id) singleton-watch
// gate. LoadOrStore returns loaded=true when a watcher is already running
// for the exact same pair, in which case the second promote skips watcher
// launch. The map self-clears on watcher exit (no AppDelete hook needed —
// keys are per-promote, not per-app-forever).
var releasePromoteWatchInflight sync.Map

// tryAcquireWatchSlot atomically checks-and-sets a per-(app, release-id)
// slot. Returns (acquired=true, release-fn) on first acquisition,
// (acquired=false, no-op-fn) when an existing watcher already holds the
// slot. Callers MUST `defer release()` immediately after acquired=true.
func tryAcquireWatchSlot(app, releaseID string) (bool, func()) {
	key := app + "/" + releaseID
	if _, loaded := releasePromoteWatchInflight.LoadOrStore(key, struct{}{}); loaded {
		return false, func() {}
	}
	return true, func() { releasePromoteWatchInflight.Delete(key) }
}

// releasePromoteWatchSlotHeldForTest reports whether a watch slot is
// currently held for the given (app, release-id) pair. Test-only.
func releasePromoteWatchSlotHeldForTest(app, releaseID string) bool {
	_, ok := releasePromoteWatchInflight.Load(app + "/" + releaseID)
	return ok
}

// runReleasePromoteWatcher polls the namespace annotation
// `convox.com/app-status` until it reaches a terminal AtomStatus value or
// the deadline (state.ExpiresAt + grace) fires. Emits a single terminal
// event using the canonical app:<resource>:<verb> convention (one of
// app:promote:completed, app:promote:errored, app:promote:cancelled);
// deletes the watch annotation; releases the per-promote slot. Outer-
// defer recovers panics + always releases the slot — same contract as
// build.go:617-632. State is taken by pointer to avoid copying the
// 100-byte struct on each launch.
func (p *Provider) runReleasePromoteWatcher(
	ctx context.Context,
	app string,
	state *structs.ReleasePromoteWatchState,
	release func(),
) {
	var resultStatus, resultError string
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=release_watcher at=panic app=%s id=%s recover=%q stack=%q\n",
				app, state.ReleaseID, r, debug.Stack())
			resultStatus = "error"
			resultError = fmt.Sprintf("watcher-panic: %v", r)
		}
		// Idempotent emit — guarded against double-emit by the per-promote slot.
		p.emitReleasePromoteResult(app, state, resultStatus, resultError)
		// Best-effort annotation cleanup — namespace-deletion races handled internally.
		if err := p.deleteReleasePromoteWatchAnnotation(ctx, app); err != nil {
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
			// api-pod shutdown — leave annotation; cold-start GC at next
			// api-pod startup re-launches the watcher.
			resultStatus = ""
			return
		case <-tick.C:
			ns, err := p.GetNamespaceFromInformer(p.AppNamespace(app))
			if err != nil {
				if kerr.IsNotFound(err) {
					// App deleted mid-watch — silent exit. Namespace
					// teardown already removed the annotation. Emitting
					// an event for a deleted app pollutes the audit log.
					resultStatus = ""
					return
				}
				continue // transient API error — retry next tick
			}
			// Supersession check uses `convox.com/app-release` (the
			// release-id mirror written by AtomController.updateNamespace).
			// state.AtomVersion holds the release-id captured at promote
			// time (sourced from p.Atom.Status() which returns the release
			// name from ReleaseCache). When a newer promote lands, the
			// AtomController updates the release annotation; the old
			// watcher detects the mismatch and exits as cancelled.
			currentRelease := ns.Annotations["convox.com/app-release"]
			if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion {
				// A newer promote has taken over. The new promote's own
				// watcher will emit its own completed/errored event. Use
				// status="cancelled" (NOT "error") so the audit log
				// distinguishes normal supersession (lifecycle event)
				// from actual rollout failure.
				resultStatus = "cancelled"
				resultError = "superseded-by-newer-promote"
				return
			}
			atomStatus := ns.Annotations["convox.com/app-status"]
			switch atomStatus {
			case "Running", "Success":
				resultStatus = "success"
				return
			case "Failure", "Reverted":
				resultStatus = "error"
				resultError = "rollout-failed: " + atomStatus
				return
			case "Cancelled":
				resultStatus = "error"
				resultError = "cancelled"
				return
			case "Deadline":
				resultStatus = "error"
				resultError = "deadline-exceeded"
				return
			case "Error", "Rollback":
				resultStatus = "error"
				resultError = "rollback: " + atomStatus
				return
			}
			// Pending / Updating — keep polling.
			if time.Now().UTC().After(deadline) {
				resultStatus = "error"
				resultError = "watcher-timeout"
				return
			}
		}
	}
}

// emitReleasePromoteResult dispatches the watcher's terminal event using
// the canonical app:<resource>:<verb> convention:
//
//	status="success"   -> action="app:promote:completed"
//	status="error"     -> action="app:promote:errored"
//	status="cancelled" -> action="app:promote:cancelled"
//
// The Status field is preserved so Console3 iconForEvent (already keyed on
// e.status) renders the right icon without needing event-type-specific
// branches. The original `release:promote Status="start"` action emitted
// at promote submission time is UNCHANGED — back-compat for webhook
// consumers that filter on action="release:promote".
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
		// Unknown status — log defensively and emit on the canonical
		// errored action so the customer sees SOMETHING in the timeline.
		fmt.Printf("ns=release_watcher at=warn kind=unknown_status app=%s status=%q\n", app, status)
		action = "app:promote:errored"
	}
	data := map[string]string{"app": app, "id": state.ReleaseID, "actor": state.Actor}
	opts := structs.EventSendOptions{
		Data:   data,
		Status: options.String(status),
	}
	// Use opts.Error only on the canonical "error" path so EventSend's
	// auto-rewrite to Status="error" matches our chosen status. For
	// "success" and "cancelled" terminal states the supplemental detail
	// goes in data.message directly so EventSend does NOT clobber the
	// status field. (event.go:101-104 sets Status="error" whenever
	// opts.Error is non-nil, which would otherwise reclassify a
	// supersession event as a failure.)
	if errMsg != "" {
		if status == "error" {
			opts.Error = options.String(errMsg)
		} else {
			data["message"] = errMsg
		}
	}
	_ = p.EventSend(action, opts)
}

// writeReleasePromoteWatchAnnotation persists the watch state JSON onto the
// app namespace. Called from ReleasePromote BEFORE the watcher goroutine
// launches, so a fast-fail between annotation-write and goroutine-launch
// is recoverable via cold-start GC at the next api-pod startup.
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

// deleteReleasePromoteWatchAnnotation removes the watch annotation from the
// app namespace via a strategic-merge null patch. Idempotent across
// concurrent calls (multiple watchers, GC scan + steady-state watcher).
func (p *Provider) deleteReleasePromoteWatchAnnotation(ctx context.Context, app string) error {
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:null}}}`, structs.ReleasePromoteWatchAnnotation))
	_, err := p.Cluster.CoreV1().Namespaces().Patch(ctx, p.AppNamespace(app), types.MergePatchType, patch, am.PatchOptions{})
	if kerr.IsNotFound(err) {
		return nil
	}
	return errors.WithStack(err)
}

// runReleasePromoteWatchGC sweeps every app namespace at api-pod startup
// and on a 5-min tick to re-launch watchers from a previous api-pod and
// to clean up stale annotations (timed-out, superseded, corrupt JSON,
// wrong schemaVersion).
func (p *Provider) runReleasePromoteWatchGC(ctx context.Context) {
	// Initial wait so the namespace informer can populate before the first
	// scan. Cold-start GC reads via informer cache; an empty cache produces
	// a no-op tick instead of recovery.
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

// scanReleasePromoteAnnotations is one GC pass: walk every app namespace
// labelled by this rack, examine the watch annotation, and either re-launch
// a watcher (in-deadline, schemaVersion=1 match) or emit a terminal event
// (timed-out, superseded) or skip (unknown schemaVersion) or delete (corrupt
// JSON).
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
			// Corrupt JSON — GC immediately. No payload to attribute an
			// event to, and the next promote re-writes a fresh annotation.
			fmt.Printf("ns=release_watcher at=warn kind=corrupt_json app=%s err=%q\n", app, err)
			_ = p.deleteReleasePromoteWatchAnnotation(ctx, app)
			continue
		}
		if state.SchemaVersion != 1 {
			// Unknown / future schemaVersion — LOG-AND-SKIP, do NOT delete.
			// During a rolling upgrade (rc6 -> rc7 with SchemaVersion=2),
			// an old api-pod (rc6) reading a future-version annotation
			// written by a new api-pod (rc7) MUST leave the annotation
			// alone. Deleting it would lose persistent watcher state
			// mid-flight during the 5-15min rolling-upgrade window —
			// future api-pods that own this schemaVersion handle it
			// via their own scan pass.
			fmt.Printf("ns=release_watcher at=warn kind=unknown_schema_version app=%s schemaVersion=%d\n",
				app, state.SchemaVersion)
			continue
		}
		if state.ExpiresAt.Before(now) {
			p.emitReleasePromoteResult(app, &state, "error", "watcher-timeout")
			_ = p.deleteReleasePromoteWatchAnnotation(ctx, app)
			continue
		}
		// Supersession via release annotation mirror — see runReleasePromoteWatcher
		// for the rationale on comparing against `convox.com/app-release`.
		currentRelease := ns.Annotations["convox.com/app-release"]
		if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion {
			p.emitReleasePromoteResult(app, &state, "cancelled", "superseded-by-newer-promote")
			_ = p.deleteReleasePromoteWatchAnnotation(ctx, app)
			continue
		}
		// Re-launch watcher. LoadOrStore prevents double-launch when the
		// steady-state watcher and GC fire in the same window. Copy state
		// onto the heap (`s := state`) so each launched goroutine owns its
		// own pointer; otherwise concurrent loop iterations would race on
		// the loop-local `state` variable.
		acquired, release := tryAcquireWatchSlot(app, state.ReleaseID)
		if !acquired {
			continue
		}
		s := state
		go p.runReleasePromoteWatcher(ctx, app, &s, release)
	}
}
