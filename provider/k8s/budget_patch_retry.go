package k8s

import (
	"context"
	"time"

	"github.com/convox/convox/pkg/structs"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// patchWithRetryAttempts is the number of attempts (1 initial + 2 retries
// per the spec §8.7 promise of 3-attempt exponential backoff).
const patchWithRetryAttempts = 3

// patchWithRetryBackoffsForTest lists the sleeps applied after attempts
// 1 and 2 (the third attempt is the last, no sleep follows). Per spec
// §8.7 line 848: "1s, 4s, 16s exponential backoff". Two retries means we
// sleep once after attempt 1 and once after attempt 2; the third failure
// surfaces. Tests override this to install zero-duration backoffs so
// retry-classification assertions don't require real sleeps.
var patchWithRetryBackoffsForTest = []time.Duration{1 * time.Second, 4 * time.Second}

// patchAttemptTimeoutForTest bounds each individual Patch call so a hung
// K8s API server can't hold the per-app advisory lock indefinitely. F-26
// previously relied on the caller's ctx for cancellation; under typical
// rack invocation the caller passes context.Background(), giving an
// effectively unbounded deadline. 30 s is well above any healthy K8s
// admission/validation latency and bounds worst-case lock-hold to ~90 s
// across all 3 attempts. Tests override to 0 to skip timing.
var patchAttemptTimeoutForTest = 30 * time.Second

// classifyPatchError maps a Kubernetes API error onto one of the canonical
// :failed reasons defined in pkg/structs/budget.go. The mapping is centralized
// here so all PATCH call sites surface the same reason for the same error
// shape — receivers parsing the failure_reason field can rely on the enum
// values being stable.
//
// F-26 fix (catalog D-4): adds 4 new classifications beyond the prior
// blanket "k8s-api-failure". Per spec §8.7 the 6 canonical reasons are:
//
//	admission-rejected   — apierrors.IsForbidden (admission webhook said no)
//	annotation-rejected  — apierrors.IsInvalid (PATCH body rejected at validation)
//	cooldown-write-failed — apierrors.IsConflict after retry exhausted
//	schema-incompatible  — apierrors.IsServerTimeout / version-skew style
//	state-corrupt        — handled at the read path (state JSON unparseable)
//	k8s-api-failure      — fallback for unrecognized errors
func classifyPatchError(err error, exhaustedConflict bool) string {
	if err == nil {
		return ""
	}
	switch {
	case ae.IsForbidden(err):
		return structs.BudgetShutdownReasonAdmissionRejected
	case ae.IsInvalid(err):
		return structs.BudgetShutdownReasonAnnotationRejected
	case exhaustedConflict && ae.IsConflict(err):
		return structs.BudgetShutdownReasonCooldownWriteFailed
	case ae.IsServerTimeout(err):
		// version-skew / serialization-mismatch presents as ServerTimeout
		// on some K8s versions; classify as schema-incompatible so the
		// FAILED banner gives the operator a meaningful hint.
		return structs.BudgetShutdownReasonSchemaIncompatible
	default:
		return structs.BudgetShutdownReasonK8sApiFailure
	}
}

// patchDeploymentWithRetry wraps Cluster.AppsV1().Deployments(ns).Patch
// in a 3-attempt exponential-backoff retry loop. On final failure
// returns the classified reason and the underlying error.
//
// Behavior:
//   - Idempotent retry on transient errors (Conflict, ServerTimeout)
//   - Fast-fail on non-retryable errors (Forbidden, Invalid, NotFound)
//   - Final failure returns a reason from classifyPatchError so callers
//     can populate AppBudgetShutdownState.FailureReason.
//
// F-26 fix (catalog D-4): spec §8.7 promised retry; pre-3.24.6 PATCH
// sites issued a single attempt and surfaced any error as
// "k8s-api-failure" without classification. Now centralized.
func patchDeploymentWithRetry(ctx context.Context, client kubernetes.Interface, ns, name string, pt types.PatchType, data []byte) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= patchWithRetryAttempts; attempt++ {
		attemptCtx, cancel := patchAttemptContext(ctx)
		_, err := client.AppsV1().Deployments(ns).Patch(attemptCtx, name, pt, data, am.PatchOptions{})
		cancel()
		if err == nil {
			return "", nil
		}
		lastErr = err
		if ae.IsForbidden(err) || ae.IsInvalid(err) || ae.IsNotFound(err) {
			break
		}
		if attempt == patchWithRetryAttempts {
			break
		}
		if attempt-1 < len(patchWithRetryBackoffsForTest) {
			select {
			case <-ctx.Done():
				return classifyPatchError(ctx.Err(), false), ctx.Err()
			case <-time.After(patchWithRetryBackoffsForTest[attempt-1]):
			}
		}
	}
	exhaustedConflict := lastErr != nil && ae.IsConflict(lastErr)
	return classifyPatchError(lastErr, exhaustedConflict), lastErr
}

// patchAttemptContext wraps the caller's ctx with a per-attempt deadline
// derived from patchAttemptTimeoutForTest. Returns a no-op cancel when the
// timeout is non-positive (test override). Caller MUST invoke cancel()
// after each attempt to release the timer goroutine.
func patchAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	if patchAttemptTimeoutForTest <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, patchAttemptTimeoutForTest)
}

// patchDynamicWithRetry is the dynamic-client variant for ScaledObject
// PATCH operations (KEDA paused-replicas annotation set/clear). Same
// retry + classification semantics as patchDeploymentWithRetry.
func patchDynamicWithRetry(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, ns, name string, pt types.PatchType, data []byte) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= patchWithRetryAttempts; attempt++ {
		attemptCtx, cancel := patchAttemptContext(ctx)
		_, err := client.Resource(gvr).Namespace(ns).Patch(attemptCtx, name, pt, data, am.PatchOptions{})
		cancel()
		if err == nil {
			return "", nil
		}
		lastErr = err
		if ae.IsForbidden(err) || ae.IsInvalid(err) || ae.IsNotFound(err) {
			break
		}
		if attempt == patchWithRetryAttempts {
			break
		}
		if attempt-1 < len(patchWithRetryBackoffsForTest) {
			select {
			case <-ctx.Done():
				return classifyPatchError(ctx.Err(), false), ctx.Err()
			case <-time.After(patchWithRetryBackoffsForTest[attempt-1]):
			}
		}
	}
	exhaustedConflict := lastErr != nil && ae.IsConflict(lastErr)
	return classifyPatchError(lastErr, exhaustedConflict), lastErr
}
