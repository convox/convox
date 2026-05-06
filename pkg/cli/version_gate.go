package cli

import (
	"fmt"
	"strings"
)

// isRackVersionGated detects errors from racks that lack a feature endpoint
// added in a newer rack version. The SDK flattens HTTP errors to plain Go
// errors prefixed with "response status %d"; for an endpoint the rack does
// not register, gorilla/mux's default NotFoundHandler returns HTTP 404. The
// substring match is case-insensitive to tolerate transport-layer error
// re-wrapping (reverse proxies, ingress, log middleware).
//
// We deliberately match ONLY "response status 404" — not bare "404" or
// "not found" — because those substrings appear in errors we MUST NOT
// swallow (e.g. `namespaces "my-app" not found` when the user mistypes
// the app name). Hiding that behind a version-gated banner would mislead.
//
// This helper is the canonical predicate for any new SDK method consumer
// that needs to differentiate "endpoint exists but returned 404 for the
// resource" from "endpoint does not exist on this rack version". The
// per-feature wrappers below (wrapVersionGate) compose this with a
// feature-specific friendly message.
func isRackVersionGated(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "response status 404")
}

// wrapVersionGate translates a rack-not-supported error into a friendly
// version-gated message naming the missing feature. Returns the original
// error untouched when it does not match the version-gate pattern.
//
// Usage:
//
//	if err := rack.AppBudgetGet(app); err != nil {
//	    return wrapVersionGate(err, "budget caps")
//	}
//
// The output text follows the pattern:
//
//	"<feature> requires rack version 3.24.6 or later"
//
// User-facing surface — every caller MUST use this exact form so
// release notes and help text can refer to a single canonical phrasing.
func wrapVersionGate(err error, feature string) error {
	if !isRackVersionGated(err) {
		return err
	}
	return fmt.Errorf("%s requires rack version 3.24.6 or later", feature)
}
