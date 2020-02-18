package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

type DNS struct {
	client   dns.Client
	handler  DNSHandler
	mux      *dns.ServeMux
	server   *dns.Server
	soa      dns.RR
	upstream string
}

type DNSHandler func(typ, host string) (string, bool)

func NewDNS(conn net.PacketConn, handler DNSHandler, upstream string) (*DNS, error) {
	mux := dns.NewServeMux()

	fmt.Printf("ns=dns at=new upstream=%s\n", upstream)

	d := &DNS{
		client:  dns.Client{Net: "udp"},
		handler: handler,
		mux:     mux,
		server: &dns.Server{
			PacketConn: conn,
			Handler:    mux,
		},
		upstream: upstream,
	}

	soa, err := dns.NewRR("$ORIGIN .\n$TTL 0\n@ SOA ns.convox. support.convox.com. 2020010100 0 0 0 0")
	if err != nil {
		return nil, err
	}

	d.soa = soa

	mux.Handle(".", d)

	return d, nil
}

func (d *DNS) ListenAndServe() error {
	fmt.Printf("ns=dns at=serve\n")

	return d.server.ActivateAndServe()
}

func (d *DNS) Shutdown(ctx context.Context) error {
	fmt.Printf("ns=dns at=shutdown\n")

	return d.server.Shutdown()
}

func (d *DNS) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) < 1 {
		dnsError(w, r, fmt.Errorf("invalid question"))
		return
	}

	q := r.Question[0]
	typ := dns.TypeToString[q.Qtype]
	question := strings.TrimSuffix(r.Question[0].Name, ".")

	fmt.Printf("ns=dns at=question type=%s question=%q\n", typ, question)

	a := &dns.Msg{}

	a.Compress = false
	a.RecursionAvailable = true

	if r.IsEdns0() != nil {
		a.SetEdns0(4096, true)
	}

	a.SetReply(r)

	if answer, ok := d.handler(typ, question); ok {
		fmt.Printf("ns=dns at=answer type=%s question=%q answer=%q\n", typ, question, answer)

		if answer != "" {
			rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", question, typ, answer))
			if err != nil {
				dnsError(w, r, err)
				return
			}

			a.Answer = append(a.Answer, rr)
			a.Authoritative = true
			a.Ns = []dns.RR{d.soa}
		}

		w.WriteMsg(a)

		return
	}

	fmt.Printf("ns=dns at=forward type=%s question=%q\n", typ, question)

	rs, _, err := d.client.Exchange(r, d.upstream)
	if err != nil {
		dnsError(w, r, err)
		return
	}

	w.WriteMsg(rs)
}

func dnsError(w dns.ResponseWriter, r *dns.Msg, err error) {
	fmt.Printf("ns=dns at=error error=%s\n", err)
	m := &dns.Msg{}
	m.SetRcode(r, dns.RcodeServerFailure)
	w.WriteMsg(m)
}
