package structs

const (
	ConvoxRoleParam     = "CONVOX_ROLE"
	ConvoxRoleRead      = "r"
	ConvoxRoleReadWrite = "rw"
	ConvoxRoleAdmin     = "rwa" // contains "r" and "w" substrings for backward-compat

	// ConvoxJwtUserParam is the audit-trail actor identity from JWT claims.
	ConvoxJwtUserParam = "CONVOX_JWT_USER"

	// SunsetDate3250 is the RFC 7231 Sunset header for AppBudget deprecation; update at 3.25.0.
	SunsetDate3250 = "Thu, 01 Oct 2026 00:00:00 GMT"
)

// Typed context keys avoid SA1029 string-key collisions in context.WithValue.

type contextJwtUserKey struct{}

// ConvoxJwtUserCtxKey threads JWT user claim through request-scoped context.
var ConvoxJwtUserCtxKey = contextJwtUserKey{}

type contextTIDKey struct{}

// ConvoxTIDCtxKey threads the multi-tenant boundary ID through request-scoped context.
var ConvoxTIDCtxKey = contextTIDKey{}
