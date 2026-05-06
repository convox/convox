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

// release-watcher constants — timing rationale: poll interval balances UX
// latency (Console events page repaints within ~3s of rollout completion)
// against load (cached annotation read, effectively free).
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
	// land before we declare watcher-timeout — preferring the user-
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

// releasePromoteWatcherPanicHookForTest is a test-only injection point for
// triggering a panic from within the watcher's polling loop. Tests set it
// via SetReleasePromoteWatcherPanicHookForTest to validate the outer
// recover() and the belt-and-suspenders bare-defer release() that runs
// last in LIFO order. Production callers MUST NOT touch this hook —
// nil is the production contract.
var releasePromoteWatcherPanicHookForTest func(app, releaseID string)

// releasePromoteCleanupDeferPanicHookForTest is a test-only injection
// point that fires from inside the cleanup defer (after the inner
// recover() but before the bare-defer release() runs). Tests use it to
// validate that a panic INSIDE the cleanup defer still leaves the slot
// released, since the LIFO bare-defer at the top of the function fires
// even when the cleanup defer itself panics. Production callers MUST
// NOT touch this hook — nil is the production contract.
var releasePromoteCleanupDeferPanicHookForTest func(app, releaseID string)

// runReleasePromoteWatcher polls the namespace annotation
// `convox.com/app-status` until it reaches a terminal AtomStatus value or
// the deadline (state.ExpiresAt + grace) fires. Emits a single terminal
// event using the canonical app:<resource>:<verb> convention (one of
// app:promote:completed, app:promote:errored, app:promote:cancelled);
// deletes the watch annotation; releases the per-promote slot. Outer-
// defer recovers panics + always releases the slot — same contract as
// build.go:617-632. State is taken by pointer to avoid copying the
// 100-byte struct on each launch.
//
// Defer execution order (Go unwinds LIFO — last-registered defer runs
// FIRST):
//  1. INNER (cleanup): runs FIRST on unwind. Registered SECOND in the
//     function body. Recovers panics from the watcher loop body, emits
//     the terminal event, deletes the watch annotation (supersession-
//     aware), and calls release() for the normal-path teardown.
//  2. OUTER (bare release): runs LAST on unwind. Registered FIRST in the
//     function body. Belt-and-suspenders: if the inner cleanup defer
//     itself panics (e.g. inside emitReleasePromoteResult or the
//     annotation delete RPC), the slot is still released. Calling
//     release twice is harmless — sync.Map.Delete is idempotent.
func (p *Provider) runReleasePromoteWatcher(
	ctx context.Context,
	app string,
	state *structs.ReleasePromoteWatchState,
	release func(),
) {
	// Outermost LIFO defer — runs LAST, after the cleanup defer below.
	// Guarantees the slot is released even if cleanup itself panics.
	// release() must be idempotent; sync.Map.Delete satisfies that.
	defer release()

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
		// Test-only hook to validate that a panic from within the cleanup
		// defer (post-recover) still surfaces release() through the bare
		// defer above. Fires only when the test explicitly installs it.
		if hook := releasePromoteCleanupDeferPanicHookForTest; hook != nil {
			hook(app, state.ReleaseID)
		}
		// Supersession-aware annotation cleanup.
		// concurrent supersession + steady-state cleanup race could
		// previously delete a NEWER promote's annotation. Read the
		// current annotation; only delete it if its release-id still
		// matches our own (i.e. nothing has overwritten our payload).
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
			// api-pod shutdown — leave annotation; cold-start GC at next
			// api-pod startup re-launches the watcher.
			resultStatus = ""
			return
		case <-tick.C:
			// Test-only injectable panic hook — exercises the outer-
			// defer recover() + the LIFO bare-defer release() in unit
			// tests. nil in production; the hook check is a single
			// nil-load and free.
			if hook := releasePromoteWatcherPanicHookForTest; hook != nil {
				hook(app, state.ReleaseID)
			}
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
			if status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus); terminal {
				resultStatus = status
				resultError = errMsg
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

// mapAppStatusToWatchResult translates the AtomController-written
// `convox.com/app-status` annotation into the watcher's emit tuple:
// (resultStatus, resultError, terminal). When terminal=false the caller
// keeps polling (atomStatus is Pending / Updating / empty). When
// terminal=true the resultStatus + resultError pair is what the
// emitReleasePromoteResult dispatcher should consume — same shape the
// in-loop steady-state switch produced before the helper extraction.
//
// The GC scanner past-deadline branch previously emitted `app:promote:errored` unconditionally
// without consulting `convox.com/app-status`. That mis-attributed real
// rollout outcomes (e.g. AtomController had already written
// app-status=Success but the watch annotation hadn't been cleaned up
// yet, so the past-deadline cleanup falsely reported the promote as
// errored). The helper now lets the GC scanner read app-status first
// and only fall back to watcher-timeout when no terminal status has
// been recorded.
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
		// errored action so the user sees SOMETHING in the timeline.
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

// deleteReleasePromoteWatchAnnotationIfMatches is the supersession-aware
// cleanup variant. It first reads the namespace annotation; if the stored
// `state.ReleaseID` differs from `expectedReleaseID`, the delete is
// skipped (a newer promote has overwritten the annotation since this
// watcher launched, and the new watcher needs the payload preserved).
// On read errors, namespace-not-found, or empty annotation, the call
// becomes a no-op so the existing best-effort semantics are preserved.
//
// Closes the supersession-cleanup race where a fast
// promote sequence (Watcher A reaches terminal state, then Watcher B
// starts and writes its own annotation, then A's cleanup defer fires)
// would previously have A clobber B's payload.
//
// The original implementation read the annotation, validated the release-id match,
// then issued a separate unconditional MergePatch — leaving a narrow
// TOCTOU window where a concurrent writer could overwrite the
// annotation between the Get and the Patch. Now uses a JSON-Patch with
// a `test` op that the apiserver evaluates atomically: if the
// annotation no longer holds our payload by the time the Patch lands,
// the apiserver rejects with Invalid (test op failed) or Conflict and
// we treat that as a supersession-aware skip rather than an error.
// This eliminates the residual TOCTOU window without changing the
// observable cleanup contract.
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
		// Annotation already gone (GC scan, prior cleanup, or never
		// written). Nothing to delete — no-op.
		return nil
	}
	var state structs.ReleasePromoteWatchState
	if jerr := json.Unmarshal([]byte(raw), &state); jerr != nil {
		// Corrupt JSON — defer to GC scan to delete via its own
		// invalid-payload branch. Safer than letting a steady-state
		// watcher delete an annotation it can't fully attribute.
		fmt.Printf("ns=release_watcher at=warn kind=cleanup_corrupt_json app=%s err=%q\n", app, jerr)
		return nil
	}
	if state.ReleaseID != expectedReleaseID {
		// A newer promote has already overwritten the annotation.
		// Skip the delete so the new watcher's payload survives —
		// the new watcher's own cleanup will handle deletion when
		// it reaches a terminal state.
		fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip app=%s mine=%s current=%s\n",
			app, expectedReleaseID, state.ReleaseID)
		return nil
	}
	// JSON-Patch with `test` op: apiserver atomically rejects the patch
	// if the annotation has been overwritten between our Get above and
	// this Patch landing on the apiserver. The annotation key
	// `convox.com/release-promote-watch` contains a `/` which JSON
	// Pointer (RFC 6901) escapes as `~1`, hence the path
	// `/metadata/annotations/convox.com~1release-promote-watch`.
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
			// Namespace gone between Get and Patch — best-effort no-op.
			return nil
		}
		if kerr.IsConflict(perr) || kerr.IsInvalid(perr) {
			// Test op failed — annotation was overwritten between our
			// Get and this Patch. Same supersession-skip semantics as
			// the in-process release-id mismatch branch above.
			fmt.Printf("ns=release_watcher at=info kind=cleanup_supersession_skip_toctou app=%s mine=%s err=%q\n",
				app, expectedReleaseID, perr)
			return nil
		}
		return errors.WithStack(perr)
	}
	return nil
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
			//
			// A12 m-3 fix: emit a structured WARN line with all fields an
			// operator needs to confirm a stuck annotation post-upgrade,
			// including the manual-recovery hint when an upgrade window
			// has closed without the future api-pod claiming the state.
			// `kubectl annotate ns <app-ns> convox.com/release-promote-watch-`
			// clears it manually if needed. The 5-minute GC tick rate bounds
			// the log volume — one WARN line per stuck annotation per tick
			// (~288 lines/day for a persistently-stuck future-schemaVersion
			// annotation).
			fmt.Printf("ns=release_watcher at=warn kind=unknown_schema_version app=%s schemaVersion=%d release_id=%s actor=%s expires_at=%q recovery=\"kubectl annotate ns %s convox.com/release-promote-watch-\"\n",
				app, state.SchemaVersion, state.ReleaseID, state.Actor, state.ExpiresAt.Format(time.RFC3339), p.AppNamespace(app))
			continue
		}
		if state.ExpiresAt.Before(now) {
			// Past-deadline no longer assumes errored. Consult `convox.com/app-status`
			// first — if AtomController has already written a terminal
			// status (Success / Failure / Cancelled / Deadline / Error /
			// Rollback / Reverted), surface that as the watch event so
			// the audit log reflects the real rollout outcome. Only
			// fall back to watcher-timeout when app-status is non-
			// terminal (Pending / Updating / empty) — i.e. the watcher
			// genuinely ran out of clock without ever observing a
			// terminal AtomController write.
			atomStatus := ns.Annotations["convox.com/app-status"]
			if status, errMsg, terminal := mapAppStatusToWatchResult(atomStatus); terminal {
				p.emitReleasePromoteResult(app, &state, status, errMsg)
			} else {
				p.emitReleasePromoteResult(app, &state, "error", "watcher-timeout")
			}
			_ = p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID)
			continue
		}
		// Supersession via release annotation mirror — see runReleasePromoteWatcher
		// for the rationale on comparing against `convox.com/app-release`.
		currentRelease := ns.Annotations["convox.com/app-release"]
		if currentRelease != "" && state.AtomVersion != "" && currentRelease != state.AtomVersion {
			p.emitReleasePromoteResult(app, &state, "cancelled", "superseded-by-newer-promote")
			_ = p.deleteReleasePromoteWatchAnnotationIfMatches(ctx, app, state.ReleaseID)
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
