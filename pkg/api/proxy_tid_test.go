package api_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"net/http/httptest"
)

type tenantProvider struct {
	*structs.MockProvider
}

func (p *tenantProvider) TenantNamespacePrefix(tid string) string {
	if tid == "" {
		return ""
	}
	return "rk-" + tid + "-"
}

func testTenantServer(t *testing.T, fn func(*stdsdk.Client, *structs.MockProvider)) {
	t.Helper()

	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(&tenantProvider{p})
	s.Logger = logger.Discard
	s.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	c, err := stdsdk.New(ht.URL)
	require.NoError(t, err)

	fn(c, p)

	p.AssertExpectations(t)
}

func tidHeaders(tid string) stdsdk.Headers {
	return stdsdk.Headers{"X-Convox-TID": tid}
}

func TestProxySameTenantTargetConnects(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		host := "web.rk-ab12-app.svc.cluster.local"

		p.On("Proxy", host, 5000, mock.Anything, structs.ProxyOptions{}).Return(nil).Run(func(args mock.Arguments) {
			rw, ok := args.Get(2).(io.ReadWriter)
			require.True(t, ok)
			_, err := rw.Write([]byte("out"))
			require.NoError(t, err)
		})

		r, err := c.Websocket("/proxy/"+host+"/5000", stdsdk.RequestOptions{Headers: tidHeaders("ab12")})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "out", string(data))
	})
}

func TestProxyCrossTenantTargetRejected(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r, err := c.Websocket("/proxy/web.rk-cd34-app.svc.cluster.local/5000", stdsdk.RequestOptions{Headers: tidHeaders("ab12")})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: invalid proxy host\n"), data)

		p.AssertNotCalled(t, "Proxy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestProxyBareLocalTargetRejectedForTenant(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r, err := c.Websocket("/proxy/web.app.rk.local/5000", stdsdk.RequestOptions{Headers: tidHeaders("ab12")})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: invalid proxy host\n"), data)

		p.AssertNotCalled(t, "Proxy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestProxySuppressedTIDRejectedOnGatedRack(t *testing.T) {
	t.Setenv("FEATURE_GATES", "tid=true")

	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r, err := c.Websocket("/proxy/web.rk-ab12-app.svc.cluster.local/5000", stdsdk.RequestOptions{})
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: invalid proxy host\n"), data)

		p.AssertNotCalled(t, "Proxy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestProxyCanonicalTIDCannotOverrideStampedTID(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{
			"X-Convox-TID": "ab12",
			"Convox-TID":   "cd34",
		}}

		r, err := c.Websocket("/proxy/web.rk-cd34-app.svc.cluster.local/5000", ro)
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: invalid proxy host\n"), data)

		p.AssertNotCalled(t, "Proxy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestProxySuppliedCanonicalTIDIgnoredOnGatedRack(t *testing.T) {
	t.Setenv("FEATURE_GATES", "tid=true")

	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		ro := stdsdk.RequestOptions{Headers: stdsdk.Headers{"Convox-TID": "cd34"}}

		r, err := c.Websocket("/proxy/web.rk-cd34-app.svc.cluster.local/5000", ro)
		require.NoError(t, err)

		data, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: invalid proxy host\n"), data,
			"a suppressed X-Convox-TID must not let the unprefixed spelling name the tenant")

		p.AssertNotCalled(t, "Proxy", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestProxyHttpServiceCrossTenantTargetRejected(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		res, err := proxyHttpServiceRequest(c, "web.rk-cd34-app.svc.cluster.local", "ab12")
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

func TestProxyHttpServiceSameTenantTargetPasses(t *testing.T) {
	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		res, err := proxyHttpServiceRequest(c, "web.rk-ab12-app.svc.cluster.local", "ab12")
		require.NoError(t, err)
		defer res.Body.Close()

		require.NotEqual(t, http.StatusForbidden, res.StatusCode,
			"a same-tenant target must reach the reverse proxy, which then fails to dial")
	})
}

func TestSystemJwtTokenRejectedOnGatedRack(t *testing.T) {
	t.Setenv("FEATURE_GATES", "tid=true")

	testTenantServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		req, err := http.NewRequest(http.MethodPost, c.Endpoint.String()+"/system/jwt/token", nil)
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()

		require.Equal(t, http.StatusForbidden, res.StatusCode)
	})
}

func proxyHttpServiceRequest(c *stdsdk.Client, host, tid string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.Endpoint.String()+"/custom/http/proxy/", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Host", host)
	req.Header.Set("X-Port", "5000")
	req.Header.Set("X-Convox-TID", tid)

	return http.DefaultClient.Do(req)
}
