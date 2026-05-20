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

// resetOptsPlain matches plain reset (no force-clear-cooldown flag).
func resetOptsPlain(opts structs.AppBudgetResetOptions) bool {
	return !opts.ForceClearCooldown
}

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

// TestAppBudgetReset_AckByDerivedFromJWT verifies the actor is derived from
// the JWT user claim when no ack_by override is sent.
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

// TestAppBudgetReset_AdminJWT_AckByIsSystemAdmin verifies admin token
// propagates "system-admin" as the actor.
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

// TestAppBudgetReset_AckByOverride_HonoredAsActor_EmitsHeader verifies ack_by
// form param overrides the JWT-derived actor and emits deprecation headers.
func TestAppBudgetReset_AckByOverride_HonoredAsActor_EmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

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

// TestAppBudgetReset_AckByOverride_SameAsJWT_StillEmitsHeader verifies
// deprecation headers fire even when ack_by matches the JWT user.
func TestAppBudgetReset_AckByOverride_SameAsJWT_StillEmitsHeader(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

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
		require.Equal(t, "true", res.Header.Get("Deprecation"), "matching ack_by must STILL emit Deprecation header (form-param path is deprecated regardless of value comparison)")
		require.NotEmpty(t, res.Header.Get("Sunset"), "matching ack_by must STILL emit Sunset header")
		require.NotEmpty(t, res.Header.Get("Link"), "matching ack_by must STILL emit Link header")
	})
}

// TestAppBudgetClear_BasicAuth_AckByIsRackPassword verifies basic-auth callers
// propagate "rack-password" as the actor to the provider.
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

// TestAppBudgetReset_EmptyJWTUser_AckByUnknown verifies empty/whitespace JWT
// user claims fall back to "unknown" without panic.
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

// TestAppBudgetReset_ConcurrentDifferentJWTs_AckByIsolated verifies per-request
// actor isolation under concurrent requests with distinct JWT users.
func TestAppBudgetReset_ConcurrentDifferentJWTs_AckByIsolated(t *testing.T) {
	users := []string{"alice", "bob", "carol", "dave", "eve", "frank", "grace", "heidi", "ivan", "judy"}

	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
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

// TestDeprecationSunsetDate_IsValidRFC7231 validates SunsetDate3250 parses
// as RFC 7231 IMF-fixdate with the correct weekday.
func TestDeprecationSunsetDate_IsValidRFC7231(t *testing.T) {
	s := structs.SunsetDate3250

	re := regexp.MustCompile(`^[A-Z][a-z]{2}, \d{2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} GMT$`)
	require.True(t, re.MatchString(s), "SunsetDate3250 must match RFC 7231 IMF-fixdate regex; got %q", s)

	parsed, err := http.ParseTime(s)
	require.NoError(t, err, "SunsetDate3250 must be parseable as RFC 7231 HTTP-date")

	require.Equal(t, time.Thursday, parsed.UTC().Weekday(), "2026-10-01 must be a Thursday — day-name in const must be 'Thu'")
}

// TestAuthenticate_BasicAuth_SetsConvoxJwtUserParam_RackPassword_E2E verifies
// basic-auth sets ConvoxJwtUserParam via the full HTTP pipeline.
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

	req, err := http.NewRequest(http.MethodGet, ht.URL+"/auth", nil)
	require.NoError(t, err)
	req.SetBasicAuth("convox", "supersecret")

	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode, "basic-auth must succeed against /auth")

}

// TestAppBudgetReset_PlainReset_WriteRoleSucceeds verifies a write-role JWT
// can perform a plain budget reset (no force-clear-cooldown).
func TestAppBudgetReset_PlainReset_WriteRoleSucceeds(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

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
// verifies plain reset routes through AppBudgetResetWithOptions (not the
// legacy AppBudgetReset) so replica restoration works for plain reset too.
func TestAppBudgetReset_PlainReset_RoutesToWithOptions_ForceClearCooldownFalse(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

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

		p.AssertNotCalled(t, "AppBudgetReset", mock.Anything, mock.Anything)
	})
}

// TestAppBudgetReset_ForceClearCooldown_AdminRoleSucceeds verifies admin
// callers can use force_clear_cooldown=true.
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

// TestAppBudgetReset_ForceClearCooldown_WriteRoleRejected verifies write-role
// callers are rejected (403) when using force_clear_cooldown=true.
func TestAppBudgetReset_ForceClearCooldown_WriteRoleRejected(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

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

// TestAppBudgetReset_AckByOverride_HonoredAcrossBothPaths verifies ack_by
// override works on both the plain and force-clear-cooldown reset paths.
func TestAppBudgetReset_AckByOverride_HonoredAcrossBothPaths(t *testing.T) {
	t.Run("plain-path-honors-override", func(t *testing.T) {
		budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
			tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleReadWrite, time.Hour)

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

// TestAppBudgetSet_AckByOverride_PersistedAsActor verifies ack_by override
// is passed as the actor to AppBudgetSet.
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

// TestAppBudgetClear_AckByOverride_PersistedAsActor verifies ack_by override
// is passed as the actor to AppBudgetClear.
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
// verifies ack_by override is honored and deprecation headers are emitted.
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

// TestAppBudgetClear_BasicAuth_WithAckByOverride_HonorsOverride verifies
// basic-auth callers can override the actor via ack_by form param.
func TestAppBudgetClear_BasicAuth_WithAckByOverride_HonorsOverride(t *testing.T) {
	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)
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

// TestAppBudgetClear_AckByOverride_ViaQueryString_HonorsOverride verifies
// ack_by override works via URL query string on DELETE requests.
func TestAppBudgetClear_AckByOverride_ViaQueryString_HonorsOverride(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("AppBudgetClear", "myapp", "alice@example.com").Return(nil)

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

// TestAppBudgetSet_AdminGate_CapChange_NonAdminRejected verifies write-role
// callers are rejected for cap changes.
func TestAppBudgetSet_AdminGate_CapChange_NonAdminRejected(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

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

// TestAppBudgetSet_AdminGate_CapChange_BasicAuth_Allowed verifies basic-auth
// callers (rack password) pass the admin gate for cap changes.
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
