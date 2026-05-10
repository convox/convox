package k8s

import "sync"

// appBudgetLockMap holds a per-app sync.Mutex used to serialize budget
// state mutations against accumulator-driven reconcileAutoShutdown.
//
// Without this lock the reset path (which fires :cancelled
// reason="reset-during-armed" then deletes the annotation) could race
// the accumulator's tick path that ALSO fires :cancelled
// reason="manual-detected" against the same in-flight armed state,
// producing TWO :cancelled events with different reasons in a
// sub-second window. Acquiring the per-app lock at both entry points
// closes that race.
//
// Cap-raise-clears-breaker extended the lock surface to AppBudgetSet:
// when a cap-raise to a value above current spend clears
// CircuitBreakerTripped + AlertFiredAt*, those are the same fields
// the accumulator's reconcileAutoShutdown reads-then-decides-then-
// writes. Without the lock a concurrent reconcileAutoShutdown could
// observe pre-clear state, decide to fire :armed, and persist a stale
// decision after our clear lands.
//
// Scoped to the auto-shutdown coordination surface: AppBudgetReset,
// AppBudgetSet, AppBudgetClear, budget_auto_shutdown reconcileAutoShutdown,
// AppBudgetDismissRecoveryWithResult. Read-only paths (AppBudgetGet)
// remain unsynchronized.
//
// AppBudgetDismissRecoveryWithResult joined the surface to close a
// concurrent-click race: two dismiss clicks both observed
// existing=nil and both wrote the annotation, producing duplicate
// :dismissed events with idempotent=false. The lock collapses the
// read-decide-write into a critical section so the second click sees
// existing != nil and returns Status="already-dismissed".
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
// Without this hook, every app that ever had its budget reconciled
// would leave a *sync.Mutex in appBudgetLockMap forever, even after
// the app is deleted. On long-lived racks with high app churn the map
// grows unbounded. AppDelete calls this after namespace deletion
// succeeds so a doomed reconciler still has a valid lock for the
// duration of its tick.
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
