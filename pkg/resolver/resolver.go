package resolver

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/convox/convox/pkg/common"
	"github.com/miekg/dns"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	UpstreamFallback = "1.1.1.1:53"
)

type Resolver struct {
	dnsExternal    *DNS
	dnsInternal    *DNS
	health         *Health
	ingress        *Ingress
	kubernetes     *kubernetes.Clientset
	namespace      string
	routerExternal string
	routerInternal string
	service        *Service
	updates        map[string]map[string]bool
	updatelock     sync.RWMutex
}

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

func New(namespace string) (*Resolver, error) {
	fmt.Printf("ns=resolver fn=new\n")

	r := &Resolver{
		namespace: namespace,
		updates:   map[string]map[string]bool{},
	}

	if err := r.setupKubernetes(); err != nil {
		return nil, err
	}

	if err := r.setupDNS(); err != nil {
		return nil, err
	}

	if err := r.setupHealth(); err != nil {
		return nil, err
	}

	if err := r.setupIngress(); err != nil {
		return nil, err
	}

	if err := r.setupRouter(); err != nil {
		return nil, err
	}

	if err := r.setupService(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Resolver) Serve() error {
	ch := make(chan error, 1)

	go serve(ch, r.dnsExternal)
	go serve(ch, r.dnsInternal)
	go serve(ch, r.health)

	go r.ingress.Start()
	go r.service.Start()

	return <-ch
}

func (r *Resolver) Shutdown(ctx context.Context) error {
	_ = r.dnsExternal.Shutdown(ctx)
	_ = r.dnsInternal.Shutdown(ctx)
	_ = r.health.Shutdown(ctx)

	r.ingress.Stop()
	r.service.Stop()

	return nil
}

func (r *Resolver) resolve(typ, host, router string) ([]string, bool) {
	switch typ {
	case "A":
		if r.ingress.HostExists(host) {
			if net.ParseIP(router) != nil {
				return []string{router}, true
			}
			ips, err := net.LookupIP(router)
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				return nil, true
			}
			if len(ips) < 0 {
				fmt.Printf("no ip found for: %s\n", router)
				return nil, true
			}
			ipss := make([]string, len(ips))
			for i := range ips {
				ipss[i] = ips[i].String()
			}
			return ipss, true
		}
	case "TXT":
		r.updatelock.RLock()
		defer r.updatelock.RUnlock()

		if vh, ok := r.updates[fmt.Sprintf("%s.", host)]; ok {
			values := []string{}
			for k := range vh {
				values = append(values, k)
			}
			return values, true
		}
	}

	return nil, false
}

func (r *Resolver) resolveExternal(typ, host string) ([]string, bool) {
	switch typ {
	// case "SOA":
	// 	r.updatelock.RLock()
	// 	defer r.updatelock.RUnlock()

	// 	if _, ok := r.updates[fmt.Sprintf("%s.", host)]; ok {
	// 		return nil, true
	// 	}
	case "TXT":
		r.updatelock.RLock()
		defer r.updatelock.RUnlock()

		if vh, ok := r.updates[fmt.Sprintf("%s.", host)]; ok {
			values := []string{}
			for k := range vh {
				values = append(values, k)
			}
			return values, true
		}

		return nil, true
	}

	return r.resolve(typ, host, r.routerExternal)
}

func (r *Resolver) resolveInternal(typ, host string) ([]string, bool) {
	if ip, ok := r.service.IP(host); ok {
		switch typ {
		case "A":
			return []string{ip}, true
		default:
			return nil, true
		}
	}

	return r.resolve(typ, host, r.routerInternal)
}

func (r *Resolver) update(typ, host string, values []string) error {
	fmt.Printf("ns=resolver at=update type=%s host=%q values=%v\n", typ, host, values)

	switch typ {
	case "TXT":
		r.updatelock.Lock()
		defer r.updatelock.Unlock()
		if _, ok := r.updates[host]; !ok {
			r.updates[host] = map[string]bool{}
		}
		for _, value := range values {
			r.updates[host][value] = true
		}
	}

	return nil
}

func (r *Resolver) setupDNS() error {
	de, err := NewDNS("udp", ":5453")
	if err != nil {
		return err
	}

	de.Resolver = r.resolveExternal

	r.dnsExternal = de

	di, err := NewDNS("udp", ":5454")
	if err != nil {
		return err
	}

	di.Resolver = r.resolveInternal
	di.Updater = r.update
	di.Upstream = r.upstream()

	r.dnsInternal = di

	return nil
}

func (r *Resolver) setupHealth() error {
	h, err := NewHealth(":5452")
	if err != nil {
		return err
	}

	r.health = h

	return nil
}

func (r *Resolver) setupKubernetes() error {
	c, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	kc, err := kubernetes.NewForConfig(c)
	if err != nil {
		return err
	}

	r.kubernetes = kc

	return nil
}

func (r *Resolver) setupIngress() error {
	i, err := NewIngress(r.kubernetes)
	if err != nil {
		return err
	}

	r.ingress = i

	return nil
}

func (r *Resolver) setupRouter() error {
	s, err := r.kubernetes.CoreV1().Services(r.namespace).Get("router", am.GetOptions{})
	if err != nil {
		return err
	}

	r.routerInternal = s.Spec.ClusterIP
	r.routerExternal = s.Spec.ClusterIP

	if is := s.Status.LoadBalancer.Ingress; len(is) > 0 {
		r.routerExternal = common.CoalesceString(is[0].IP, is[0].Hostname)
	}

	return nil
}

func (r *Resolver) setupService() error {
	s, err := NewService(r.kubernetes)
	if err != nil {
		return err
	}

	r.service = s

	return nil
}

func (r *Resolver) upstream() string {
	cc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return UpstreamFallback
	}

	if len(cc.Servers) < 1 {
		return UpstreamFallback
	}

	return fmt.Sprintf("%s:53", cc.Servers[0])
}

func listOptions(opts *am.ListOptions) {
	opts.LabelSelector = "system=convox"
}

func serve(ch chan error, s Server) {
	if err := s.ListenAndServe(); err != nil {
		ch <- err
	}
}
