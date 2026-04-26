package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/convox/convox/pkg/api"
	cjwt "github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var fxSystem = structs.System{
	Count:      1,
	Domain:     "domain",
	Name:       "name",
	Outputs:    map[string]string{"k1": "v1", "k2": "v2"},
	Parameters: map[string]string{"k1": "v1", "k2": "v2"},
	Provider:   "provider",
	Region:     "region",
	Status:     "status",
	Type:       "type",
	Version:    "version",
}

var fxMetric = structs.Metric{
	Name: "metric1",
	Values: structs.MetricValues{
		{
			Time:    time.Date(2018, 9, 1, 0, 0, 0, 0, time.UTC),
			Average: 2.0,
			Minimum: 1.0,
			Maximum: 3.0,
		},
		{
			Time:    time.Date(2018, 9, 1, 1, 0, 0, 0, time.UTC),
			Average: 2.0,
			Minimum: 1.0,
			Maximum: 3.0,
		},
	},
}

func TestSystemGet(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		s1 := fxSystem
		s2 := structs.System{}
		p.On("SystemGet").Return(&s1, nil)
		err := c.Get("/system", stdsdk.RequestOptions{}, &s2)
		require.NoError(t, err)
		require.Equal(t, s1, s2)
	})
}

func TestSystemGetError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var s1 *structs.System
		p.On("SystemGet").Return(nil, fmt.Errorf("err1"))
		err := c.Get("/system", stdsdk.RequestOptions{}, s1)
		require.EqualError(t, err, "err1")
		require.Nil(t, s1)
	})
}

func TestSystemLogs(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		d1 := []byte("test")
		r1 := io.NopCloser(bytes.NewReader(d1))
		opts := structs.LogsOptions{Since: options.Duration(2 * time.Minute)}
		p.On("SystemLogs", opts).Return(r1, nil)
		r2, err := c.Websocket("/system/logs", stdsdk.RequestOptions{})
		require.NoError(t, err)
		d2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Equal(t, d1, d2)
	})
}

func TestSystemLogsError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.LogsOptions{Since: options.Duration(2 * time.Minute)}
		p.On("SystemLogs", opts).Return(nil, fmt.Errorf("err1"))
		r1, err := c.Websocket("/system/logs", stdsdk.RequestOptions{})
		require.NoError(t, err)
		require.NotNil(t, r1)
		d1, err := io.ReadAll(r1)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: err1\n"), d1)
	})
}

func TestSystemMetrics(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		m1 := structs.Metrics{fxMetric, fxMetric}
		m2 := structs.Metrics{}
		opts := structs.MetricsOptions{
			End:     options.Time(time.Date(2018, 10, 1, 3, 4, 5, 0, time.UTC)),
			Metrics: []string{"foo", "bar"},
			Period:  options.Int64(300),
			Start:   options.Time(time.Date(2018, 9, 1, 2, 3, 4, 0, time.UTC)),
		}
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"end":     "20181001.030405.000000000",
				"metrics": "foo,bar",
				"period":  "300",
				"start":   "20180901.020304.000000000",
			},
		}
		p.On("SystemMetrics", opts).Return(m1, nil)
		err := c.Get("/system/metrics", ro, &m2)
		require.NoError(t, err)
		require.Equal(t, m1, m2)
	})
}

func TestSystemMetricsError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var m1 structs.Metrics
		p.On("SystemMetrics", structs.MetricsOptions{}).Return(nil, fmt.Errorf("err1"))
		err := c.Get("/system/metrics", stdsdk.RequestOptions{}, &m1)
		require.EqualError(t, err, "err1")
		require.Nil(t, m1)
	})
}

func TestSystemProcesses(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p1 := structs.Processes{fxProcess, fxProcess}
		p2 := structs.Processes{}
		opts := structs.SystemProcessesOptions{
			All: options.Bool(true),
		}
		ro := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"all": "true",
			},
		}
		p.On("SystemProcesses", opts).Return(p1, nil)
		err := c.Get("/system/processes", ro, &p2)
		require.NoError(t, err)
		require.Equal(t, p1, p2)
	})
}

func TestSystemProcessesError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var p1 structs.Processes
		p.On("SystemProcesses", structs.SystemProcessesOptions{}).Return(nil, fmt.Errorf("erp1"))
		err := c.Get("/system/processes", stdsdk.RequestOptions{}, &p1)
		require.EqualError(t, err, "erp1")
		require.Nil(t, p1)
	})
}

func TestSystemReleases(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r1 := structs.Releases{fxRelease, fxRelease}
		r2 := structs.Releases{}
		p.On("SystemReleases").Return(r1, nil)
		err := c.Get("/system/releases", stdsdk.RequestOptions{}, &r2)
		require.NoError(t, err)
		require.Equal(t, r1, r2)
	})
}

func TestSystemReleasesError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var r1 structs.Releases
		p.On("SystemReleases").Return(nil, fmt.Errorf("err1"))
		err := c.Get("/system/releases", stdsdk.RequestOptions{}, &r1)
		require.EqualError(t, err, "err1")
		require.Nil(t, r1)
	})
}

func TestSystemUpdate(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.SystemUpdateOptions{
			Count:      options.Int(1),
			Parameters: map[string]string{"k1": "v1", "k2": "v2"},
			Type:       options.String("type"),
			Version:    options.String("version"),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"count":      "1",
				"parameters": "k1=v1&k2=v2",
				"type":       "type",
				"version":    "version",
			},
		}
		p.On("SystemUpdate", opts).Return(nil)
		err := c.Put("/system", ro, nil)
		require.NoError(t, err)
	})
}

func TestSystemUpdateError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("SystemUpdate", structs.SystemUpdateOptions{}).Return(fmt.Errorf("err1"))
		err := c.Put("/system", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}

// TestSystemJwtToken_UnknownRole_Returns400 sets up an httptest server with
// MockProvider, sends POST /system/jwt/token with an unknown role, and
// expects HTTP 400 with body containing the substring "invalid role". Locks
// the new default clause in source.
func TestSystemJwtToken_UnknownRole_Returns400(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		// stdsdk doesn't surface the response status code easily, so go through
		// the underlying http.Client directly.
		form := url.Values{}
		form.Set("role", "bogus")
		form.Set("durationInHour", "1")
		req, err := http.NewRequest(http.MethodPost, c.Endpoint.String()+"/system/jwt/token", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusBadRequest, res.StatusCode)

		bodyBytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Contains(t, string(bodyBytes), "invalid role")
	})
}

// TestSystemJwtToken_GeneratesRwaRole_VerifiesOnRack POSTs /system/jwt/token
// with role=admin and durationInHour=1; asserts HTTP 200; parses response
// body as structs.SystemJwt; verifies the returned token via a JwtManager
// matching the test signing key — Role should be "rwa", User should be
// "system-admin".
func TestSystemJwtToken_GeneratesRwaRole_VerifiesOnRack(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		form := url.Values{}
		form.Set("role", "admin")
		form.Set("durationInHour", "1")
		req, err := http.NewRequest(http.MethodPost, c.Endpoint.String()+"/system/jwt/token", strings.NewReader(form.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)

		var jwtRes structs.SystemJwt
		decErr := json.NewDecoder(res.Body).Decode(&jwtRes)
		require.NoError(t, decErr)
		require.NotEmpty(t, jwtRes.Token)

		// Verify the token via a JwtManager with matching signing key.
		jm := cjwt.NewJwtManager("test")
		data, err := jm.Verify(jwtRes.Token)
		require.NoError(t, err)
		require.Equal(t, "rwa", data.Role)
		require.Equal(t, "system-admin", data.User)
	})
}

// jwtAuthTestServer spins up a full api.Server (including the s.authenticate
// middleware) with a MockProvider so that JWT-based Basic Auth is exercised
// end-to-end. testServer (the existing helper) doesn't go through s.authenticate
// for the default route case because stdsdk.Client doesn't set Basic Auth by
// default; this helper is needed for any test that wants to present a JWT.
func jwtAuthTestServer(t *testing.T, fn func(*httptest.Server, *structs.MockProvider, *cjwt.JwtManager)) {
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

// TestSystemJwtToken_AdminTokenWorksOnWriteEndpoint mints an Admin token
// directly via JwtMngr.AdminToken and presents it as Basic Auth (username
// "jwt") to a CanWrite-gated (non-Admin) endpoint, AppBudgetSet. Asserts
// HTTP 200 — Admin token successfully writes through CanWrite. Catches a
// regression where a future PR accidentally narrows CanWrite to exclude
// Admin tokens (e.g. by replacing strings.Contains(v, "w") with v == "rw").
func TestSystemJwtToken_AdminTokenWorksOnWriteEndpoint(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetSet", "myapp", structs.AppBudgetOptions{}, "system-admin").Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "Admin token must satisfy CanWrite — got body %q", string(bodyBytes))
	})
}

// TestAppBudgetReset_RoleMatrix is the table-driven r/w/Admin matrix test
// that consolidates the customer-visible matrix into one place. Failure on
// any row indicates a substring-match regression.
//
// E.1 / E.2 sequencing note: matrix is complete as of E.2 — write-token-403
// row is fully active (was t.Skip-gated during E.1 standalone window).
func TestAppBudgetReset_RoleMatrix(t *testing.T) {
	tests := []struct {
		name           string
		mintToken      func(jm *cjwt.JwtManager) (string, error)
		expectedStatus int
		providerCalled bool
	}{
		{
			name: "read-token-401",
			mintToken: func(jm *cjwt.JwtManager) (string, error) {
				return jm.ReadToken(time.Hour)
			},
			expectedStatus: http.StatusUnauthorized,
			providerCalled: false,
		},
		{
			name: "write-token-403",
			mintToken: func(jm *cjwt.JwtManager) (string, error) {
				return jm.WriteToken(time.Hour)
			},
			expectedStatus: http.StatusForbidden,
			providerCalled: false,
		},
		{
			name: "admin-token-200",
			mintToken: func(jm *cjwt.JwtManager) (string, error) {
				return jm.AdminToken(time.Hour)
			},
			expectedStatus: http.StatusOK,
			providerCalled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
				tk, err := tc.mintToken(jm)
				require.NoError(t, err)

				if tc.providerCalled {
					p.On("AppBudgetReset", "myapp", "system-admin").Return(nil)
				}

				req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.SetBasicAuth("jwt", tk)

				res, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer res.Body.Close()
				bodyBytes, _ := io.ReadAll(res.Body)
				require.Equal(t, tc.expectedStatus, res.StatusCode, "row=%s body=%q", tc.name, string(bodyBytes))
			})
		})
	}
}

// TestAppBudgetReset_RequiresAdminRole_403sOnWriteRole asserts that a w-role
// JWT token presented to AppBudgetReset returns HTTP 403 (CanAdmin guard
// fired) AND the mock provider's AppBudgetReset is NEVER called (the guard
// short-circuits before the handler reaches the provider). Complements the
// matrix test by adding the explicit "provider not called" assertion.
func TestAppBudgetReset_RequiresAdminRole_403sOnWriteRole(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// Intentionally do NOT register p.On("AppBudgetReset", ...). If the guard
		// fails to fire, mock will record an unexpected call and AssertExpectations
		// will fail.

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader("ack_by=alice"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusForbidden, res.StatusCode, "w-token must 403 on AppBudgetReset — got body %q", string(bodyBytes))
		p.AssertNotCalled(t, "AppBudgetReset")
	})
}

// TestAppBudgetReset_AcceptsAdminRole_NoGuardBlock asserts that an Admin-role
// JWT token presented to AppBudgetReset proceeds past the CanAdmin guard and
// reaches the provider call (HTTP 200; mock's AppBudgetReset called once).
// Complements the matrix admin-token-200 row by also exercising the ackBy
// pass-through in the same call.
func TestAppBudgetReset_AcceptsAdminRole_NoGuardBlock(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetReset", "myapp", "system-admin").Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader("ack_by=alice"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "Admin token must satisfy CanAdmin — got body %q", string(bodyBytes))
	})
}

// TestAppBudgetReset_403BodyMatchesPinnedString locks in the R3 amendments
// pinned 403 body. Customer tooling (CLI "convox budget reset" failure
// renderer; future Phase 1 GUI guidance modal) parses this body for
// migration guidance — wording, role identifier ('w'), and the "Use Admin
// token" clause are deterministic. Any future PR that changes the wording
// MUST update this test.
func TestAppBudgetReset_403BodyMatchesPinnedString(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget/reset", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		// stdapi.writeErrorResponse only emits JSON when the request advertises it.
		req.Header.Set("Accept", "application/json")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusForbidden, res.StatusCode)
		bodyBytes, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		// json.Encoder.Encode appends a trailing newline. The pinned R3 contract
		// is the JSON object body itself; trim the encoder newline before compare.
		got := strings.TrimRight(string(bodyBytes), "\n")
		want := "{\"error\":\"AppBudgetReset requires Admin role; current role is 'w'. Contact rack admin or use Admin token.\"}"
		require.Equal(t, want, got, "403 body must match R3-pinned string verbatim")
	})
}

// TestAppBudgetSet_StillAcceptsWriteRole_NoElevation asserts that AppBudgetSet
// remains CanWrite-gated and is NOT accidentally elevated to Admin. Guards
// against an over-eager future PR that "consistently" elevates all
// AppBudget* endpoints — the gating taxonomy (Set vs Reset) is intentional
// per spec A.0: Set/Clear are ordinary CRUD writes, Reset is the safety-gate
// bypass requiring Admin.
func TestAppBudgetSet_StillAcceptsWriteRole_NoElevation(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetSet", "myapp", structs.AppBudgetOptions{}, "system-write").Return(nil)

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/budget", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "AppBudgetSet must stay CanWrite-only — got body %q", string(bodyBytes))
	})
}

// TestAppBudgetClear_StillAcceptsWriteRole_NoElevation asserts that
// AppBudgetClear remains CanWrite-gated. Sibling regression to AppBudgetSet
// — guards the same gating-taxonomy invariant.
func TestAppBudgetClear_StillAcceptsWriteRole_NoElevation(t *testing.T) {
	jwtAuthTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		p.On("AppBudgetClear", "myapp", "system-write").Return(nil)

		req, err := http.NewRequest(http.MethodDelete, ht.URL+"/apps/myapp/budget", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "AppBudgetClear must stay CanWrite-only — got body %q", string(bodyBytes))
	})
}
