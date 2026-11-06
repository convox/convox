package resolver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func fakeDiscovery(present bool) discovery.DiscoveryInterface {
	resources := []*metav1.APIResourceList{}
	if present {
		resources = append(resources, &metav1.APIResourceList{
			GroupVersion: "projectcontour.io/v1",
			APIResources: []metav1.APIResource{{Name: "httpproxies"}},
		})
	}
	return &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
}

func httpProxyObj(class, fqdn string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{Object: map[string]interface{}{}}
	o.SetAPIVersion("projectcontour.io/v1")
	o.SetKind("HTTPProxy")

	spec := map[string]interface{}{}
	if class != "" {
		spec["ingressClassName"] = class
	}
	if fqdn != "" {
		spec["virtualhost"] = map[string]interface{}{"fqdn": fqdn}
	}
	o.Object["spec"] = spec

	return o
}

func TestHTTPProxyHostClassification(t *testing.T) {
	h, err := NewHTTPProxy(nil, nil)
	require.NoError(t, err)

	h.add(httpProxyObj("contour", "web.app.example.com"))
	h.add(httpProxyObj("contour-internal", "api.app.convox.local"))

	require.True(t, h.HostExists("web.app.example.com"))
	require.False(t, h.HostInternal("web.app.example.com"))

	require.True(t, h.HostExists("api.app.convox.local"))
	require.True(t, h.HostInternal("api.app.convox.local"))

	require.False(t, h.HostExists("unknown.example.com"))
}

func TestHTTPProxyDeleteAndTombstone(t *testing.T) {
	h, err := NewHTTPProxy(nil, nil)
	require.NoError(t, err)

	internal := httpProxyObj("contour-internal", "api.app.convox.local")
	h.add(internal)
	require.True(t, h.HostInternal("api.app.convox.local"))

	h.delete(cache.DeletedFinalStateUnknown{Key: "x", Obj: internal})
	require.False(t, h.HostExists("api.app.convox.local"))
	require.False(t, h.HostInternal("api.app.convox.local"))
}

func TestHTTPProxyNoFqdnIgnored(t *testing.T) {
	h, err := NewHTTPProxy(nil, nil)
	require.NoError(t, err)

	h.add(httpProxyObj("contour-internal", ""))
	require.False(t, h.HostInternal(""))
	require.Equal(t, 0, len(h.hosts))
}

func TestHTTPProxyUpdate(t *testing.T) {
	h, err := NewHTTPProxy(nil, nil)
	require.NoError(t, err)

	h.add(httpProxyObj("contour-internal", "api.app.convox.local"))
	require.True(t, h.HostInternal("api.app.convox.local"))

	h.update(httpProxyObj("contour-internal", "api.app.convox.local"), httpProxyObj("contour-internal", "api2.app.convox.local"))
	require.False(t, h.HostExists("api.app.convox.local"))
	require.True(t, h.HostInternal("api2.app.convox.local"))
}

func TestHTTPProxyCRDPresent(t *testing.T) {
	present, err := NewHTTPProxy(nil, fakeDiscovery(true))
	require.NoError(t, err)
	require.True(t, present.crdPresent())

	absent, err := NewHTTPProxy(nil, fakeDiscovery(false))
	require.NoError(t, err)
	require.False(t, absent.crdPresent())
}

func TestHTTPProxyStartInertWithoutCRD(t *testing.T) {
	h, err := NewHTTPProxy(nil, fakeDiscovery(false))
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		h.Start()
		close(done)
	}()

	h.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Stop when the CRD is absent")
	}

	require.Equal(t, 0, len(h.hosts))
	require.Equal(t, 0, len(h.internalHosts))
}

func TestResolveInternalRoutesHTTPProxyHosts(t *testing.T) {
	h, err := NewHTTPProxy(nil, nil)
	require.NoError(t, err)
	h.add(httpProxyObj("contour-internal", "api.app.convox.local"))
	h.add(httpProxyObj("contour", "web.app.example.com"))

	r := &Resolver{
		ingress:                 &Ingress{hosts: map[string]bool{}, internalHosts: map[string]bool{}},
		httpproxy:               h,
		routerInternal:          "10.0.0.9",
		routerExternalClusterIP: "10.0.0.1",
	}

	ip, ok := r.resolveInternal("A", "api.app.convox.local")
	require.True(t, ok)
	require.Equal(t, "10.0.0.9", ip)

	ip, ok = r.resolveInternal("A", "web.app.example.com")
	require.True(t, ok)
	require.Equal(t, "10.0.0.1", ip)
}

func TestResolveNilHTTPProxyNoPanic(t *testing.T) {
	r := &Resolver{
		ingress:   &Ingress{hosts: map[string]bool{}, internalHosts: map[string]bool{}},
		httpproxy: nil,
		service:   &Service{},
	}

	require.NotPanics(t, func() {
		_, ok := r.resolveInternal("A", "unknown.example.com")
		require.False(t, ok)
	})
}
