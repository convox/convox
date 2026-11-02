package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTenantIngressAnnotations(t *testing.T) {
	tests := []struct {
		Key string
		Err string
	}{
		{Key: "nginx.ingress.kubernetes.io/limit-rpm"},
		{Key: "nginx.ingress.kubernetes.io/enable-access-log"},
		{Key: "nginx.ingress.kubernetes.io/auth-signin"},
		{Key: "nginx.ingress.kubernetes.io/proxy-body-size"},
		{Key: "nginx.ingress.kubernetes.io/server-alias"},
		{Key: "example.com/team"},
		{Key: "nginx.ingress.kubernetes.io/limit-rpm "},
		{Key: "nginx.ingress.kubernetes.io/configuration-snippet", Err: "ingress annotation nginx.ingress.kubernetes.io/configuration-snippet is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/server-snippet", Err: "ingress annotation nginx.ingress.kubernetes.io/server-snippet is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/auth-snippet", Err: "ingress annotation nginx.ingress.kubernetes.io/auth-snippet is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/stream-snippet", Err: "ingress annotation nginx.ingress.kubernetes.io/stream-snippet is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/modsecurity-snippet", Err: "ingress annotation nginx.ingress.kubernetes.io/modsecurity-snippet is not supported on this rack"},
		{Key: "Nginx.Ingress.Kubernetes.io/Configuration-Snippet", Err: "ingress annotation Nginx.Ingress.Kubernetes.io/Configuration-Snippet is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/canary", Err: "ingress annotation nginx.ingress.kubernetes.io/canary is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/canary-weight", Err: "ingress annotation nginx.ingress.kubernetes.io/canary-weight is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/auth-url", Err: "ingress annotation nginx.ingress.kubernetes.io/auth-url is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/mirror-target", Err: "ingress annotation nginx.ingress.kubernetes.io/mirror-target is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/mirror-host", Err: "ingress annotation nginx.ingress.kubernetes.io/mirror-host is not supported on this rack"},
		{Key: "nginx.ingress.kubernetes.io/server-snippet ", Err: "ingress annotation nginx.ingress.kubernetes.io/server-snippet is not supported on this rack"},
		{Key: `"nginx.ingress.kubernetes.io/configuration\x2dsnippet"`, Err: `ingressAnnotations key "\"nginx.ingress.kubernetes.io/configuration\\x2dsnippet\"" is not a valid annotation key`},
	}

	for _, test := range tests {
		t.Run(test.Key, func(t *testing.T) {
			m := &Manifest{Services: Services{{Name: "web", IngressAnnotations: Annotations{{Key: test.Key, Value: "x"}}}}}

			err := m.ValidateTenantIngressAnnotations()
			if test.Err == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, "validation errors:\nservice web: "+test.Err)
		})
	}
}

func TestValidateTenantIngressAnnotationsCollectsAll(t *testing.T) {
	m := &Manifest{Services: Services{{Name: "web", IngressAnnotations: Annotations{
		{Key: "nginx.ingress.kubernetes.io/auth-url", Value: "x"},
		{Key: "nginx.ingress.kubernetes.io/canary", Value: "true"},
	}}}}

	require.EqualError(t, m.ValidateTenantIngressAnnotations(), "validation errors:\nservice web: ingress annotation nginx.ingress.kubernetes.io/auth-url is not supported on this rack\nservice web: ingress annotation nginx.ingress.kubernetes.io/canary is not supported on this rack")
}

func TestValidateTenantIngressAnnotationsLoaded(t *testing.T) {
	m, err := Load([]byte("services:\n  web:\n    port: 5000\n    ingressAnnotations:\n      - \"nginx.ingress.kubernetes.io/server-snippet =return 418;\"\n"), map[string]string{})
	require.NoError(t, err)

	require.EqualError(t, m.ValidateTenantIngressAnnotations(), "validation errors:\nservice web: ingress annotation nginx.ingress.kubernetes.io/server-snippet is not supported on this rack")
}
