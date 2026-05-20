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

const patchWithRetryAttempts = 3

var patchWithRetryBackoffsForTest = []time.Duration{1 * time.Second, 4 * time.Second}
var patchAttemptTimeoutForTest = 30 * time.Second

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
		return structs.BudgetShutdownReasonSchemaIncompatible
	default:
		return structs.BudgetShutdownReasonK8sApiFailure
	}
}

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

func patchAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	if patchAttemptTimeoutForTest <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, patchAttemptTimeoutForTest)
}

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
