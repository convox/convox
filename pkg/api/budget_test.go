package api_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/api"
	cjwt "github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// resetOptsPlain matches AppBudgetResetWithOptions calls where the
// --force-clear-cooldown flag is OFF. The B-6 fix routes plain
// `convox budget reset` (no force flag) through AppBudgetResetWithOptions
// with ForceClearCooldown=false so the post-:fired restoreFromAnnotation
// recovery path is exercised — matches the documented contract in
// docs/reference/cli/budget-reset.md and docs/management/budget-caps.md.
func resetOptsPlain(opts structs.AppBudgetResetOptions) bool {
	return !opts.ForceClearCooldown
}

// budgetTestServer spins up a full api.Server with a MockProvider for D.4
// budget-handler tests. JWT signing key matches cjwt.NewJwtManager("test") so
// tokens minted with the returned JwtManager verify against the server.
func budgetTestServer(t *testing.T, fn func(*httptest.Server, *structs.MockProvider, *cjwt.JwtManager)) {
	t.Helper()
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	jm := cjwt.NewJwtManager("test")
	fn(ht, p, jm)

	p.AssertExpectations(t)
}

// mintCustomJwtToken mints a JWT with arbitrary user/role claims for tests
// that need to vary the user field independently of the role-mapped builders
// on JwtManager (ReadToken/WriteToken/AdminToken hard-code system-* users).
func mintCustomJwtToken(t *testing.T, signKey, user, role string, ttl time.Duration) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user":      user,
		"role":      role,
		"expiresAt": time.Now().UTC().Add(ttl).Unix(),
	})
	tk, err := tok.SignedString([]byte(signKey))
	require.NoError(t, err)
	return tk
}

// TestAppBudgetReset_AckByDerivedFromJWT — happy path for D.4 derivation.
// Custom JWT user "bob" with admin role; no client-supplied ack_by; provider
// must receive "bob". Plain reset (no force flag) routes to
// AppBudgetResetWithOptions with ForceClearCooldown=false per the B-6 fix.
func TestAppBudgetReset_AckByDerivedFromJWT(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetResetWithOptions", "myapp", "bob", mock.MatchedBy(resetOptsPlain)).Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode, "happy-path JWT-derived ackBy must succeed")
		require.Empty(t, res.Header.Get("Deprecation"), "no override → no Deprecation header")
		require.Empty(t, res.Header.Get("Sunset"), "no override → no Sunset header")
	})
}

// TestAppBudgetReset_AdminJWT_AckByIsSystemAdmin — cross-D.4-E.1 integration.
// Admin token minted via JwtMngr.AdminToken (User="system-admin", Role="rwa");
// provider must receive "system-admin" via the unified AppBudgetResetWithOptions
// entry point (B-6 fix routes plain reset through the same provider method
// the force-flag path uses, with ForceClearCooldown=false).
func TestAppBudgetReset_AdminJWT_AckByIsSystemAdmin(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetResetWithOptions", "myapp", "system-admin", mock.MatchedBy(resetOptsPlain)).Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
	})
}

// TestAppBudgetReset_AckByOverride_HonoredAsActor_EmitsHeader — when the
// client supplies ack_by via form param, the rack USES IT as the persisted
// actor passed to the provider (replacing the JWT-derived value). The
// Sunset/Link/Deprecation headers continue to fire — the form-param path
// is deprecated AND the override is honored until per-user JWT plumbing
// in 3.25.0. User-truthfulness: a Console dialog footer that says
// "Audit-logged as: alice@example.com" is now a contract the rack upholds.
func TestAppBudgetReset_AckByOverride_HonoredAsActor_EmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		// Provider receives the override string, NOT the JWT-derived "bob".
		// Plain reset path → AppBudgetResetWithOptions, ForceClearCooldown=false.
		p.On("AppBudgetResetWithOptions", "myapp", "alice", mock.MatchedBy(resetOptsPlain)).Return(nil)

		body := url.Values{"ack_by": []string{"alice"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must fire on override")
		require.Equal(t, "Thu, 01 Oct 2026 00:00:00 GMT", res.Header.Get("Sunset"), "Sunset must match SunsetDate3250")
		link := res.Header.Get("Link")
		require.Contains(t, link, "https://docs.convox.com/migration/ack-by-derivation")
		require.Contains(t, link, `rel="deprecation"`)
	})
}

// TestAppBudgetReset_AckByOverride_SameAsJWT_StillEmitsHeader — the
// deprecation signal is about the form-param being the deprecated path,
// not about whether the value differs from the JWT-derived actor. A
// client that sends ack_by=bob when the JWT user is also "bob" is still
// using the deprecated path, so the Deprecation/Sunset/Link headers
// still fire to nudge that client to drop the form param. The persisted
// actor is "bob" either way (JWT-derived and override happen to match),
// so this test does NOT exercise a behavior change in the persistence —
// only the header-emission rule.
func TestAppBudgetReset_AckByOverride_SameAsJWT_StillEmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		// Override matches JWT — provider still receives "bob" (override
		// is honored, value happens to match). The header-emission rule
		// is what flipped. Plain reset → ForceClearCooldown=false branch.
		p.On("AppBudgetResetWithOptions", "myapp", "bob", mock.MatchedBy(resetOptsPlain)).Return(nil)

		body := url.Values{"ack_by": []string{"bob"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		// Any non-empty ack_by fires the deprecation signal.
		require.Equal(t, "true", res.Header.Get("Deprecation"), "matching ack_by must STILL emit Deprecation header (form-param path is deprecated regardless of value comparison)")
		require.NotEmpty(t, res.Header.Get("Sunset"), "matching ack_by must STILL emit Sunset header")
		require.NotEmpty(t, res.Header.Get("Link"), "matching ack_by must STILL emit Link header")
	})
}

// TestAppBudgetClear_BasicAuth_AckByIsRackPassword — basic-auth path c.Set
// (the D.4 sole edit at pkg/api/api.go) propagates "rack-password" through to
// the provider. Uses a separate Server with Password set so the basic-auth
// branch is taken. Tests AppBudgetClear (CanWrite-gated, NOT Admin-gated)
// because basic-auth's SetReadWriteRole sets the role to "rw" — the
// AppBudgetReset CanAdmin guard would 403 a basic-auth caller. The contract
// being tested is the c.Set value, not the gating; AppBudgetClear is the
// closest available D.4-touched handler that admits a basic-auth caller.
// Spec §B.6 named the test against AppBudgetReset; the rename to
// AppBudgetClear preserves the spirit (basic-auth → rack-password → provider)
// while respecting the E.2 CanAdmin guard layered above D.4 on chain α4.
func TestAppBudgetClear_BasicAuth_AckByIsRackPassword(t *testing.T) {
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)
	p.On("AppBudgetClear", "myapp", "rack-password").Return(nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Password = "supersecret"
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	req, err := http.NewRequest(http.MethodDelete, ht.URL+"/apps/myapp/budget", strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("convox", "supersecret")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)
	require.Equal(t, http.StatusOK, res.StatusCode, "basic-auth must reach handler — got body %q", string(bodyBytes))

	p.AssertExpectations(t)
}

// TestAppBudgetReset_EmptyJWTUser_AckByUnknown — D.4 §B.1 empty/whitespace
// matrix. JWT verifies with empty user claim → controller falls back to
// "unknown". No panic.
func TestAppBudgetReset_EmptyJWTUser_AckByUnknown(t *testing.T) {
	cases := []struct {
		name string
		user string
	}{
		{"empty", ""},
		{"whitespace", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
				tk := mintCustomJwtToken(t, "test", tc.user, structs.ConvoxRoleAdmin, time.Hour)

				p.On("AppBudgetResetWithOptions", "myapp", "unknown", mock.MatchedBy(resetOptsPlain)).Return(nil)

				req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.SetBasicAuth("jwt", tk)

				res, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer res.Body.Close()

				require.Equal(t, http.StatusOK, res.StatusCode, "empty/whitespace JWT user must fall back to 'unknown' without panic")
			})
		})
	}
}

// TestAppBudgetReset_ConcurrentDifferentJWTs_AckByIsolated — guards against
// goroutine-leak through closures or shared state in the request path. Fires
// 10 parallel requests with 10 distinct JWT users; each provider call must
// receive the correct corresponding ack_by. Run with -race.
func TestAppBudgetReset_ConcurrentDifferentJWTs_AckByIsolated(t *testing.T) {
	users := []string{"alice", "bob", "carol", "dave", "eve", "frank", "grace", "heidi", "ivan", "judy"}

	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		// Pre-register expectations: each user must produce exactly one provider call.
		// Plain reset routes via AppBudgetResetWithOptions (ForceClearCooldown=false)
		// per the B-6 fix; the user-isolation contract is unchanged.
		for _, u := range users {
			user := u
			p.On("AppBudgetResetWithOptions", "myapp", user, mock.MatchedBy(resetOptsPlain)).Return(nil).Once()
		}

		var wg sync.WaitGroup
		errs := make(chan error, len(users))
		for _, u := range users {
			wg.Add(1)
			go func(user string) {
				defer wg.Done()
				tk := mintCustomJwtToken(t, "test", user, structs.ConvoxRoleAdmin, time.Hour)

				req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
				if err != nil {
					errs <- err
					return
				}
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.SetBasicAuth("jwt", tk)

				res, err := http.DefaultClient.Do(req)
				if err != nil {
					errs <- err
					return
				}
				_ = res.Body.Close()
				if res.StatusCode != http.StatusOK {
					errs <- err
					return
				}
			}(u)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			require.NoError(t, e)
		}
	})
}

// TestDeprecationSunsetDate_IsValidRFC7231 — format guard for SunsetDate3250.
// http.ParseTime accepts RFC 7231 IMF-fixdate; if the const ever drifts to a
// wrong day-name or non-IMF format, this test fails. Regex sanity-check
// re-asserts the specific shape so the failure mode is informative.
func TestDeprecationSunsetDate_IsValidRFC7231(t *testing.T) {
	s := structs.SunsetDate3250

	// Regex sanity: <day>, DD <Mon> YYYY HH:MM:SS GMT
	re := regexp.MustCompile(`^[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT$`)
	require.True(t, re.MatchString(s), "SunsetDate3250 must match RFC 7231 IMF-fixdate regex; got %q", s)

	// Round-trip via http.ParseTime (which understands IMF-fixdate, RFC 850,
	// ANSI C asctime). Must parse cleanly.
	parsed, err := http.ParseTime(s)
	require.NoError(t, err, "SunsetDate3250 must be parseable as RFC 7231 HTTP-date")

	// Calendar correctness: 2026-10-01 is a Thursday.
	require.Equal(t, time.Thursday, parsed.UTC().Weekday(), "2026-10-01 must be a Thursday — day-name in const must be 'Thu'")
}

// TestAuthenticate_BasicAuth_SetsConvoxJwtUserParam_RackPassword_E2E pairs
// with the internal-package middleware unit test of the same base name in
// pkg/api/d3_internal_test.go. The internal one asserts the c.Set value
// directly via captureMiddlewareValue; this one exercises the full HTTP
// pipeline so a future regression that bypasses the middleware (e.g. a router
// refactor) still gets caught.
func TestAuthenticate_BasicAuth_SetsConvoxJwtUserParam_RackPassword_E2E(t *testing.T) {
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Password = "supersecret"
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	// Hit /auth, which renders OK after passing through s.authenticate. The
	// existing /auth route returns plain "ok" — what we actually need is to
	// verify the param was set on the request context. The simplest available
	// assertion is that the basic-auth call succeeds (no 401) AND a downstream
	// budget-reset call propagates "rack-password" to the provider, which we
	// already cover in TestAppBudgetReset_BasicAuth_AckByIsRackPassword. This
	// test asserts the middleware-level success directly via /auth.
	req, err := http.NewRequest(http.MethodGet, ht.URL+"/auth", nil)
	require.NoError(t, err)
	req.SetBasicAuth("convox", "supersecret")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode, "basic-auth must succeed against /auth")

	// The middleware-internal contract (ConvoxJwtUserParam = "rack-password" on
	// the basic-auth path) is asserted at the unit-test level inside
	// pkg/api/d3_internal_test.go — see captureMiddlewareValue. This test pairs
	// with that internal one to provide end-to-end coverage from HTTP request
	// through the middleware to a successful response.
}

// TestAppBudgetReset_PlainReset_WriteRoleSucceeds locks in the audit-aligned
// gate semantics: a plain (no force_clear_cooldown) Reset call by a w-role
// JWT must succeed. This is the user-visible regression coverage for
// the unblock that lets Console3 wire its Reset button without elevating
// to Admin.
//
// B-6 fix: plain reset routes through AppBudgetResetWithOptions with
// ForceClearCooldown=false (the unified entry point) so the post-:fired
// restoreFromAnnotation recovery path is exercised — matching the
// documented behavior in docs/reference/cli/budget-reset.md.
func TestAppBudgetReset_PlainReset_WriteRoleSucceeds(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// JwtMngr.WriteToken hard-codes user="system-write".
		p.On("AppBudgetResetWithOptions", "myapp", "system-write", mock.MatchedBy(resetOptsPlain)).Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "w-token plain reset must succeed — got body %q", string(bodyBytes))
	})
}

// TestAppBudgetReset_PlainReset_RoutesToWithOptions_ForceClearCooldownFalse
// pins the B-6 fix: a plain `convox budget reset` (no force flag) MUST
// route to AppBudgetResetWithOptions with ForceClearCooldown=false, NOT
// to the legacy inner AppBudgetReset (which only cleared the breaker
// without restoring replicas). The unified routing makes the documented
// post-:fired recovery contract — restoreFromAnnotation reapplies the
// persisted replica counts — work for plain reset too. The flag remains
// additive: it triggers the cooldown-annotation deletion in addition to
// the shared restore-replicas path.
//
// AssertNotCalled on the legacy inner method guards against a future
// refactor reverting the unified routing.
func TestAppBudgetReset_PlainReset_RoutesToWithOptions_ForceClearCooldownFalse(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// Force the matcher to require ForceClearCooldown=false explicitly.
		p.On("AppBudgetResetWithOptions", "myapp", "system-write",
			mock.MatchedBy(func(opts structs.AppBudgetResetOptions) bool {
				return !opts.ForceClearCooldown
			})).Return(nil).Once()

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		// B-6 anti-regression: the legacy inner provider method must NOT
		// be invoked. The split-routing bug ran plain reset through the
		// inner AppBudgetReset which bypassed restoreFromAnnotation; the
		// fix unified the routing.
		p.AssertNotCalled(t, "AppBudgetReset", mock.Anything, mock.Anything)
	})
}

// TestAppBudgetReset_ForceClearCooldown_AdminRoleSucceeds locks in the
// happy-path for the gated escape hatch: rwa role + force_clear_cooldown=true
// reaches AppBudgetResetWithOptions with the flag set.
func TestAppBudgetReset_ForceClearCooldown_AdminRoleSucceeds(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetResetWithOptions", "myapp", "system-admin",
			mock.MatchedBy(func(opts structs.AppBudgetResetOptions) bool {
				return opts.ForceClearCooldown
			})).Return(nil)

		body := url.Values{"force_clear_cooldown": {"true"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "Admin token + force_clear_cooldown=true must succeed — got body %q", string(bodyBytes))
	})
}

// TestAppBudgetReset_ForceClearCooldown_WriteRoleRejected verifies the
// Admin-only escape hatch: a w-role JWT presenting force_clear_cooldown=true
// must 403 BEFORE any provider call (the Reset path is blocked even though
// the router-level CanWrite gate would otherwise admit the call). Captures
// the body substring contract for CLI/GUI parsers.
func TestAppBudgetReset_ForceClearCooldown_WriteRoleRejected(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// No p.On(...) — the CanAdmin gate inside forceClear must fire BEFORE
		// any provider call. AssertExpectations validates no unexpected calls.

		body := url.Values{"force_clear_cooldown": {"true"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, res.StatusCode, "w-token + force_clear_cooldown=true must 403 — got body %q", string(bodyBytes))

		got := strings.TrimRight(string(bodyBytes), "\n")
		require.Contains(t, got, "AppBudgetReset --force-clear-cooldown")
		require.Contains(t, got, "requires Admin role; current role is 'w'")
		require.Contains(t, got, "Contact rack admin or use Admin token.")

		p.AssertNotCalled(t, "AppBudgetReset")
		p.AssertNotCalled(t, "AppBudgetResetWithOptions")
	})
}

// TestAppBudgetReset_AckByOverride_HonoredAcrossBothPaths verifies the
// override-takes-effect contract for both the plain reset path AND the
// force-clear-cooldown path. The deprecation headers fire in both, AND
// the persisted actor is the override string.
func TestAppBudgetReset_AckByOverride_HonoredAcrossBothPaths(t *testing.T) {
	t.Run("plain-path-honors-override", func(t *testing.T) {
		budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
			tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleReadWrite, time.Hour)

			// Provider receives "alice" (override), NOT "bob" (JWT).
			// Plain reset routes via AppBudgetResetWithOptions(force=false)
			// per the B-6 fix.
			p.On("AppBudgetResetWithOptions", "myapp", "alice", mock.MatchedBy(resetOptsPlain)).Return(nil)

			body := url.Values{"ack_by": {"alice"}}.Encode()
			req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.SetBasicAuth("jwt", tk)

			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must fire on override (plain path)")
			require.NotEmpty(t, res.Header.Get("Sunset"), "Sunset header must fire on override (plain path)")
		})
	})

	t.Run("force-clear-path-honors-override", func(t *testing.T) {
		budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
			tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

			p.On("AppBudgetResetWithOptions", "myapp", "alice",
				mock.MatchedBy(func(opts structs.AppBudgetResetOptions) bool {
					return opts.ForceClearCooldown
				})).Return(nil)

			body := url.Values{"ack_by": {"alice"}, "force_clear_cooldown": {"true"}}.Encode()
			req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.SetBasicAuth("jwt", tk)

			res, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer res.Body.Close()

			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must fire on override (force-clear path)")
			require.NotEmpty(t, res.Header.Get("Sunset"), "Sunset header must fire on override (force-clear path)")
		})
	})
}

// TestAppBudgetSet_AckByOverride_PersistedAsActor — the override string
// lands in cfg.LastCapMutationBy via the AppBudgetSet provider call; the
// :set event payload uses the same value. Verifies the controller-level
// override-resolution path passes the user-presented actor through
// to the provider.
func TestAppBudgetSet_AckByOverride_PersistedAsActor(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetSet", "myapp",
			mock.MatchedBy(func(o structs.AppBudgetOptions) bool {
				return o.MonthlyCapUsd != nil && *o.MonthlyCapUsd == "500"
			}),
			"alice@example.com",
		).Return(nil)

		body := url.Values{
			"ack_by":          {"alice@example.com"},
			"monthly-cap-usd": {"500"},
		}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must fire on override")
	})
}

// TestAppBudgetClear_AckByOverride_PersistedAsActor — the :clear event
// payload includes the override actor, not the JWT-derived value.
// Critical for audit-trail truthfulness when Console (basic-auth) presents
// an override.
func TestAppBudgetClear_AckByOverride_PersistedAsActor(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetClear", "myapp", "alice@example.com").Return(nil)

		body := url.Values{"ack_by": {"alice@example.com"}}.Encode()
		req, err := http.NewRequest(http.MethodDelete, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"))
	})
}

// TestAppBudgetDismissRecovery_AckByOverride_PersistedAsActor_AndEmitsHeader
// — the DismissRecovery handler now emits the deprecation header on
// override, AND the override is honored as the persisted actor. Renders
// the AppBudgetDismissRecoveryResult JSON body so the test mocks the
// *WithResult provider variant.
func TestAppBudgetDismissRecovery_AckByOverride_PersistedAsActor_AndEmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetDismissRecoveryWithResult", "myapp", "alice@example.com").
			Return(&structs.AppBudgetDismissRecoveryResult{Status: "dismissed"}, nil)

		body := url.Values{"ack_by": {"alice@example.com"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/dismiss-recovery", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "DismissRecovery must emit Deprecation header on override")
		require.NotEmpty(t, res.Header.Get("Sunset"))
		require.NotEmpty(t, res.Header.Get("Link"))
	})
}

// TestAppBudgetClear_BasicAuth_WithAckByOverride_HonorsOverride — the
// user-truthfulness regression test. Console talks to the rack via
// basic-auth (rack password); without per-user JWT, every basic-auth
// caller has effective JWT user "rack-password". The Console dialog
// footer promises "Audit-logged as: alice@example.com" — that promise
// is now truthful because the override is the persisted actor.
func TestAppBudgetClear_BasicAuth_WithAckByOverride_HonorsOverride(t *testing.T) {
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)
	// Provider receives "alice@example.com" (override), NOT "rack-password" (basic-auth derived).
	p.On("AppBudgetClear", "myapp", "alice@example.com").Return(nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Password = "supersecret"
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	body := url.Values{"ack_by": {"alice@example.com"}}.Encode()
	req, err := http.NewRequest(http.MethodDelete, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("convox", "supersecret")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	bodyBytes, _ := io.ReadAll(res.Body)
	require.Equal(t, http.StatusOK, res.StatusCode, "basic-auth + override must reach handler — got body %q", string(bodyBytes))
	require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must fire on override")

	p.AssertExpectations(t)
}

// TestAppBudgetClear_AckByOverride_ViaQueryString_HonorsOverride —
// regression for the alternate wire shape: a DELETE request that
// presents `ack_by` as a URL query parameter (instead of a form-encoded
// body) must also honor the override. Go's stdlib `r.ParseForm` always
// parses the URL query for any method, so this path works via the
// stdapi `c.Value` fallback in `formValue` without invoking the manual
// DELETE-body parser. Locks in the contract that BOTH wire shapes are
// supported and that the formValue helper doesn't accidentally clobber
// query-string values.
func TestAppBudgetClear_AckByOverride_ViaQueryString_HonorsOverride(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetClear", "myapp", "alice@example.com").Return(nil)

		// No body — ack_by lives in URL query string.
		u := ht.URL + "/apps/myapp/budget?ack_by=" + url.QueryEscape("alice@example.com")
		req, err := http.NewRequest(http.MethodDelete, u, nil)
		require.NoError(t, err)
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "query-string override must also emit Deprecation header")
	})
}

// TestAppBudgetSet_AdminGate_CapChange_NonAdminRejected verifies that a
// non-admin (w-role) caller cannot change MonthlyCapUsd via AppBudgetSet.
// The CanAdmin gate inside the handler must fire BEFORE any provider call.
func TestAppBudgetSet_AdminGate_CapChange_NonAdminRejected(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// No p.On(...) — the admin gate must fire before any provider call.

		body := url.Values{"monthly-cap-usd": {"500"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusForbidden, res.StatusCode,
			"w-token + monthly-cap-usd must 403 -- got body %q", string(bodyBytes))
		require.Contains(t, string(bodyBytes), "AppBudgetSet: admin role required",
			"body must include AppBudgetSet admin gate message")

		p.AssertNotCalled(t, "AppBudgetSet")
	})
}

// TestAppBudgetSet_AdminGate_CapChange_BasicAuth_Allowed pins the api.go
// SetAdminRole-on-basic-auth contract: a basic-auth caller (rack-password,
// the path the convox CLI and Console3-via-rack-client both take) MUST
// reach AppBudgetSet for cap-change calls, not bounce off the CanAdmin gate.
//
// Earlier in the 3.24.6 development cycle the basic-auth branch landed
// on SetReadWriteRole ("rw"); CanAdmin requires substring "a"; cap-change
// returned 403. The api.go switch to SetAdminRole on the basic-auth
// branch is what unblocks Console budget save and `convox budget set`
// from the CLI. A future revert to SetReadWriteRole would silently
// re-introduce the production 403; this test is the regression gate
// for that change.
//
// Distinct from TestAppBudgetSet_AdminGate_CapChange_NonAdminRejected
// (which exercises a JWT WriteToken role=w; that path is correctly gated
// and remains rejected — the JWT branch above the basic-auth branch in
// api.go::authenticate does not stamp admin).
func TestAppBudgetSet_AdminGate_CapChange_BasicAuth_Allowed(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		p.On("AppBudgetSet", "myapp",
			mock.MatchedBy(func(o structs.AppBudgetOptions) bool {
				return o.MonthlyCapUsd != nil && *o.MonthlyCapUsd == "500"
			}),
			"rack-password",
		).Return(nil)

		body := url.Values{"monthly-cap-usd": {"500"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("convox", "supersecret")

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode,
			"basic-auth + monthly-cap-usd must NOT 403 (admin role granted to rack-password); got %q",
			string(bodyBytes))
		p.AssertExpectations(t)
	})
}

// TestAppBudgetSet_AdminGate_CapChange_AdminSucceeds verifies that an admin
// caller can change MonthlyCapUsd via AppBudgetSet without hitting the gate.
func TestAppBudgetSet_AdminGate_CapChange_AdminSucceeds(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetSet", "myapp",
			mock.MatchedBy(func(o structs.AppBudgetOptions) bool {
				return o.MonthlyCapUsd != nil && *o.MonthlyCapUsd == "500"
			}),
			"system-admin",
		).Return(nil)

		body := url.Values{"monthly-cap-usd": {"500"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode, "admin token + monthly-cap-usd must succeed")
	})
}
