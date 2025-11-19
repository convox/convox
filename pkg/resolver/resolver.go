package resolver

import (
	"context"
	"fmt"
	"net"
	"os"

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
	ingress        *Ingress
	kubernetes     *kubernetes.Clientset
	namespace      string
	routerExternal string
	routerInternal string
	service        *Service
}

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

func New(namespace string) (*Resolver, error) {
	fmt.Printf("ns=resolver fn=new\n")

	r := &Resolver{
		namespace: namespace,
	}

	if err := r.setupKubernetes(); err != nil {
		return nil, err
	}

	if err := r.setupDNS(); err != nil {
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

	go r.ingress.Start()
	go r.service.Start()

	return <-ch
}

func (r *Resolver) Shutdown(ctx context.Context) error {
	_ = r.dnsExternal.Shutdown(ctx)
	_ = r.dnsInternal.Shutdown(ctx)

	r.ingress.Stop()
	r.service.Stop()

	return nil
}

func (r *Resolver) resolve(typ, host, router string) (string, bool) {
	if r.ingress.HostExists(host) {
		switch typ {
		case "A":
			if net.ParseIP(router) != nil {
				return router, true
			}
			ips, err := net.LookupIP(router)
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				return "", true
			}
			if len(ips) < 0 {
				fmt.Printf("no ip found for: %s\n", router)
				return "", true
			}
			return ips[0].String(), true
		default:
			return "", true
		}
	}

	if ip, ok := r.service.IP(host); ok {
		switch typ {
		case "A":
			return ip, true
		default:
			return "", true
		}
	}

	return "", false
}

func (r *Resolver) resolveExternal(typ, host string) (string, bool) {
	return r.resolve(typ, host, r.routerExternal)
}

func (r *Resolver) resolveInternal(typ, host string) (string, bool) {
	return r.resolve(typ, host, r.routerInternal)
}

func (r *Resolver) setupDNS() error {
	upstream := r.upstream()

	ce, err := net.ListenPacket("udp", ":5453")
	if err != nil {
		return err
	}

	de, err := NewDNS(ce, r.resolveExternal, upstream)
	if err != nil {
		return err
	}

	r.dnsExternal = de

	ci, err := net.ListenPacket("udp", ":5454")
	if err != nil {
		return err
	}

	di, err := NewDNS(ci, r.resolveInternal, upstream)
	if err != nil {
		return err
	}

	r.dnsInternal = di

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
	s, err := r.kubernetes.CoreV1().Services(r.namespace).Get(context.TODO(), "router", am.GetOptions{})
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
	if dns_upstream := os.Getenv("DNS_UPSTREAM"); dns_upstream != "" {
		return dns_upstream
	}

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
