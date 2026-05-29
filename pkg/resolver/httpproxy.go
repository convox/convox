package resolver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

const httpProxyGroupVersion = "projectcontour.io/v1"

var httpProxyGVR = schema.GroupVersionResource{Group: "projectcontour.io", Version: "v1", Resource: "httpproxies"}

// HTTPProxy watches Contour HTTPProxy objects so the resolver classifies their
// fqdns the same way it classifies networking Ingress hosts. It is inert until
// the projectcontour.io CRD exists, so nginx racks and non-AWS providers never
// start the watch.
type HTTPProxy struct {
	discovery     discovery.DiscoveryInterface
	dynamic       dynamic.Interface
	hosts         map[string]bool
	internalHosts map[string]bool
	lock          sync.Mutex
	stopch        chan struct{}
}

func NewHTTPProxy(dc dynamic.Interface, disco discovery.DiscoveryInterface) (*HTTPProxy, error) {
	return &HTTPProxy{
		discovery:     disco,
		dynamic:       dc,
		hosts:         map[string]bool{},
		internalHosts: map[string]bool{},
		stopch:        make(chan struct{}),
	}, nil
}

func (h *HTTPProxy) Start() {
	if !h.waitForCRD() {
		return
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(opts am.ListOptions) (runtime.Object, error) {
				opts.LabelSelector = "system=convox"
				return h.dynamic.Resource(httpProxyGVR).List(context.Background(), opts)
			},
			WatchFunc: func(opts am.ListOptions) (watch.Interface, error) {
				opts.LabelSelector = "system=convox"
				return h.dynamic.Resource(httpProxyGVR).Watch(context.Background(), opts)
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    h.add,
		DeleteFunc: h.delete,
		UpdateFunc: h.update,
	}); err != nil {
		fmt.Printf("ns=httpproxy at=start error=%q\n", err)
		return
	}

	informer.Run(h.stopch)
}

func (h *HTTPProxy) Stop() {
	close(h.stopch)
}

func (h *HTTPProxy) waitForCRD() bool {
	for {
		if h.crdPresent() {
			return true
		}

		select {
		case <-h.stopch:
			return false
		case <-time.After(30 * time.Second):
		}
	}
}

func (h *HTTPProxy) crdPresent() bool {
	rs, err := h.discovery.ServerResourcesForGroupVersion(httpProxyGroupVersion)
	if err != nil {
		return false
	}

	for i := range rs.APIResources {
		if rs.APIResources[i].Name == "httpproxies" {
			return true
		}
	}

	return false
}

func (h *HTTPProxy) HostExists(host string) bool {
	h.lock.Lock()
	defer h.lock.Unlock()

	return h.hosts[host]
}

func (h *HTTPProxy) HostInternal(host string) bool {
	h.lock.Lock()
	defer h.lock.Unlock()

	return h.internalHosts[host]
}

func (h *HTTPProxy) add(obj interface{}) {
	hp, err := assertHTTPProxy(obj)
	if err != nil {
		fmt.Printf("ns=httpproxy at=add error=%q\n", err)
		return
	}

	fmt.Printf("ns=httpproxy at=add namespace=%s name=%s\n", hp.GetNamespace(), hp.GetName())

	h.lock.Lock()
	defer h.lock.Unlock()

	h.addProxy(hp)
}

func (h *HTTPProxy) delete(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	hp, err := assertHTTPProxy(obj)
	if err != nil {
		fmt.Printf("ns=httpproxy at=delete error=%q\n", err)
		return
	}

	fmt.Printf("ns=httpproxy at=delete namespace=%s name=%s\n", hp.GetNamespace(), hp.GetName())

	h.lock.Lock()
	defer h.lock.Unlock()

	h.deleteProxy(hp)
}

func (h *HTTPProxy) update(prev, cur interface{}) {
	pp, err := assertHTTPProxy(prev)
	if err != nil {
		fmt.Printf("ns=httpproxy at=update error=%q\n", err)
		return
	}

	cp, err := assertHTTPProxy(cur)
	if err != nil {
		fmt.Printf("ns=httpproxy at=update error=%q\n", err)
		return
	}

	fmt.Printf("ns=httpproxy at=update namespace=%s name=%s\n", cp.GetNamespace(), cp.GetName())

	h.lock.Lock()
	defer h.lock.Unlock()

	h.deleteProxy(pp)
	h.addProxy(cp)
}

func (h *HTTPProxy) addProxy(hp *unstructured.Unstructured) {
	host, internal := httpProxyHost(hp)
	if host == "" {
		return
	}

	h.hosts[host] = true
	if internal {
		h.internalHosts[host] = true
	}
}

func (h *HTTPProxy) deleteProxy(hp *unstructured.Unstructured) {
	host, _ := httpProxyHost(hp)
	if host == "" {
		return
	}

	delete(h.hosts, host)
	delete(h.internalHosts, host)
}

func httpProxyHost(hp *unstructured.Unstructured) (string, bool) {
	host, _, _ := unstructured.NestedString(hp.Object, "spec", "virtualhost", "fqdn")
	class, _, _ := unstructured.NestedString(hp.Object, "spec", "ingressClassName")

	return host, strings.Contains(class, "internal")
}

func assertHTTPProxy(v interface{}) (*unstructured.Unstructured, error) {
	hp, ok := v.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("could not assert httpproxy for type: %T", v)
	}

	return hp, nil
}
