package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/mock"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/k8s/template"
	"github.com/stretchr/testify/require"
)

func httpProxyRenderParams(class string, proxyProtocol bool, whitelist []string, timeoutResponse string) map[string]interface{} {
	return map[string]interface{}{
		"Annotations":                map[string]string{},
		"App":                        "test-app",
		"BackendProtocol":            "",
		"ConvoxDomainTLSCertDisable": true,
		"Host":                       "web.test-app.example.com",
		"Idles":                      false,
		"IngressClassName":           class,
		"Namespace":                  "ns",
		"ProxyProtocol":              proxyProtocol,
		"Rack":                       "rack",
		"RateLimitRPS":               0,
		"Service": manifest.Service{
			Name:    "web",
			Port:    manifest.ServicePortScheme{Port: 8080, Scheme: "http"},
			Timeout: 60,
		},
		"TimeoutResponse": timeoutResponse,
		"TimeoutIdle":     "60s",
		"WhitelistCIDRs":  whitelist,
	}
}

func TestRenderTemplateHTTPProxySourceSelection(t *testing.T) {
	p := Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(template.TemplatesFS)

	ext, err := p.RenderTemplate("app/httpproxy", httpProxyRenderParams("contour", true, []string{"10.0.0.0/8"}, ""))
	require.NoError(t, err)
	require.Contains(t, string(ext), "ingressClassName: contour\n")
	require.Contains(t, string(ext), "source: Remote")
	require.NotContains(t, string(ext), "source: Peer")

	intl, err := p.RenderTemplate("app/httpproxy", httpProxyRenderParams("contour-internal", false, []string{"10.0.0.0/8"}, ""))
	require.NoError(t, err)
	require.Contains(t, string(intl), "ingressClassName: contour-internal")
	require.Contains(t, string(intl), "source: Peer")
	require.NotContains(t, string(intl), "source: Remote")
}

func TestResponseTimeout(t *testing.T) {
	require.Equal(t, "infinity", responseTimeout(""))
	require.Equal(t, "120s", responseTimeout("120s"))
}

func TestParseWhitelistCIDRs(t *testing.T) {
	require.Equal(t, []string{"10.0.0.0/8", "192.168.0.0/16"}, parseWhitelistCIDRs(" 10.0.0.0/8 ,, 192.168.0.0/16 "))
	require.Nil(t, parseWhitelistCIDRs(""))
	require.Nil(t, parseWhitelistCIDRs("  , ,  "))
}

func TestRenderTemplateHTTPProxyStreamingTimeout(t *testing.T) {
	p := Provider{Engine: &mock.TestEngine{}}
	p.templater = templater.New(template.TemplatesFS)

	uncapped, err := p.RenderTemplate("app/httpproxy", httpProxyRenderParams("contour", true, nil, "infinity"))
	require.NoError(t, err)
	require.Contains(t, string(uncapped), "response: infinity")
	require.Contains(t, string(uncapped), "idle:")

	capped, err := p.RenderTemplate("app/httpproxy", httpProxyRenderParams("contour", true, nil, "120s"))
	require.NoError(t, err)
	require.Contains(t, string(capped), "response: 120s")
}
