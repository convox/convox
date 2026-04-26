package k8s_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// patchRetryDeploymentFixture creates a minimal Deployment in the fake
// clientset so the PATCH reactor has a target. The reactor intercepts
// the actual patch call before the underlying object is touched.
func patchRetryDeploymentFixture(t *testing.T, c *fake.Clientset, ns, name string) {
	t.Helper()
	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: am.ObjectMeta{Name: name, Namespace: ns},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	_, err := c.AppsV1().Deployments(ns).Create(context.Background(), dep, am.CreateOptions{})
	require.NoError(t, err)
}

// TestPatchWithRetry_RetriesThreeTimesOnTransientError — F-26 fix
// (catalog F-26). Confirms the 3-attempt loop fires on transient errors
// (Conflict). The final error surfaces with the canonical reason
// `cooldown-write-failed` per spec §8.7.
func TestPatchWithRetry_RetriesThreeTimesOnTransientError(t *testing.T) {
	restore := k8s.SetPatchRetryBackoffsForTest([]time.Duration{0, 0})
	defer restore()

	c := fake.NewSimpleClientset()
	patchRetryDeploymentFixture(t, c, "ns1", "svc1")

	var attempts int
	c.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, ae.NewConflict(schema.GroupResource{Resource: "deployments"}, "svc1", errors.New("write conflict"))
	})

	reason, perr := k8s.PatchDeploymentWithRetryForTest(context.Background(), c, "ns1", "svc1", types.MergePatchType, []byte("{}"))
	require.Error(t, perr, "exhausted retries must surface error")
	assert.Equal(t, structs.BudgetShutdownReasonCooldownWriteFailed, reason,
		"3-attempt exhausted Conflict must classify as cooldown-write-failed")
	assert.Equal(t, 3, attempts, "must attempt exactly 3 times before surfacing")
}

// TestPatchWithRetry_ClassifiesForbidden — F-26 fix.
// Forbidden errors are non-retryable (admission webhook said no); fail
// fast with the canonical reason `admission-rejected`.
func TestPatchWithRetry_ClassifiesForbidden(t *testing.T) {
	restore := k8s.SetPatchRetryBackoffsForTest([]time.Duration{0, 0})
	defer restore()

	c := fake.NewSimpleClientset()
	patchRetryDeploymentFixture(t, c, "ns1", "svc1")

	var attempts int
	c.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, ae.NewForbidden(schema.GroupResource{Resource: "deployments"}, "svc1", errors.New("admission denied"))
	})

	reason, perr := k8s.PatchDeploymentWithRetryForTest(context.Background(), c, "ns1", "svc1", types.MergePatchType, []byte("{}"))
	require.Error(t, perr)
	assert.Equal(t, structs.BudgetShutdownReasonAdmissionRejected, reason,
		"Forbidden must classify as admission-rejected")
	assert.Equal(t, 1, attempts, "Forbidden is non-retryable; must attempt exactly once")
}

// TestPatchWithRetry_ClassifiesInvalid — F-26 fix.
// Invalid errors are non-retryable; fail fast with `annotation-rejected`.
func TestPatchWithRetry_ClassifiesInvalid(t *testing.T) {
	restore := k8s.SetPatchRetryBackoffsForTest([]time.Duration{0, 0})
	defer restore()

	c := fake.NewSimpleClientset()
	patchRetryDeploymentFixture(t, c, "ns1", "svc1")

	var attempts int
	c.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, ae.NewInvalid(schema.GroupKind{Kind: "Deployment"}, "svc1", nil)
	})

	reason, perr := k8s.PatchDeploymentWithRetryForTest(context.Background(), c, "ns1", "svc1", types.MergePatchType, []byte("{}"))
	require.Error(t, perr)
	assert.Equal(t, structs.BudgetShutdownReasonAnnotationRejected, reason,
		"Invalid must classify as annotation-rejected")
	assert.Equal(t, 1, attempts, "Invalid is non-retryable; must attempt exactly once")
}

// TestPatchWithRetry_ClassifiesConflictAfterExhausted — F-26 fix.
// Conflict surfaced after retry exhaustion is `cooldown-write-failed`.
// Companion to TestPatchWithRetry_RetriesThreeTimesOnTransientError —
// asserts the underlying error remains a Conflict for callers that
// want to introspect via apierrors.IsConflict.
func TestPatchWithRetry_ClassifiesConflictAfterExhausted(t *testing.T) {
	restore := k8s.SetPatchRetryBackoffsForTest([]time.Duration{0, 0})
	defer restore()

	c := fake.NewSimpleClientset()
	patchRetryDeploymentFixture(t, c, "ns1", "svc1")

	c.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, ae.NewConflict(schema.GroupResource{Resource: "deployments"}, "svc1", errors.New("write conflict"))
	})

	reason, perr := k8s.PatchDeploymentWithRetryForTest(context.Background(), c, "ns1", "svc1", types.MergePatchType, []byte("{}"))
	require.Error(t, perr)
	assert.True(t, ae.IsConflict(perr), "underlying error must be Conflict for caller introspection")
	assert.Equal(t, structs.BudgetShutdownReasonCooldownWriteFailed, reason,
		"exhausted Conflict must surface cooldown-write-failed")
}

// TestPatchWithRetry_ClassifiesServerTimeout — MF-3 fix (R4 γ-10 ADV-K8S-11).
// ServerTimeout is the K8s-server-side signal for version-skew /
// serialization-mismatch; classify as `schema-incompatible` so the FAILED
// banner surfaces a meaningful operator hint. ServerTimeout is treated as
// a transient class — retry up to 3 times before surfacing.
func TestPatchWithRetry_ClassifiesServerTimeout(t *testing.T) {
	restore := k8s.SetPatchRetryBackoffsForTest([]time.Duration{0, 0})
	defer restore()

	c := fake.NewSimpleClientset()
	patchRetryDeploymentFixture(t, c, "ns1", "svc1")

	var attempts int
	c.PrependReactor("patch", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		return true, nil, ae.NewServerTimeout(schema.GroupResource{Resource: "deployments"}, "patch", 1)
	})

	reason, perr := k8s.PatchDeploymentWithRetryForTest(context.Background(), c, "ns1", "svc1", types.MergePatchType, []byte("{}"))
	require.Error(t, perr)
	assert.True(t, ae.IsServerTimeout(perr), "underlying error must be ServerTimeout for caller introspection")
	assert.Equal(t, structs.BudgetShutdownReasonSchemaIncompatible, reason,
		"ServerTimeout must classify as schema-incompatible")
	assert.Equal(t, 3, attempts, "ServerTimeout is a transient class; must retry 3 times")
}

// TestPatchAttemptContext_DefaultsToBoundedDeadline — MF-5 fix (R4 γ-8 A-5).
// Confirms patchAttemptContext wraps each call with a bounded deadline so
// a hung K8s API server can't hold the per-app advisory lock indefinitely.
// Default is 30s; deadline must be set on the returned context.
func TestPatchAttemptContext_DefaultsToBoundedDeadline(t *testing.T) {
	ctx, cancel := k8s.PatchAttemptContextForTest(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	require.True(t, ok, "patchAttemptContext must set a deadline by default")
	remaining := time.Until(deadline)
	assert.True(t, remaining > 25*time.Second && remaining <= 30*time.Second,
		"deadline should be ~30s in the future, got %v", remaining)
}

// TestPatchAttemptContext_ZeroTimeoutDisables — MF-5 fix.
// When patchAttemptTimeoutForTest is zeroed (test override), the helper
// returns the parent context untouched. This is required for tests that
// install per-test ctx with shorter deadlines or none at all.
func TestPatchAttemptContext_ZeroTimeoutDisables(t *testing.T) {
	restore := k8s.SetPatchAttemptTimeoutForTest(0)
	defer restore()

	parent := context.Background()
	ctx, cancel := k8s.PatchAttemptContextForTest(parent)
	defer cancel()

	_, ok := ctx.Deadline()
	assert.False(t, ok, "zero timeout must disable context.WithTimeout (parent context returned)")
}
