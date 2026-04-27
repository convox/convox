package k8s

import "sync"

// appBudgetLockMap holds a per-app sync.Mutex used to serialize budget
// state mutations against accumulator-driven reconcileAutoShutdown.
//
// F-19 fix (catalog D-7 — reset-during-tick race): without this lock
// reset (which fires :cancelled reason="reset-during-armed" then deletes
// the annotation) could race the accumulator's tick path that ALSO fires
// :cancelled reason="manual-detected" against the same in-flight armed
// state, producing TWO :cancelled events with different reasons in a
// sub-second window. Acquiring the per-app lock at both entry points
// closes that race.
//
// Decision 3 (cap-raise clears breaker) extended the lock surface to
// AppBudgetSet: when a cap-raise to a value above current spend clears
// CircuitBreakerTripped + AlertFiredAt*, those are the same fields the
// accumulator's reconcileAutoShutdown reads-then-decides-then-writes.
// Without the lock a concurrent reconcileAutoShutdown could observe
// pre-clear state, decide to fire :armed, and persist a stale decision
// after our clear lands.
//
// Scoped to the auto-shutdown coordination surface (AppBudgetReset,
// AppBudgetSet, budget_auto_shutdown reconcileAutoShutdown). Read-only
// paths (AppBudgetGet) and AppBudgetClear remain unsynchronized —
// AppBudgetClear's full-annotation delete races neither :cancelled nor
// breaker-state mutations.
var (
	appBudgetLockMapMu sync.Mutex
	appBudgetLockMap   = map[string]*sync.Mutex{}
)

// appBudgetLock returns the per-app advisory mutex, creating it lazily
// on first use. Callers must Lock/Unlock — the helper does not return
// a held lock.
func appBudgetLock(app string) *sync.Mutex {
	appBudgetLockMapMu.Lock()
	defer appBudgetLockMapMu.Unlock()
	mu, ok := appBudgetLockMap[app]
	if !ok {
		mu = &sync.Mutex{}
		appBudgetLockMap[app] = mu
	}
	return mu
}

// removeAppLock deletes the per-app mutex entry from appBudgetLockMap.
//
// MF-8 fix (catalog A-1 — lockMap memory leak): without this hook, every
// app that ever had its budget reconciled leaves a *sync.Mutex in
// appBudgetLockMap forever, even after the app is deleted. On long-lived
// racks with high app churn the map grows unbounded. AppDelete calls this
// after namespace deletion succeeds so a doomed reconciler still has a
// valid lock for the duration of its tick.
func removeAppLock(app string) {
	appBudgetLockMapMu.Lock()
	defer appBudgetLockMapMu.Unlock()
	delete(appBudgetLockMap, app)
}

// RemoveAppLock is the Provider-method wrapper around removeAppLock.
// Exposed so AppDelete (and tests) can drop per-app lock entries without
// reaching into the package-level map directly.
func (p *Provider) RemoveAppLock(app string) {
	removeAppLock(app)
}
