package k8s

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	apiProxyPrefix = "/kubernetes/"
)

type apiProxy struct {
	handler http.Handler
}

func (p *Provider) apiProxy() (*apiProxy, error) {
	host := p.Config.Host
	if !strings.HasSuffix(host, "/") {
		host = host + "/"
	}

	target, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	responder := &apiResponder{}

	transport, err := rest.TransportFor(p.Config)
	if err != nil {
		return nil, err
	}

	upgradeTransport, err := makeUpgradeTransport(p.Config, 0)
	if err != nil {
		return nil, err
	}

	proxy := proxy.NewUpgradeAwareHandler(target, transport, false, false, responder)
	proxy.UpgradeTransport = upgradeTransport
	proxy.UseRequestLocation = true

	proxyServer := http.Handler(proxy)

	mux := http.NewServeMux()

	mux.HandleFunc(apiProxyPrefix, stripLeaveSlash(apiProxyPrefix, p.apiProxyAuthenticate(proxyServer)))

	return &apiProxy{handler: mux}, nil
}

func (p *Provider) apiProxyAuthenticate(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, _ := r.BasicAuth()
		if username == "jwt" && p.JwtMngr != nil {
			_, err := p.JwtMngr.Verify(password)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		} else {
			if password != p.Password {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		r.Header.Del("Authorization")

		handler.ServeHTTP(w, r)
	}
}

func (p *apiProxy) ListenAndServe(addr string, port int) error {
	s := http.Server{Handler: p.handler}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return fmt.Errorf("error: could not create kubernetes proxy listener: %v", err)
	}

	if err := s.Serve(ln); err != nil {
		return fmt.Errorf("error: could not start kubernetes proxy listener: %v", err)
	}

	return nil
}

func makeUpgradeTransport(config *rest.Config, keepalive time.Duration) (proxy.UpgradeRequestRoundTripper, error) {
	transportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	tlsConfig, err := transport.TLSConfigFor(transportConfig)
	if err != nil {
		return nil, err
	}
	rt := utilnet.SetOldTransportDefaults(&http.Transport{
		TLSClientConfig: tlsConfig,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: keepalive,
		}).DialContext,
	})

	upgrader, err := transport.HTTPWrappersForConfig(transportConfig, proxy.MirrorRequest)
	if err != nil {
		return nil, err
	}
	return proxy.NewUpgradeRequestRoundTripper(rt, upgrader), nil
}

func stripLeaveSlash(prefix string, fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, prefix)
		if len(p) >= len(req.URL.Path) {
			http.NotFound(w, req)
			return
		}
		if len(p) > 0 && p[:1] != "/" {
			p = "/" + p
		}
		req.URL.Path = p
		fn(w, req)
	}
}

type apiResponder struct{}

func (r *apiResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	fmt.Printf("error: api proxy failure: %v\n", err)
}
