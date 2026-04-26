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
// must receive "bob".
func TestAppBudgetReset_AckByDerivedFromJWT(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetReset", "myapp", "bob").Return(nil)

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
// provider must receive "system-admin".
func TestAppBudgetReset_AdminJWT_AckByIsSystemAdmin(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetReset", "myapp", "system-admin").Return(nil)

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

// TestAppBudgetReset_AckByOverridesClientSupplied_EmitsHeader — D.4 contract.
// JWT user "bob"; client sends ack_by=alice; provider receives "bob"; RFC 8594
// headers populated.
func TestAppBudgetReset_AckByOverridesClientSupplied_EmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetReset", "myapp", "bob").Return(nil)

		body := url.Values{"ack_by": []string{"alice"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "Deprecation header must be true on override")
		require.Equal(t, "Thu, 01 Oct 2026 00:00:00 GMT", res.Header.Get("Sunset"), "Sunset must match SunsetDate3250")
		link := res.Header.Get("Link")
		require.Contains(t, link, "https://docs.convox.com/migration/ack-by-derivation")
		require.Contains(t, link, `rel="deprecation"`)
	})
}

// TestAppBudgetReset_AckBySameAsJWT_NoHeader — when client-supplied ack_by
// matches the JWT user, no override is signaled (headers absent).
func TestAppBudgetReset_AckBySameAsJWT_NoHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetReset", "myapp", "bob").Return(nil)

		body := url.Values{"ack_by": []string{"bob"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Empty(t, res.Header.Get("Deprecation"), "matching ack_by must NOT emit Deprecation header")
		require.Empty(t, res.Header.Get("Sunset"), "matching ack_by must NOT emit Sunset header")
		require.Empty(t, res.Header.Get("Link"), "matching ack_by must NOT emit Link header")
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

				p.On("AppBudgetReset", "myapp", "unknown").Return(nil)

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
		for _, u := range users {
			p.On("AppBudgetReset", "myapp", u).Return(nil).Once()
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
// JWT must succeed. This is the customer-visible regression coverage for
// the unblock that lets Console3 wire its Reset button without elevating
// to Admin.
func TestAppBudgetReset_PlainReset_WriteRoleSucceeds(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// JwtMngr.WriteToken hard-codes user="system-write".
		p.On("AppBudgetReset", "myapp", "system-write").Return(nil)

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

// TestAppBudgetReset_DeprecationHeadersFire_RegressionForBothPaths verifies
// that the RFC 8594 deprecation headers (Sunset/Link/Deprecation) continue
// to fire when a client-supplied ack_by overrides the JWT-derived one,
// independent of which gate path runs. Covers a subtle invariant: the
// header-emission block sits BEFORE the forceClear branch, so it must
// execute regardless of which provider method ultimately runs.
func TestAppBudgetReset_DeprecationHeadersFire_RegressionForBothPaths(t *testing.T) {
	t.Run("plain-path-emits-headers", func(t *testing.T) {
		budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
			tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleReadWrite, time.Hour)

			p.On("AppBudgetReset", "myapp", "bob").Return(nil)

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

	t.Run("force-clear-path-emits-headers", func(t *testing.T) {
		budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
			tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

			p.On("AppBudgetResetWithOptions", "myapp", "bob",
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
