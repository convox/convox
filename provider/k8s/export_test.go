package k8s

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/convox/convox/pkg/billing"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	v1 "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// SkopeoExecForTest exposes the skopeo binary launcher for unit tests in
// build-import-image flows (added in PR #1018). Tests substitute a stub
// command-runner; production code paths must NOT touch this hook.
var SkopeoExecForTest = &skopeoExec

// MaxConcurrentImportsForTest exposes the import-limiter cap for tests that
// verify the 409 backpressure path. Tests must call ResetImportSlotsForTest
// after mutating this value to rebuild the slot channel at the new capacity;
// otherwise the channel size baked in by the first production use wins.
// Test-only; production callers reference maxConcurrentImports directly.
var MaxConcurrentImportsForTest = &maxConcurrentImports

// ResetImportSlotsForTest re-creates the limiter channel sized to the current
// maxConcurrentImports and clears the lazy-init guard. Tests must call this
// AFTER mutating MaxConcurrentImportsForTest and again on cleanup to leave a
// fresh channel for the next test in the suite. Holds importSlotsMu so a
// concurrent acquire/release in a still-running goroutine sees a stable
// snapshot. Test-only.
func ResetImportSlotsForTest() {
	importSlotsMu.Lock()
	defer importSlotsMu.Unlock()
	importSlots = make(chan struct{}, maxConcurrentImports)
	importSlotsOnce = sync.Once{}
	importSlotsOnce.Do(func() {})
}

// AccumulateBudgetAppForTest is a test-only hook exposing the per-app
// accumulator tick without the leader-election + polling scaffolding. A
// variadic `now` lets tests inject a deterministic clock; omit it to use
// time.Now().UTC(). Internally threads context.Background() so the existing
// 30+ tests that do not exercise cancellation continue to compile against
// the unexported accumulator path's ctx-aware signature. Tests that want to
// drive cancellation must use AccumulateBudgetAppCtxForTest instead.
// Production callers must use the leader-election entry point
// (runBudgetAccumulator) instead.
func AccumulateBudgetAppForTest(p *Provider, app string, now ...time.Time) error {
	t := time.Now().UTC()
	if len(now) > 0 {
		t = now[0]
	}
	return p.accumulateBudgetApp(context.Background(), app, t)
}

// AccumulateBudgetAppCtxForTest is the ctx-aware test hook: it mirrors
// AccumulateBudgetAppForTest but lets a caller supply a cancellable ctx so
// shutdown-cancellation behavior on the namespace Get/Update RPCs can be
// asserted. Pass context.Background() if the test does not exercise
// cancellation. Test-only; production callers must use the leader-election
// entry point.
func AccumulateBudgetAppCtxForTest(p *Provider, ctx context.Context, app string, now time.Time) error {
	return p.accumulateBudgetApp(ctx, app, now)
}

// RunBudgetAccumulatorForTest exposes the unexported accumulator entry
// point so deterministic shutdown-lifecycle tests can drive a controlled
// ctx without standing up the leader-election scaffolding. Production
// callers MUST NOT use this -- the only correct entry point in production
// is RunUsingLeaderElection -> p.runBudgetAccumulator. Test-only.
func RunBudgetAccumulatorForTest(p *Provider, ctx context.Context) {
	p.runBudgetAccumulator(ctx)
}

// AccumulateBudgetTickForTest exposes the per-tick walk so the per-app
// cancellation test can assert the ctx.Err() check at the top of the
// for-loop short-circuits remaining apps mid-walk. Test-only.
func AccumulateBudgetTickForTest(p *Provider, ctx context.Context) error {
	return p.accumulateBudgetTick(ctx)
}

// BudgetTickShutdownGraceForTest exposes the shutdown-grace constant for
// lifecycle tests that need to assert wait timing relative to the
// production grace window. Test-only.
func BudgetTickShutdownGraceForTest() time.Duration {
	return budgetTickShutdownGrace
}

// DominantResourceFractionForTest exposes the dominant-resource attribution
// formula for unit tests. Kept a thin wrapper — the production function
// signature must stay internal because it couples to v1.Pod/v1.Node
// pointers we do not want leaking into the package's public surface.
// Test-only; production code calls dominantResourceFraction directly.
func DominantResourceFractionForTest(pod *v1.Pod, node *v1.Node, price billing.InstancePrice) float64 {
	return dominantResourceFraction(pod, node, price)
}

// NodeInstanceTypeForTest exposes the node-label priority helper for tests
// that exercise node-label fallback logic
// (`node.kubernetes.io/instance-type` → `beta.kubernetes.io/instance-type`).
// Test-only; production callers use nodeInstanceType directly.
func NodeInstanceTypeForTest(n *v1.Node) string {
	return nodeInstanceType(n)
}

// SanitizeAckByForTest exposes the ack_by audit-string sanitizer for tests.
// The sanitizer caps length at 256 runes and strips control characters to
// guard against annotation-size DoS and webhook/log injection. Test-only;
// production callers use sanitizeAckBy directly.
func SanitizeAckByForTest(s string) string {
	return sanitizeAckBy(s)
}

// RedactedParamsForTest exposes the comma-joined redactedParams string so the
// alphabetical-order regression test in telemetry_test.go can validate ordering
// without leaking the package-private symbol into production callers. Test-only;
// production code must not reference this hook.
var RedactedParamsForTest = &redactedParams

// SetWebhookClientTimeoutForTest overrides the package-scoped webhook HTTP
// client so unit tests can install a sub-second deadline without waiting
// 30s real time. Returns a restore function that the test must defer to
// reinstate the production 30s timeout. Test-only; production code paths
// must NOT touch this hook.
func SetWebhookClientTimeoutForTest(d time.Duration) func() {
	prev := webhookClient
	prevTimeout := webhookClientTimeout
	webhookClientTimeout = d
	webhookClient = newWebhookClientForTest(d)
	return func() {
		webhookClient = prev
		webhookClientTimeout = prevTimeout
	}
}

func newWebhookClientForTest(d time.Duration) *http.Client {
	return &http.Client{Timeout: d}
}

// SetWebhooksForTest installs a webhook URL slice on the provider so unit
// tests can drive EventSend's dispatch loop without booting the
// controller_webhook informer. Test-only.
func SetWebhooksForTest(p *Provider, urls []string) {
	p.webhooks = urls
}

// DispatchWebhookForTest exposes the package-private dispatch entry point
// for unit tests that exercise transport, status, and timeout paths without
// going through EventSend's goroutine fan-out. Test-only.
func DispatchWebhookForTest(url string, body []byte) error {
	return dispatchWebhook(url, body)
}

// DispatchWebhookSafelyForTest exposes the panic-recovery wrapper for unit
// tests that need to assert recover() catches a deliberate panic. The
// signingKeys arg is variadic so existing call sites that pre-date D.2
// keep working with no source change. Test-only.
func DispatchWebhookSafelyForTest(url string, body []byte, signingKeys ...[]byte) {
	dispatchWebhookSafely(url, body, signingKeys)
}

// RedactURLHostForTest exposes the host-only URL redactor so tests can
// assert query-string secrets never reach log output. Test-only.
func RedactURLHostForTest(raw string) string {
	return redactURLHost(raw)
}

// RedactedWebhookURLForTest exposes the scheme+host URL redactor used by
// :armed/:fired payload emit sites. Distinct from RedactURLHostForTest:
// returns an RFC 3986-valid URL so customer webhook receivers parsing
// payload.webhook_url with new URL(...) don't throw. MF-4 fix.
func RedactedWebhookURLForTest(raw string) string {
	return redactedWebhookURL(raw)
}

// SetDispatchWebhookFnForTest swaps the inner dispatcher invoked from
// within dispatchWebhookSafely's recover scope so unit tests can install a
// stub that panics, returns a specific error, or counts invocations.
// Setting the hook also flips dispatchHookOverridden so the safely-wrapper
// routes through the (url, body) signature; this preserves pre-D.2 test
// stubs that don't know about signingKeys. Returns a restore function the
// test must defer to reinstate the production dispatcher. Test-only.
func SetDispatchWebhookFnForTest(fn func(url string, body []byte) error) func() {
	prev := dispatchWebhookFn
	prevOverride := dispatchHookOverridden
	dispatchWebhookFn = fn
	dispatchHookOverridden = true
	return func() {
		dispatchWebhookFn = prev
		dispatchHookOverridden = prevOverride
	}
}

// SetDispatchWebhookSignedFnForTest swaps the signed dispatcher so D.2
// integration tests can intercept (url, body, signingKeys) calls and
// assert keys are threaded through correctly. Returns a restore function.
// Test-only; production callers MUST NOT touch this hook.
func SetDispatchWebhookSignedFnForTest(fn func(url string, body []byte, signingKeys [][]byte) error) func() {
	prev := dispatchWebhookSignedFn
	dispatchWebhookSignedFn = fn
	return func() { dispatchWebhookSignedFn = prev }
}

// DispatchWebhookSignedForTest exposes the signed dispatcher so unit tests
// can drive a single signed POST without going through EventSend's
// goroutine fan-out. Test-only.
func DispatchWebhookSignedForTest(url string, body []byte, signingKeys [][]byte) error {
	return dispatchWebhookSigned(url, body, signingKeys)
}

// SetWebhookSigningKeyForTest installs a webhook signing key on a test
// Provider so EventSend's parse-and-sign path can be exercised without
// running FromEnv. Test-only; production callers MUST NOT touch.
func SetWebhookSigningKeyForTest(p *Provider, key string) {
	p.WebhookSigningKey = key
}

// SetWebhookClientTransportForTest swaps the http.RoundTripper used by
// the package-scoped webhook client. Used by header-collision tests to
// observe the request as it leaves the client. Returns a restore
// function the test must defer. Test-only.
func SetWebhookClientTransportForTest(rt http.RoundTripper) func() {
	prev := webhookClient
	webhookClient = &http.Client{Timeout: webhookClientTimeout, Transport: rt}
	return func() { webhookClient = prev }
}

// ShutdownServiceForTest exposes the per-service shutdown algorithm
// (paused-replicas annotation + Replicas=0 PATCH + grace-period PATCH).
// Test-only; production callers go through accumulateBudgetTick.
func ShutdownServiceForTest(p *Provider, app, svc string, gracePeriodSeconds int64) error {
	return p.shutdownService(context.Background(), app, svc, gracePeriodSeconds)
}

// ApplyPausedReplicasAnnotationForTest exposes the KEDA paused-replicas
// annotation write path (idempotent).
func ApplyPausedReplicasAnnotationForTest(p *Provider, ns, name string) error {
	return p.applyPausedReplicasAnnotation(context.Background(), ns, name)
}

// WriteBudgetShutdownStateAnnotationForTest exposes the writer for tests
// that need to seed an annotation for restore/GC scenarios.
func WriteBudgetShutdownStateAnnotationForTest(p *Provider, app string, s *structs.AppBudgetShutdownState) error {
	return p.writeBudgetShutdownStateAnnotation(context.Background(), app, s, "")
}

// WriteFlapSuppressedUntilAnnotationForTest exposes the flap-suppressed
// annotation writer so tests can seed the cooldown carry-over state for
// :flap-suppressed lifecycle scenarios. Test-only.
func WriteFlapSuppressedUntilAnnotationForTest(p *Provider, app string, until time.Time) error {
	return p.writeFlapSuppressedUntilAnnotation(context.Background(), app, until)
}

// ReadBudgetShutdownStateAnnotationForTest exposes the parser for tests
// that need to assert the annotation parse error paths (corrupt JSON,
// future schema version, missing required fields).
func ReadBudgetShutdownStateAnnotationForTest(ann map[string]string) (*structs.AppBudgetShutdownState, error) {
	return readBudgetShutdownStateAnnotation(ann)
}

// RestoreFromAnnotationForTest exposes the restore loop for tests that
// drive the per-service restore + pre-flight check path.
func RestoreFromAnnotationForTest(p *Provider, app, ackBy string, state *structs.AppBudgetShutdownState, trigger string) error {
	return p.restoreFromAnnotation(context.Background(), app, ackBy, state, trigger)
}

// RunStaleAnnotationGCForTest exposes the unconditional-GC for tests
// that drive lifecycle-terminal-state cleanup scenarios.
func RunStaleAnnotationGCForTest(p *Provider, app string, tickInterval time.Duration) error {
	return p.runStaleAnnotationGC(context.Background(), app, tickInterval)
}

// GenerateShutdownTickIDForTest exposes the UUID-like generator for
// uniqueness assertions.
func GenerateShutdownTickIDForTest(now time.Time) string {
	return generateShutdownTickID(now)
}

// ShutdownPlanForTest is the test-friendly alias for the unexported
// shutdownPlan struct so the ordering helper can be exercised without
// the full plan-build path.
type ShutdownPlanForTest struct {
	Service     string
	Replicas    int32
	HasKeda     bool
	GraceSecs   int64
	Cost        float64
	LastUpdated time.Time
}

// OrderShutdownPlansForTest exposes the ordering helper.
func OrderShutdownPlansForTest(in []ShutdownPlanForTest, order string) []ShutdownPlanForTest {
	internal := make([]shutdownPlan, len(in))
	for i, p := range in {
		internal[i] = shutdownPlan(p)
	}
	out := orderShutdownPlans(internal, order)
	res := make([]ShutdownPlanForTest, len(out))
	for i, p := range out {
		res[i] = ShutdownPlanForTest(p)
	}
	return res
}

// ReconcileAutoShutdownWithManifestForTest exposes the post-manifest-load
// tail of reconcileAutoShutdown so the full lifecycle (armed → fired →
// restored, plus :expired / :flap-suppressed / :noop reasons) can be
// exercised end-to-end without standing up Atom + ReleaseGet mocking.
// The caller pre-builds the manifest, the namespace pointer (so the
// flap-suppressed and dedup annotation reads find the test fixture), the
// shutdown-state pointer (or nil for first-tick arming), and the budget
// state. Production code path is unaffected — production calls
// reconcileAutoShutdown which loads the manifest then forwards here.
// Test-only; production callers MUST go through accumulateBudgetApp.
func ReconcileAutoShutdownWithManifestForTest(p *Provider, ctx context.Context, app string, cfg *structs.AppBudget, baseState *structs.AppBudgetState, m *manifest.Manifest, now time.Time) {
	if cfg == nil || cfg.AtCapAction != structs.BudgetAtCapActionAutoShutdown {
		return
	}
	if !p.costTrackingEnabled() {
		return
	}
	nsName := p.AppNamespace(app)
	ns, err := p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		return
	}
	state, parseErr := readBudgetShutdownStateAnnotation(ns.Annotations)
	if parseErr != nil {
		// Mirror production: F10 dedup gate + R8.5 F-1 persist-then-emit.
		if !p.stateCorruptDedupExpired(ns.Annotations, now) {
			return
		}
		if perr := p.patchNamespaceStringAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation, now.UTC().Format(time.RFC3339)); perr == nil {
			p.fireFailedEventStateCorrupt(app, cfg, baseState, now)
		}
		return
	}
	// R11.5 F-1 (R11A1): mirror production's clean-parse dedup-clear at
	// budget_auto_shutdown.go:79-82.
	if _, ok := ns.Annotations[structs.BudgetShutdownStateCorruptFiredAtAnnotation]; ok {
		_ = p.deleteNamespaceAnnotation(ctx, app, structs.BudgetShutdownStateCorruptFiredAtAnnotation)
	}
	// R11.5 F-1 (R11A2): mirror production's F8 inline manual-detected
	// armed-window branch at budget_auto_shutdown.go:84-102 (delete-then-emit
	// per R7.5 F-3).
	if state != nil && state.ArmedAt != nil && !state.ArmedAt.IsZero() &&
		(state.ShutdownAt == nil || state.ShutdownAt.IsZero()) {
		if p.armedWindowManuallyScaledUp(ctx, app, state.Services) {
			derr := p.deleteBudgetShutdownStateAnnotation(ctx, app)
			if derr == nil || ae.IsNotFound(derr) {
				p.fireCancelledEventRich(app, cfg, baseState, state, "system", "manual-detected", 0, 0, "", now)
			}
			return
		}
	}
	// Branches (1) and (2) — :expired and manual-detected :restored —
	// run before the manifest is loaded in production. Mirror the order
	// here so tests can exercise them without supplying a manifest the
	// branch wouldn'\''t actually consult.
	if handled := p.reconcileAutoShutdownPreManifest(ctx, app, cfg, state, baseState, now); handled {
		return
	}
	if baseState == nil || baseState.AlertFiredAtCap.IsZero() {
		return
	}
	// Re-read ns so any persistShutdownState writes from the pre-manifest
	// branches are reflected (the production path does not need this
	// because pre-manifest branches return early on success).
	ns, err = p.Cluster.CoreV1().Namespaces().Get(ctx, nsName, am.GetOptions{})
	if err != nil {
		return
	}
	state, _ = readBudgetShutdownStateAnnotation(ns.Annotations)
	p.reconcileAutoShutdownWithManifest(ctx, app, cfg, baseState, ns, state, m, now)
}

// PatchDeploymentWithRetryForTest exposes patchDeploymentWithRetry for
// unit-testing the retry + classification helper without needing a fully
// initialized Provider. F-26 fix (catalog F-26).
func PatchDeploymentWithRetryForTest(ctx context.Context, client kubernetes.Interface, ns, name string, pt types.PatchType, data []byte) (string, error) {
	return patchDeploymentWithRetry(ctx, client, ns, name, pt, data)
}

// SetPatchRetryBackoffsForTest installs a faster backoff schedule for
// the patch-retry helper so tests do not sleep for 5 real seconds.
// Returns a restore function. F-26 fix.
func SetPatchRetryBackoffsForTest(backoffs []time.Duration) func() {
	prev := patchWithRetryBackoffsForTest
	patchWithRetryBackoffsForTest = backoffs
	return func() { patchWithRetryBackoffsForTest = prev }
}

// SetPatchAttemptTimeoutForTest overrides the per-attempt timeout used
// by patchAttemptContext. Pass 0 to disable the timeout entirely. Returns
// a restore function. MF-5 fix (R4 γ-8 A-5).
func SetPatchAttemptTimeoutForTest(d time.Duration) func() {
	prev := patchAttemptTimeoutForTest
	patchAttemptTimeoutForTest = d
	return func() { patchAttemptTimeoutForTest = prev }
}

// PatchAttemptContextForTest exposes patchAttemptContext for unit-testing
// the per-attempt deadline behavior without needing a fully initialized
// Provider. MF-5 fix.
func PatchAttemptContextForTest(parent context.Context) (context.Context, context.CancelFunc) {
	return patchAttemptContext(parent)
}

// AppBudgetLockMapHasForTest reports whether appBudgetLockMap currently
// holds an entry for the given app name. Used by MF-8 tests that verify
// AppDelete drops the per-app mutex so the map doesn't grow unbounded.
func AppBudgetLockMapHasForTest(app string) bool {
	appBudgetLockMapMu.Lock()
	defer appBudgetLockMapMu.Unlock()
	_, ok := appBudgetLockMap[app]
	return ok
}

// AcquireAppBudgetLockForTest is a no-op-on-fast-path helper that ensures
// an appBudgetLockMap entry exists for `app`. Tests use it to seed the map
// before exercising RemoveAppLock. Test-only.
func AcquireAppBudgetLockForTest(app string) {
	_ = appBudgetLock(app)
}
