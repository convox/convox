package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
)

// deprecationSunsetDate returns the RFC 7231 IMF-fixdate when --ack-by will be
// rejected. Pinned to 6 months from anticipated 3.24.6 cut date; update
// structs.SunsetDate3250 at 3.25.0 release-cut.
func deprecationSunsetDate() string {
	return structs.SunsetDate3250
}

// resolveAckByOverride is the single-source-of-truth for the budget-handler
// actor-resolution rule. The rule is:
//
//  1. Derive the actor from the JWT (ConvoxJwtUserParam set by the
//     Authorize middleware; "rack-password" for basic-auth callers per
//     pkg/api/api.go SetReadWriteRole). Trim whitespace; fall back to
//     "unknown" if empty.
//
//  2. If the request body has a non-empty ack_by form param, USE THAT as
//     the persisted actor. This is the customer-truthfulness override
//     that Console relies on today (no per-user JWT plumbing — every
//     basic-auth caller is "rack-password" without this override).
//
//  3. ANY non-empty ack_by triggers the RFC 8594 deprecation signal
//     (Deprecation/Sunset/Link headers + stdout audit trail). The signal
//     fires regardless of whether the override differs from the JWT
//     user — the deprecation is about the FORM-PARAM PATH being
//     deprecated, not about the value comparison. Per-user JWT plumbing
//     lands in 3.25.0; until then ack_by is the bridge AND the migration
//     target.
//
// Returns the actor string the caller should pass to the provider. Both
// the JWT-derived and override paths flow through provider-side
// sanitizeAckBy at write time; this helper does NOT sanitize (the
// provider does the canonical pass).
//
// Stable across the four mutation handlers:
//   - AppBudgetSet              — persists into cfg.LastCapMutationBy + :set event
//   - AppBudgetClear            — emitted in :clear event prev_ack_by metadata
//   - AppBudgetReset            — persists into state.CircuitBreakerAckBy + :reset event
//   - AppBudgetDismissRecovery  — emitted in :dismissed event actor
//
// app is passed in for the stdout audit-trail line (helps operators
// correlate override events to the affected app).
//
// Back-compat firewall: the form-param `ack_by` path predates the 3.24.6
// deprecation infrastructure and is rack-internal API surface. Per the
// 3-year AWS-deprecation horizon for rack-internal surfaces, the rack
// will continue to honor the form-param path indefinitely — Console3
// callers (Console3 holds the rack password and is today's primary
// driver of this path) MUST keep sending the form-param so older racks
// (3.24.5 and below, which lack the `Authorization: Bearer` accept path)
// remain audit-truthful through a long mixed-version window. Dropping
// the form-param requires a coordinated 3.25.0+ rack-side Bearer accept
// path AND a multi-year Console3 dual-path send window. See the spec at
// gpu-ai-inference-vertical/phase-i-post-rc4/phase-b/items/item-04-actor-resolution-verify.md
// for the full deferred-3.25.0 enumeration. The Sunset header date
// emitted below is a courtesy hint per RFC 8594; it is not a binding
// contract and may be extended or removed in a future release.
//
// Empty-string degenerate case: if a caller sends form-param `ack_by`
// empty (e.g. via a future Console3 regression where authenticatedUser
// returns nil and the fallback is dropped), formValue returns "" and the
// `if rawAckBy == ""` short-circuit below returns the JWT-derived actor
// unchanged with no deprecation triple emitted — pre-3.24.6 audit
// behavior, no panic, no half-deprecation signal.
func resolveAckByOverride(c *stdapi.Context, app string) string {
	derived, _ := c.Get(structs.ConvoxJwtUserParam).(string)
	derived = strings.TrimSpace(derived)
	if derived == "" {
		derived = "unknown" // sanitizeAckBy maps "" → "unknown"; defense in depth
	}

	rawAckBy := strings.TrimSpace(formValue(c, "ack_by"))
	if rawAckBy == "" {
		return derived
	}

	// Override sent — emit the deprecation signal AND honor the value.
	// Sunset is HTTP-date per RFC 7231 §7.1.1.1; Link rel="deprecation"
	// follows RFC 8631 §4.6 + RFC 8288.
	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", deprecationSunsetDate())
	c.Response().Header().Set("Link", `<https://docs.convox.com/migration/ack-by-derivation>; rel="deprecation"; type="text/html"`)
	// stdout audit trail of the override (operator-side via fluentd):
	fmt.Printf("ns=api at=warn kind=ack_by_override app=%s client_supplied=%q jwt_user=%q\n",
		app, rawAckBy, derived)
	return rawAckBy
}

// formValue reads a form parameter, honoring DELETE bodies that Go's
// stdlib r.ParseForm skips. The SDK (sdk/methods.go::AppBudgetClear)
// sends ack_by via x-www-form-urlencoded body on DELETE for the
// AppBudgetClear endpoint; without this manual parse, c.Value("ack_by")
// returns "" on DELETE and the override is silently dropped, defeating
// the customer-truthfulness contract.
//
// Idempotent: parses at most once per request (gated on r.PostForm ==
// nil). Body is re-buffered into r.Body so any downstream consumer that
// re-reads the request still sees the original payload.
//
// Stdlib interaction: net/http's r.ParseForm has two short-circuit
// gates depending on method. For POST/PUT/PATCH it skips body parsing
// when r.PostForm != nil; for GET/DELETE/HEAD it skips URL-query
// parsing when r.Form != nil. We populate BOTH here so any downstream
// caller (regardless of method) sees the same map and reads other
// fields (e.g. monthly-cap-usd) without confusion. AppBudgetClear
// doesn't currently call UnmarshalOptions, but the contract is sound
// for any future handler that pairs the helper with body unmarshaling.
//
// Form-merge precedence: when r.Form already has entries (e.g. from a
// URL query string parsed by an earlier ParseForm call), the manually-
// parsed body values are APPENDED — preserving Go's stdlib precedence
// where r.FormValue returns r.Form[k][0], so URL query wins over body
// when both are present. No current caller sends ack_by via both, but
// the precedence matches stdlib behavior either way.
func formValue(c *stdapi.Context, name string) string {
	r := c.Request()
	if r.Method == http.MethodDelete && r.PostForm == nil &&
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		if vals, err := url.ParseQuery(string(body)); err == nil {
			r.PostForm = vals
			if r.Form == nil {
				r.Form = url.Values{}
			}
			for k, v := range vals {
				r.Form[k] = append(r.Form[k], v...)
			}
		}
	}
	return c.Value(name)
}
