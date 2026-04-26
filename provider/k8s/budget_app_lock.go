package k8s

import "sync"

// appBudgetLockMap holds a per-app sync.Mutex used to serialize
// AppBudgetReset against accumulator-driven reconcileAutoShutdown.
//
// F-19 fix (catalog D-7 — reset-during-tick race): without this lock
// reset (which fires :cancelled reason="reset-during-armed" then deletes
// the annotation) could race the accumulator's tick path that ALSO fires
// :cancelled reason="manual-detected" against the same in-flight armed
// state, producing TWO :cancelled events with different reasons in a
// sub-second window. Acquiring the per-app lock at both entry points
// closes that race.
//
// Deliberately scoped to the auto-shutdown coordination surface
// (budget_accumulator AppBudgetReset + budget_auto_shutdown
// reconcileAutoShutdown). Other budget paths (AppBudgetGet,
// AppBudgetSet) remain unsynchronized — they don't fire :cancelled.
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
