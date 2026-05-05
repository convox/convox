package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/assert"
)

// TestAuthenticate_BasicAuthWithConvoxActorHeader_SetsActor verifies the
// 3.24.6 universal-actor-attribution bridge: when a basic-auth caller
// supplies the canonical Convox-Actor header on a rack mutation, the auth
// middleware uses the header value as the audit actor identity instead of
// the generic "rack-password" sentinel. The header value flows through
// ContextActor() to EventSend's central injection, so every audit event
// stamped during the request lands the customer-truthful actor without
// per-controller form-param plumbing.
//
// Trust model: anyone with the rack password can already do anything as
// root; the header is a customer-truthfulness override, not a security
// boundary. Forged identities through this header are no worse than
// forged identities passed via the existing form-param `ack_by` path.
func TestAuthenticate_BasicAuthWithConvoxActorHeader_SetsActor(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("Convox-Actor", "alice@example.com")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code, "basic-auth with Convox-Actor must return 200")
	assert.Equal(t, "alice@example.com", got,
		"ConvoxJwtUserParam must be set to the Convox-Actor header value")
}

// TestAuthenticate_BasicAuthWithLegacyXConvoxActorHeader_SetsActor —
// dual-read pattern (matches the X-Convox-TID precedent at
// pkg/api/helpers.go:32-35). Pre-3.24.6 callers may continue sending
// X-Convox-Actor; the rack honors the legacy form when no canonical
// Convox-Actor header is present.
func TestAuthenticate_BasicAuthWithLegacyXConvoxActorHeader_SetsActor(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("X-Convox-Actor", "bob@example.com")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code, "basic-auth with X-Convox-Actor must return 200")
	assert.Equal(t, "bob@example.com", got,
		"ConvoxJwtUserParam must be set to the legacy X-Convox-Actor header value")
}

// TestAuthenticate_BasicAuthWithBothHeaders_CanonicalWins — when both
// canonical and legacy forms are present (e.g. a Console3 upgrade in
// flight where one proxy sends both for safety), canonical wins. Matches
// the X-Convox-TID precedent at pkg/api/helpers.go.
func TestAuthenticate_BasicAuthWithBothHeaders_CanonicalWins(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("X-Convox-Actor", "legacy@example.com")
	req.Header.Set("Convox-Actor", "canonical@example.com")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "canonical@example.com", got,
		"when both forms present, canonical Convox-Actor must win over legacy X-Convox-Actor")
}

// TestAuthenticate_BasicAuthEmptyActorHeader_FallsBackToRackPassword —
// degenerate case: header sent but value is empty. Must NOT poison the
// audit record with empty-string; falls through to the literal
// "rack-password" sentinel that pre-3.24.6 callers see.
func TestAuthenticate_BasicAuthEmptyActorHeader_FallsBackToRackPassword(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("Convox-Actor", "")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "rack-password", got,
		"empty Convox-Actor must fall back to the rack-password sentinel")
}

// TestAuthenticate_BasicAuthWhitespaceActorHeader_FallsBackToRackPassword —
// whitespace-only header value (TrimSpace yields ""). Must fall back to
// the rack-password sentinel; sanitizing whitespace-only would otherwise
// produce "unknown" which pre-3.24.6 callers do not expect on the
// basic-auth path. Pin: pre-trim falls back BEFORE sanitization.
func TestAuthenticate_BasicAuthWhitespaceActorHeader_FallsBackToRackPassword(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("Convox-Actor", "   \t  ")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "rack-password", got,
		"whitespace-only Convox-Actor must fall back to the rack-password sentinel")
}

// TestAuthenticate_BasicAuthHostileActorHeader_Sanitized verifies the
// defense-in-depth requirement: hostile header values containing C0/C1,
// BiDi overrides, or zero-width characters are stripped before being
// stamped into ConvoxJwtUserParam. The sanitization runs through the
// shared pkg/audit.SanitizeActor helper which is the single canonical
// guarantee that no control char ever reaches the audit-event payload.
func TestAuthenticate_BasicAuthHostileActorHeader_Sanitized(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	// Right-to-left override (U+202E) + zero-width space (U+200B). A
	// rendered audit-log viewer would otherwise display this as a
	// reversed/obfuscated email. The raw bytes are constructed via
	// concatenation rather than embedded literals so the source text
	// stays readable.
	hostile := "alice@example.com" + "‮" + "test" + "​" + "foo"
	want := "alice@example.comtestfoo"

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("Convox-Actor", hostile)

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, want, got,
		"hostile Convox-Actor header must be sanitized through pkg/audit.SanitizeActor")
}

// TestAuthenticate_JwtAuth_IgnoresActorHeader pins the precedence
// invariant: when the request authenticates via JWT, the JWT data.User
// claim wins unconditionally. The Convox-Actor header is a basic-auth-
// path override only; honoring it on the JWT branch would give a
// holder of a JWT carte-blanche to spoof any user identity in audit
// events, which the JWT path is explicitly designed to prevent.
//
// Once Console3 migrates to per-user JWT minting in 3.25.0, the
// Convox-Actor header becomes dead code on Console3 paths. The header
// branch lives in the basic-auth `else` only so the JWT branch's
// "claim wins" guarantee is preserved.
func TestAuthenticate_JwtAuth_IgnoresActorHeader(t *testing.T) {
	jm := jwt.NewJwtManager("TEST")
	s := &Server{JwtMngr: jm}

	tok := mintJwtTokenForTest(t, jm, "rw")
	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("jwt", tok))
	req.Header.Set("Convox-Actor", "spoofed@example.com")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code, "JWT-auth path must return 200 for valid token")
	assert.Equal(t, "system-write", got,
		"JWT data.User claim must win — Convox-Actor header MUST NOT override on the JWT branch")
}

// TestAuthenticate_BasicAuthCanonicalHostileOnly_FallsBackToLegacy pins the
// round-2 review finding R2-A1-1 fix: when the canonical Convox-Actor header
// contains ONLY strip-set chars (e.g. bidi overrides, zero-width characters
// that TrimSpace does NOT recognize as whitespace), SanitizeActor returns
// "unknown" — and the auth middleware MUST fall through to the legacy
// X-Convox-Actor header rather than silently null-routing the legitimate
// legacy attribution by stamping "unknown".
//
// Without this guard, a hostile MITM / browser extension / buggy proxy
// that injects e.g. "Convox-Actor: ‮‮‮" while leaving X-Convox-Actor
// clean would suppress the real attribution.
func TestAuthenticate_BasicAuthCanonicalHostileOnly_FallsBackToLegacy(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	// Three RLO bidi-override runes — NOT TrimSpace'd by Go (not IsSpace),
	// stripped by SanitizeActor, sanitize-to-empty returns "unknown".
	req.Header.Set("Convox-Actor", "‮‮‮")
	req.Header.Set("X-Convox-Actor", "legacy@example.com")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "legacy@example.com", got,
		"hostile-canonical-only must fall through to legacy, NOT stamp 'unknown'")
}

// TestAuthenticate_BasicAuthBothHostile_FallsBackToRackPassword —
// when BOTH canonical and legacy headers sanitize to "unknown" (e.g. both
// contain only strip-set chars), the middleware falls through to the
// "rack-password" sentinel rather than stamping "unknown".
func TestAuthenticate_BasicAuthBothHostile_FallsBackToRackPassword(t *testing.T) {
	s := &Server{JwtMngr: nil, Password: "rack-pass"}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/auth", nil)
	req.Header.Set("Authorization", basicAuthHeader("convox", "rack-pass"))
	req.Header.Set("Convox-Actor", "‮‮‮")
	req.Header.Set("X-Convox-Actor", "​​")

	got, code := captureMiddlewareValue(t, s, req, structs.ConvoxJwtUserParam)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "rack-password", got,
		"both-hostile must fall through to rack-password, NOT stamp 'unknown'")
}
