package structs

const (
	ConvoxRoleParam     = "CONVOX_ROLE"
	ConvoxRoleRead      = "r"
	ConvoxRoleReadWrite = "rw"
	ConvoxRoleAdmin     = "rwa" // contains both "r" and "w" substrings for 3.24.5 backward-compat

	// CONVOX_ROLE substring registry — MUST extend, never collide.
	// "r" — read tier (ConvoxRoleRead)
	// "w" — write tier (ConvoxRoleReadWrite)
	// "a" — admin tier (ConvoxRoleAdmin); RESERVED for "Admin" semantic, NOT for any future
	//        role whose primary semantic differs (no "audit", "auth", etc. on this letter).
	// Future role tiers MUST extend the existing string (e.g. "rwax") and MUST NOT introduce
	// new single-char predicates that could collide with existing role-strings.

	// CONVOX_JWT_USER param — JWT user claim, exposed to handlers and propagated
	// into provider context for audit-event actor derivation. Set by the
	// authenticate middleware on the JWT-auth path (data.User from the verified
	// claim) and threaded into context via contextFrom; ContextActor reads it
	// off the provider's stored ctx so EventSend can populate the audit-event
	// "actor" field downstream (audit trail for SOC 2 CC6.2 / HIPAA §164.312(b) /
	// EU AI Act Article 12 record-keeping).
	ConvoxJwtUserParam = "CONVOX_JWT_USER"

	// SunsetDate3250 — RFC 7231 IMF-fixdate format (RFC 8594 Sunset header
	// value). Pinned 6 months from anticipated 3.24.6 cut date (~2026-05);
	// update at 3.25.0 release-cut. Day-name "Thu" is calendar-correct for
	// 2026-10-01 — the format-guard test TestDeprecationSunsetDate_IsValidRFC7231
	// re-parses via http.ParseTime so a future drift to a wrong day-name will
	// be caught. Used by D.4's deprecationSunsetDate helper at
	// pkg/api/deprecation.go to populate the Sunset response header on
	// AppBudget* override-detection paths.
	SunsetDate3250 = "Thu, 01 Oct 2026 00:00:00 GMT"
)

// ConvoxJwtUserCtxKey is the typed context.WithValue key used by
// pkg/api.contextFrom and provider/k8s.ContextActor to thread the JWT user
// claim through the request-scoped Provider context. The string-keyed
// equivalent (ConvoxJwtUserParam) is the c.Set/c.Get key used at the
// stdapi.Context boundary — keeping the string and the typed key as a
// matched pair means handlers stay readable while the deeper context plumbing
// avoids the SA1029 collision warning.
type contextJwtUserKey struct{}

// ConvoxJwtUserCtxKey is exported as a sentinel value (zero-valued struct) so
// callers reference structs.ConvoxJwtUserCtxKey rather than constructing the
// type themselves.
var ConvoxJwtUserCtxKey = contextJwtUserKey{}

// ConvoxTIDCtxKey is the typed context.WithValue key for the multi-tenant
// boundary identifier propagated through pkg/api.contextFrom and read by
// provider/k8s.ContextTID. Used for namespace labeling, app-list scoping,
// service URL routing, and build/pod-env injection. Set by Convox Cloud
// (console3) on every proxied request.
//
// On-the-wire header name: `Convox-TID` (canonical, RFC 6648 compliant)
// OR `X-Convox-TID` (legacy form — still accepted for backward
// compatibility with older Cloud releases). Both forms map to the same
// ctx key; canonical wins when both are present. See pkg/api/helpers.go
// MF-13 fix for the dual-read implementation.
//
// Typed-key avoids the SA1029 collision warning while keeping the
// header-on-the-wire as the source of identity.
type contextTIDKey struct{}

// ConvoxTIDCtxKey is the sentinel typed key (zero-valued struct).
var ConvoxTIDCtxKey = contextTIDKey{}
