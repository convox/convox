package k8s

import "sync"

// per-app mutex for budget mutations.
var (
	appBudgetLockMapMu sync.Mutex
	appBudgetLockMap   = map[string]*sync.Mutex{}
)

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

func removeAppLock(app string) {
	appBudgetLockMapMu.Lock()
	defer appBudgetLockMapMu.Unlock()
	delete(appBudgetLockMap, app)
}

func (p *Provider) RemoveAppLock(app string) {
	removeAppLock(app)
}
