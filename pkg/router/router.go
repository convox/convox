package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/miekg/dns"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

const (
	healthTick  = 10 * time.Second
	idleTick    = 1 * time.Minute
	idleTimeout = 60 * time.Minute
)

var (
	targetParser = regexp.MustCompile(`^([^.]+)\.([^.]+)\.svc\.cluster\.local$`)
)

type Router struct {
	DNSExternal Server
	DNSInternal Server
	HTTP        Server
	HTTPS       Server

	backend Backend
	cache   autocert.Cache
	certs   sync.Map
	storage Storage
}

type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func New() (*Router, error) {
	fmt.Printf("ns=router fn=new\n")

	r := &Router{
		certs: sync.Map{},
	}

	switch os.Getenv("BACKEND") {
	default:
		b, err := NewBackendKubernetes(r, os.Getenv("NAMESPACE"))
		if err != nil {
			return nil, err
		}

		r.backend = b
	}

	switch os.Getenv("CACHE") {
	case "dynamodb":
		c, err := NewCacheDynamo(os.Getenv("DYNAMODB_CACHE"))
		if err != nil {
			return nil, err
		}

		r.cache = c
	case "redis":
		c, err := NewCacheRedis(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_AUTH"), os.Getenv("REDIS_SECURE") == "true")
		if err != nil {
			return nil, err
		}

		r.cache = c
	default:
		r.cache = NewCacheMemory()
	}

	switch os.Getenv("STORAGE") {
	case "dynamodb":
		s, err := NewStorageDynamo(os.Getenv("DYNAMODB_HOSTS"), os.Getenv("DYNAMODB_TARGETS"))
		if err != nil {
			return nil, err
		}

		r.storage = s
	case "redis":
		s, err := NewStorageRedis(os.Getenv("REDIS_ADDR"), os.Getenv("REDIS_AUTH"), os.Getenv("REDIS_SECURE") == "true")
		if err != nil {
			return nil, err
		}

		r.storage = s
	default:
		r.storage = NewStorageMemory()
	}

	fmt.Printf("ns=router fn=new at=backend.start\n")

	if err := r.backend.Start(); err != nil {
		return nil, err
	}

	fmt.Printf("ns=router fn=new at=dns\n")

	if err := r.setupDNS(); err != nil {
		return nil, err
	}

	fmt.Printf("ns=router fn=new at=http\n")

	if err := r.setupHTTP(); err != nil {
		return nil, err
	}

	fmt.Printf("ns=router fn=new at=done\n")

	return r, nil
}

func (r *Router) RouterIP(internal bool) string {
	if internal {
		return r.backend.InternalIP()
	} else {
		return r.backend.ExternalIP()
	}
}

func (r *Router) Serve() error {
	ch := make(chan error, 1)

	go serve(ch, r.DNSExternal)
	go serve(ch, r.DNSInternal)
	go serve(ch, r.HTTP)
	go serve(ch, r.HTTPS)

	go r.healthTicker()
	go r.idleTicker()

	return <-ch
}

func (r *Router) Shutdown(ctx context.Context) error {
	if err := r.HTTPS.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

func (r *Router) RequestBegin(target string) error {
	fmt.Printf("ns=router at=request.begin target=%q\n", target)

	if err := r.storage.RequestBegin(target); err != nil {
		return err
	}

	idle, err := r.backend.IdleGet(target)
	if err != nil {
		return fmt.Errorf("could not fetch idle status: %s", err)
	}

	if idle {
		if err := r.backend.IdleSet(target, false); err != nil {
			return fmt.Errorf("could not unidle: %s", err)
		}
	}

	return nil
}

func (r *Router) RequestEnd(target string) error {
	fmt.Printf("ns=router at=request.end target=%q\n", target)

	return r.storage.RequestEnd(target)
}

func (r *Router) Route(host string) (string, error) {
	fmt.Printf("ns=router at=route host=%q\n", host)

	for _, vr := range validRoutes(host) {
		ts, err := r.TargetList(vr)
		if err != nil {
			return "", fmt.Errorf("error reaching backend")
		}

		if len(ts) > 0 {
			return ts[rand.Intn(len(ts))], nil
		}
	}

	return "", fmt.Errorf("no backends available")
}

// if the target will idle make sure it has the system=convox label
// so the controller can watch it
func (r *Router) TargetAdd(host, target string, idles bool) error {
	fmt.Printf("ns=router at=target.add host=%q target=%q\n", host, target)

	if err := r.storage.TargetAdd(host, target, idles); err != nil {
		return err
	}

	return nil
}

func (r *Router) TargetList(host string) ([]string, error) {
	fmt.Printf("ns=router at=target.list host=%q\n", host)

	return r.storage.TargetList(host)
}

func (r *Router) TargetRemove(host, target string) error {
	fmt.Printf("ns=router at=target.delete host=%q target=%q\n", host, target)

	return r.storage.TargetRemove(host, target)
}

func (r *Router) Upstream() (string, error) {
	cc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return "", err
	}

	if len(cc.Servers) < 1 {
		return "", fmt.Errorf("no upstream dns")
	}

	return fmt.Sprintf("%s:53", cc.Servers[0]), nil
}

func (r *Router) autocertHostPolicy(ctx context.Context, host string) error {
	ts, err := r.storage.TargetList(host)
	if err != nil {
		return err
	}
	if len(ts) == 0 {
		return fmt.Errorf("unknown host")
	}

	return nil
}

func (r *Router) generateCertificateAutocert(m *autocert.Manager) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if hello.ServerName == "" {
			return common.CertificateSelfSigned("convox")
		}

		c, err := m.GetCertificate(hello)
		if err != nil {
			fmt.Printf("err: %+v\n", err)
			return nil, err
		}

		return c, nil
	}
}

func (r *Router) generateCertificateCA(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := hello.ServerName

	v, ok := r.certs.Load(host)
	if ok {
		if c, ok := v.(tls.Certificate); ok {
			return &c, nil
		}
	}

	ca, err := r.backend.CA()
	if err != nil {
		return nil, err
	}

	c, err := common.CertificateCA(host, ca)
	if err != nil {
		return nil, err
	}

	r.certs.Store(host, *c)

	return c, nil
}

// try to request every known host on a timer to trigger things like
// certificate generation before the user gets to them
func (r *Router) healthTicker() {
	time.Sleep(60 * time.Second)

	for range time.Tick(healthTick) {
		if err := r.healthTick(); err != nil {
			fmt.Printf("ns=router at=health.ticker error=%v\n", err)
		}
	}
}

func (r *Router) healthTick() error {
	hs, err := r.storage.HostList()
	if err != nil {
		return err
	}

	for _, h := range hs {
		if strings.HasSuffix(h, ".local") {
			continue
		}

		if _, err = http.Get(fmt.Sprintf("https://%s/convox/health", h)); err != nil {
			fmt.Printf("ns=router at=health.tick host=%q error=%v\n", h, err)
		}
	}

	return nil
}

func (r *Router) idleTicker() {
	for range time.Tick(idleTick) {
		if err := r.idleTick(); err != nil {
			fmt.Printf("ns=router at=idle.ticker error=%v\n", err)
		}
	}
}

func (r *Router) idleTick() error {
	ts, err := r.storage.Stale(time.Now().UTC().Add(-1 * idleTimeout))
	if err != nil {
		return err
	}

	for _, t := range ts {
		idle, err := r.backend.IdleGet(t)
		if err != nil {
			return err
		}
		if idle {
			continue
		}

		if err := r.backend.IdleSet(t, true); err != nil {
			fmt.Printf("err = %+v\n", err)
		}
	}

	return nil
}

func (r *Router) setupDNS() error {
	ce, err := net.ListenPacket("udp", ":5453")
	if err != nil {
		return err
	}

	de, err := NewDNS(ce, r, false)
	if err != nil {
		return err
	}

	ci, err := net.ListenPacket("udp", ":5454")
	if err != nil {
		return err
	}

	di, err := NewDNS(ci, r, true)
	if err != nil {
		return err
	}

	r.DNSExternal = de
	r.DNSInternal = di

	return nil
}

func (r *Router) setupHTTP() error {
	if os.Getenv("AUTOCERT") == "true" {
		return r.setupHTTPAutocert()
	}

	ln, err := tls.Listen("tcp", ":443", &tls.Config{
		GetCertificate: r.generateCertificateCA,
	})
	if err != nil {
		return err
	}

	https, err := NewHTTP(ln, r)
	if err != nil {
		return err
	}

	r.HTTPS = https

	r.HTTP = &http.Server{Addr: ":80", Handler: redirectHTTPS(https.ServeHTTP)}

	return nil
}

func (r *Router) setupHTTPAutocert() error {
	m := &autocert.Manager{
		Cache:      r.cache,
		HostPolicy: r.autocertHostPolicy,
		Prompt:     autocert.AcceptTOS,
	}

	ln, err := tls.Listen("tcp", fmt.Sprintf(":443"), &tls.Config{
		GetCertificate: r.generateCertificateAutocert(m),
		NextProtos:     []string{"h2", "http/1.1", acme.ALPNProto},
	})
	if err != nil {
		return err
	}

	https, err := NewHTTP(ln, r)
	if err != nil {
		return err
	}

	r.HTTPS = https

	r.HTTP = &http.Server{Addr: ":80", Handler: m.HTTPHandler(redirectHTTPS(https.ServeHTTP))}

	return nil
}

func parseTarget(target string) (string, string, bool) {
	u, err := url.Parse(target)
	if err != nil {
		return "", "", false
	}

	if m := targetParser.FindStringSubmatch(u.Hostname()); len(m) == 3 {
		return m[1], m[2], true
	}

	return "", "", false
}

func redirectHTTPS(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Proto") == "https" {
			fn(w, r)
			return
		}

		target := url.URL{Scheme: "https", Host: r.Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}

		http.Redirect(w, r, target.String(), http.StatusMovedPermanently)
	}
}

func serve(ch chan error, s Server) {
	err := s.ListenAndServe()

	switch err {
	case http.ErrServerClosed:
	case nil:
	default:
		ch <- err
	}
}

func validRoutes(host string) []string {
	if net.ParseIP(host) != nil {
		return []string{host}
	}

	parts := strings.Split(host, ".")

	rs := make([]string, len(parts))

	rs[0] = host

	for i := 1; i < len(parts); i++ {
		rs[i] = fmt.Sprintf("*.%s", strings.Join(parts[i:], "."))
	}

	return rs
}
