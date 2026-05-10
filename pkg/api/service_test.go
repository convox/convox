package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	cjwt "github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/require"
)

var fxService = structs.Service{
	Name:      "service1",
	Count:     1,
	Cpu:       2,
	Domain:    "domain",
	Gpu:       0,
	GpuVendor: "",
	Memory:    3,
	Ports: []structs.ServicePort{
		{Balancer: 1, Certificate: "cert1", Container: 2},
		{Balancer: 1, Certificate: "cert1", Container: 2},
	},
}

func TestServiceList(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		s1 := structs.Services{fxService, fxService}
		s2 := structs.Services{}
		p.On("ServiceList", "app1").Return(s1, nil)
		err := c.Get("/apps/app1/services", stdsdk.RequestOptions{}, &s2)
		require.NoError(t, err)
		require.Equal(t, s1, s2)
	})
}

func TestServiceListError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var s1 structs.Services
		p.On("ServiceList", "app1").Return(nil, fmt.Errorf("err1"))
		err := c.Get("/apps/app1/services", stdsdk.RequestOptions{}, &s1)
		require.EqualError(t, err, "err1")
		require.Nil(t, s1)
	})
}

func TestServiceUpdate(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.ServiceUpdateOptions{
			Count:  options.Int(1),
			Cpu:    options.Int(2),
			Memory: options.Int(3),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"count":  "1",
				"cpu":    "2",
				"memory": "3",
			},
		}
		p.On("ServiceUpdate", "app1", "service1", opts).Return(nil)
		err := c.Put("/apps/app1/services/service1", ro, nil)
		require.NoError(t, err)
	})
}

func TestServiceUpdateError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("ServiceUpdate", "app1", "service1", structs.ServiceUpdateOptions{}).Return(fmt.Errorf("err1"))
		err := c.Put("/apps/app1/services/service1", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}

func TestServiceUpdateGpu(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.ServiceUpdateOptions{
			Count:     options.Int(1),
			Gpu:       options.Int(2),
			GpuVendor: options.String("nvidia"),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"count":      "1",
				"gpu":        "2",
				"gpu-vendor": "nvidia",
			},
		}
		p.On("ServiceUpdate", "app1", "service1", opts).Return(nil)
		err := c.Put("/apps/app1/services/service1", ro, nil)
		require.NoError(t, err)
	})
}

// ----- Item-23 §4.3 ServiceScaleOverrideSet — API controller tests -----

// TestServiceScaleOverrideSet_HappyPath — admin token + active=true, expect 200.
func TestServiceScaleOverrideSet_HappyPath(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("ServiceScaleOverrideSet", "myapp", "web", true, "system-admin").Return(nil)

		body := url.Values{"active": {"true"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "happy-path admin call must succeed — got %q", string(bodyBytes))
	})
}

// TestServiceScaleOverrideSet_MissingActiveParam — 400 when active form-param absent.
func TestServiceScaleOverrideSet_MissingActiveParam(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		// no p.On(...) — the controller must reject before provider call.

		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusBadRequest, res.StatusCode, "missing active param must yield 400")
		p.AssertNotCalled(t, "ServiceScaleOverrideSet")
	})
}

// TestServiceScaleOverrideSet_BadActiveParam — 400 on unparseable bool.
func TestServiceScaleOverrideSet_BadActiveParam(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		body := url.Values{"active": {"maybe"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusBadRequest, res.StatusCode, "active=maybe must yield 400")
		p.AssertNotCalled(t, "ServiceScaleOverrideSet")
	})
}

// TestServiceScaleOverrideSet_AckByForwarded — provider call receives the
// override ack_by string (mirrors AppBudgetSet ack_by-precedence test).
func TestServiceScaleOverrideSet_AckByForwarded(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, _ *cjwt.JwtManager) {
		tk := mintCustomJwtToken(t, "test", "bob", structs.ConvoxRoleAdmin, time.Hour)

		p.On("ServiceScaleOverrideSet", "myapp", "web", true, "alice@example.com").Return(nil)

		body := url.Values{"active": {"true"}, "ack_by": {"alice@example.com"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "true", res.Header.Get("Deprecation"), "ack_by override must emit Deprecation header (form-param path is deprecated)")
	})
}

// TestServiceScaleOverrideSet_RBAC_ReadWriteAllowed verifies that the
// rw-role succeeds (no admin escalation required). Scale override
// gates on the per-app Write permission, mirroring all other
// operational service controls (deploy, rollback, env edit). Console
// enforces the per-app Write check at the mutation layer; this
// rack-side gate matches.
func TestServiceScaleOverrideSet_RBAC_ReadWriteAllowed(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.WriteToken(time.Hour)
		require.NoError(t, err)

		// JwtMngr.WriteToken hard-codes user="system-write".
		p.On("ServiceScaleOverrideSet", "myapp", "web", true, "system-write").Return(nil)

		body := url.Values{"active": {"true"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "rw-token must succeed — got body %q", string(bodyBytes))
	})
}

// TestServiceScaleOverrideSet_RBAC_ReadOnlyForbidden verifies that
// the r-role gets 403 before any provider call. Read-only callers
// cannot mutate the override annotation. The Authorize middleware
// would already 401 a GET-only token on a POST, but the explicit
// gate produces a clearer endpoint-named error and acts as
// defense-in-depth if the middleware is ever skipped.
func TestServiceScaleOverrideSet_RBAC_ReadOnlyForbidden(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.ReadToken(time.Hour)
		require.NoError(t, err)

		// No p.On(...) — the gate must fire BEFORE any provider call.

		body := url.Values{"active": {"true"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		// Either the Authorize middleware (401 "you are unauthorized") or the
		// explicit endpoint gate (403 "requires Read+Write role") must reject;
		// in practice the middleware fires first for a pure-r token. Accept
		// either status to keep the test robust to middleware ordering changes
		// while still proving r-role cannot mutate.
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, res.StatusCode,
			"r-token must be denied — got %d body %q", res.StatusCode, string(bodyBytes))

		p.AssertNotCalled(t, "ServiceScaleOverrideSet")
	})
}

// TestServiceScaleOverrideSet_RBAC_AdminAllowed — rwa-role must continue to
// succeed. The HappyPath test above already covers the admin token via
// JwtMngr.AdminToken (User="system-admin", Role="rwa"); this test names the
// behavior explicitly so the rwa→200 contract is locked under its own test
// rather than coincidentally inheriting from a non-RBAC happy-path test.
func TestServiceScaleOverrideSet_RBAC_AdminAllowed(t *testing.T) {
	budgetTestServer(t, func(ht *httptest.Server, p *structs.MockProvider, jm *cjwt.JwtManager) {
		tk, err := jm.AdminToken(time.Hour)
		require.NoError(t, err)

		p.On("ServiceScaleOverrideSet", "myapp", "web", false, "system-admin").Return(nil)

		body := url.Values{"active": {"false"}}.Encode()
		req, err := http.NewRequest(http.MethodPost, ht.URL+"/apps/myapp/services/web/scale-override", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("jwt", tk)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		bodyBytes, _ := io.ReadAll(res.Body)
		require.Equal(t, http.StatusOK, res.StatusCode, "rwa-token must continue to succeed — got body %q", string(bodyBytes))
	})
}
