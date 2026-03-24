package resolver

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSetupRouterOverrideUsed(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "10.96.100.50")

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.100.50", r.routerInternal)
	require.Equal(t, "10.96.100.50", r.routerExternal)
}

func TestSetupRouterOverrideEmptyStringIgnored(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.100",
		},
	}

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(svc),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.0.100", r.routerInternal)
	require.Equal(t, "10.96.0.100", r.routerExternal)
}

func TestSetupRouterFallbackToServiceLookup(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.200",
		},
	}

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(svc),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.0.200", r.routerInternal)
	require.Equal(t, "10.96.0.200", r.routerExternal)
}

func TestSetupRouterFallbackWithLoadBalancer(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "")

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.100",
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "203.0.113.50"},
				},
			},
		},
	}

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(svc),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.0.100", r.routerInternal)
	require.Equal(t, "203.0.113.50", r.routerExternal)
}

func TestSetupRouterOverrideTakesPrecedenceOverService(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "10.96.100.99")

	// Even with a service that has a LoadBalancer, the override should win
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router",
			Namespace: "test-ns",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.100",
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{IP: "203.0.113.50"},
				},
			},
		},
	}

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(svc),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.100.99", r.routerInternal)
	require.Equal(t, "10.96.100.99", r.routerExternal)
}

func TestSetupRouterNoOverrideNoService(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "")

	// No router service exists — should return an error (cloud rack misconfiguration)
	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.Error(t, err)
}

func TestSetupRouterOverrideInvalidIP(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "not-an-ip")

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a valid IPv4 address")
}

func TestSetupRouterOverrideWhitespaceOnly(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "   ")

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.Error(t, err)
}

func TestSetupRouterOverrideIPv6Rejected(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "::1")

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a valid IPv4 address")
}

func TestSetupRouterOverrideTrimmedWhitespace(t *testing.T) {
	t.Setenv("ROUTER_IP_OVERRIDE", "  10.96.0.1  ")

	r := &Resolver{
		namespace:  "test-ns",
		kubernetes: fake.NewSimpleClientset(),
	}

	err := r.setupRouter()
	require.NoError(t, err)
	require.Equal(t, "10.96.0.1", r.routerInternal)
	require.Equal(t, "10.96.0.1", r.routerExternal)
}
