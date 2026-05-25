package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslateNginxAnnotations(t *testing.T) {
	cases := []struct {
		name     string
		anns     map[string]string
		expected httpProxyTranslation
	}{
		{
			name:     "empty",
			anns:     map[string]string{},
			expected: httpProxyTranslation{Annotations: map[string]string{}},
		},
		{
			name: "proxy-read-timeout sets response timeout",
			anns: map[string]string{"nginx.ingress.kubernetes.io/proxy-read-timeout": "120"},
			expected: httpProxyTranslation{
				Annotations:     map[string]string{},
				TimeoutResponse: "120s",
			},
		},
		{
			name: "proxy-send-timeout sets idle timeout",
			anns: map[string]string{"nginx.ingress.kubernetes.io/proxy-send-timeout": "90"},
			expected: httpProxyTranslation{
				Annotations: map[string]string{},
				TimeoutIdle: "90s",
			},
		},
		{
			name: "proxy-connect-timeout sets idle timeout",
			anns: map[string]string{"nginx.ingress.kubernetes.io/proxy-connect-timeout": "45"},
			expected: httpProxyTranslation{
				Annotations: map[string]string{},
				TimeoutIdle: "45s",
			},
		},
		{
			name: "idle timeout takes larger of send and connect",
			anns: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-send-timeout":    "30",
				"nginx.ingress.kubernetes.io/proxy-connect-timeout": "60",
			},
			expected: httpProxyTranslation{
				Annotations: map[string]string{},
				TimeoutIdle: "60s",
			},
		},
		{
			name: "idle timeout takes larger reversed order",
			anns: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-send-timeout":    "90",
				"nginx.ingress.kubernetes.io/proxy-connect-timeout": "30",
			},
			expected: httpProxyTranslation{
				Annotations: map[string]string{},
				TimeoutIdle: "90s",
			},
		},
		{
			name: "limit-rps",
			anns: map[string]string{"nginx.ingress.kubernetes.io/limit-rps": "100"},
			expected: httpProxyTranslation{
				Annotations:  map[string]string{},
				RateLimitRPS: 100,
			},
		},
		{
			name:     "non-integer limit-rps silently dropped",
			anns:     map[string]string{"nginx.ingress.kubernetes.io/limit-rps": "abc"},
			expected: httpProxyTranslation{Annotations: map[string]string{}},
		},
		{
			name: "untranslatable nginx annotation warns",
			anns: map[string]string{"nginx.ingress.kubernetes.io/server-snippet": "location /health { return 200; }"},
			expected: httpProxyTranslation{
				Annotations:          map[string]string{},
				IncompatibleWarnings: []string{"  - nginx.ingress.kubernetes.io/server-snippet (raw nginx config has no Contour equivalent)"},
			},
		},
		{
			name: "unknown nginx annotation warns",
			anns: map[string]string{"nginx.ingress.kubernetes.io/custom-thing": "val"},
			expected: httpProxyTranslation{
				Annotations:          map[string]string{},
				IncompatibleWarnings: []string{"  - nginx.ingress.kubernetes.io/custom-thing (no known Contour equivalent)"},
			},
		},
		{
			name: "contour-native annotation passes through",
			anns: map[string]string{"projectcontour.io/response-timeout": "30s"},
			expected: httpProxyTranslation{
				Annotations: map[string]string{"projectcontour.io/response-timeout": "30s"},
			},
		},
		{
			name: "non-nginx annotation passes through",
			anns: map[string]string{"custom.io/something": "value"},
			expected: httpProxyTranslation{
				Annotations: map[string]string{"custom.io/something": "value"},
			},
		},
		{
			name: "combined scenario",
			anns: map[string]string{
				"nginx.ingress.kubernetes.io/proxy-read-timeout": "300",
				"nginx.ingress.kubernetes.io/limit-rps":          "50",
				"custom.io/team":                                 "platform",
			},
			expected: httpProxyTranslation{
				Annotations:     map[string]string{"custom.io/team": "platform"},
				TimeoutResponse: "300s",
				RateLimitRPS:    50,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateNginxAnnotations(tc.anns, "test-svc")
			assert.Equal(t, tc.expected.TimeoutResponse, got.TimeoutResponse)
			assert.Equal(t, tc.expected.TimeoutIdle, got.TimeoutIdle)
			assert.Equal(t, tc.expected.RateLimitRPS, got.RateLimitRPS)
			assert.Equal(t, tc.expected.Annotations, got.Annotations)
			assert.Equal(t, tc.expected.IncompatibleWarnings, got.IncompatibleWarnings)
		})
	}
}

func TestContourBackendProtocol(t *testing.T) {
	cases := []struct {
		scheme   string
		expected string
	}{
		{"HTTP", ""},
		{"HTTPS", "tls"},
		{"GRPC", "h2c"},
		{"GRPCS", "h2"},
		{"http", ""},
		{"https", "tls"},
		{"TCP", ""},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.scheme, func(t *testing.T) {
			assert.Equal(t, tc.expected, contourBackendProtocol(tc.scheme))
		})
	}
}
