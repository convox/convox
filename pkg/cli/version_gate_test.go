package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsRackVersionGated_MatchesStdsdkErrorFormat pins the contract with
// stdsdk's error-format string. If stdsdk ever changes its
// `fmt.Errorf("response status %d", res.StatusCode)` literal, this test
// fails and forces the predicate to be re-aligned.
func TestIsRackVersionGated_MatchesStdsdkErrorFormat(t *testing.T) {
	// Match the literal stdsdk emits for a 404 response.
	err := fmt.Errorf("response status %d", 404)
	assert.True(t, isRackVersionGated(err), "must recognize stdsdk 404 error format")
}

// TestIsRackVersionGated_CaseInsensitive verifies that re-wrapped error
// strings (different casing from a reverse proxy, ingress, or log
// middleware) still match the predicate.
func TestIsRackVersionGated_CaseInsensitive(t *testing.T) {
	upper := errors.New("RESPONSE STATUS 404")
	mixed := errors.New("Response Status 404")
	lower := errors.New("response status 404")
	assert.True(t, isRackVersionGated(upper), "upper-case re-wrap must match")
	assert.True(t, isRackVersionGated(mixed), "mixed-case re-wrap must match")
	assert.True(t, isRackVersionGated(lower), "lower-case literal must match")
}

// TestIsRackVersionGated_NilError covers the nil short-circuit.
func TestIsRackVersionGated_NilError(t *testing.T) {
	assert.False(t, isRackVersionGated(nil), "nil error must not match")
}

// TestIsRackVersionGated_DoesNotMatchAppNotFound is the critical negative —
// substring "404" or "not found" alone MUST NOT match, otherwise we'd
// swallow legitimate "app not found" / "namespace not found" errors and
// leak them as misleading version-gated banners.
func TestIsRackVersionGated_DoesNotMatchAppNotFound(t *testing.T) {
	cases := []error{
		errors.New(`namespaces "my-app" not found`),
		errors.New("404 Not Found"), // bare-404 without "response status" prefix
		errors.New("app not found"),
		errors.New("resource not found"),
	}
	for _, e := range cases {
		assert.False(t, isRackVersionGated(e), "must NOT match: %v", e)
	}
}

// TestIsRackVersionGated_DoesNotMatchTransientErrors covers errors that
// could be misclassified as version-gated but are actually transient
// (network blip, rack restart, internal server error).
func TestIsRackVersionGated_DoesNotMatchTransientErrors(t *testing.T) {
	cases := []error{
		errors.New("response status 502"),
		errors.New("response status 503"),
		errors.New("connection refused"),
		errors.New("context deadline exceeded"),
	}
	for _, e := range cases {
		assert.False(t, isRackVersionGated(e), "must NOT match: %v", e)
	}
}

// TestWrapVersionGate_TranslatesMatchedError verifies the friendly message
// shape and feature name interpolation.
func TestWrapVersionGate_TranslatesMatchedError(t *testing.T) {
	err := fmt.Errorf("response status %d", 404)
	got := wrapVersionGate(err, "budget caps")
	assert.EqualError(t, got, "budget caps requires rack version 3.24.6 or later")
}

// TestWrapVersionGate_PassesThroughNonGatedError verifies the predicate-
// negative branch: non-version-gated errors are returned unchanged so the
// caller still sees the underlying transient / app-not-found error.
func TestWrapVersionGate_PassesThroughNonGatedError(t *testing.T) {
	original := errors.New(`namespaces "my-app" not found`)
	got := wrapVersionGate(original, "budget caps")
	assert.Equal(t, original, got, "non-gated error must pass through unchanged")
}

// TestWrapVersionGate_NilStaysNil verifies the nil-error short-circuit.
func TestWrapVersionGate_NilStaysNil(t *testing.T) {
	assert.NoError(t, wrapVersionGate(nil, "budget caps"))
}
