package resolver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type DNS struct {
	Resolver DNSResolver
	Updater  DNSUpdater
	Upstream string

	addr   string
	client dns.Client
	proto  string
	server *dns.Server
	soa    dns.RR
}

type DNSResolver func(typ, host string) ([]string, bool)
type DNSUpdater func(typ, host string, values []string) error

func NewDNS(proto, addr string) (*DNS, error) {
	mux := dns.NewServeMux()

	fmt.Printf("ns=dns at=new proto=%s addr=%s\n", proto, addr)

	d := &DNS{
		addr:   addr,
		client: dns.Client{Net: "udp"},
		proto:  proto,
		server: &dns.Server{Handler: mux},
	}

	soa, err := dns.NewRR("$ORIGIN .\n$TTL 0\n@ SOA ns.convox. support.convox.com. 2020010100 0 0 0 0")
	if err != nil {
		return nil, err
	}

	d.soa = soa

	mux.Handle(".", d)

	fmt.Println("setting tsig secret")
	d.server.TsigSecret = map[string]string{"axfr.": "Zm9vCg=="}

	d.server.MsgAcceptFunc = func(h dns.Header) dns.MsgAcceptAction {
		// x := dns.DefaultMsgAcceptFunc(h)
		return dns.MsgAccept
	}

	return d, nil
}

func (d *DNS) ListenAndServe() error {
	fmt.Printf("ns=dns at=serve\n")

	conn, err := net.ListenPacket(d.proto, d.addr)
	if err != nil {
		return err
	}

	d.server.PacketConn = conn

	return d.server.ActivateAndServe()
}

func (d *DNS) Shutdown(ctx context.Context) error {
	fmt.Printf("ns=dns at=shutdown\n")

	if err := d.server.Shutdown(); err != nil {
		return err
	}

	if err := d.server.PacketConn.Close(); err != nil {
		return err
	}

	return nil
}

func (d *DNS) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// fmt.Printf("r: %+v\n", r)

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

	if d.Updater != nil {
		if r.Opcode == dns.OpcodeUpdate && r.IsTsig() != nil && w.TsigStatus() == nil {
			for _, ns := range r.Ns {
				fmt.Printf("ns: %+v\n", ns)
				if txt, ok := ns.(*dns.TXT); ok {
					fmt.Printf("txt: %#v\n", txt)
					switch txt.Header().Class {
					case dns.ClassINET:
						name := txt.Header().Name
						vals := txt.Txt

						fmt.Printf("name: %+v\n", name)
						fmt.Printf("vals: %+v\n", vals)

						if err := d.Updater("TXT", name, vals); err != nil {
							dnsError(w, r, err)
							return
						}
					}
				}
			}
			a.SetTsig("axfr.", dns.HmacMD5, 300, time.Now().Unix())
			w.WriteMsg(a)
			return
		}
	}

	if d.Resolver != nil {
		if answer, ok := d.Resolver(typ, question); ok {
			fmt.Printf("ns=dns at=resolver type=%s question=%q answer=%v\n", typ, question, answer)

			a.Authoritative = true
			a.Ns = []dns.RR{d.soa}

			switch typ {
			// case "SOA":
			// 	a.Answer = append(a.Answer, d.soa)
			default:
				for _, value := range answer {
					rr, err := dns.NewRR(fmt.Sprintf("%s %s %s", question, typ, value))
					if err != nil {
						dnsError(w, r, err)
						return
					}

					rr.Header().Ttl = 60

					a.Answer = append(a.Answer, rr)
				}
			}

			w.WriteMsg(a)

			return
		}
	}

	if d.Upstream != "" {
		fmt.Printf("ns=dns at=forward type=%s question=%q\n", typ, question)

		rs, _, err := d.client.Exchange(r, d.Upstream)
		if err != nil {
			dnsError(w, r, err)
			return
		}

		w.WriteMsg(rs)
	}
}

func dnsError(w dns.ResponseWriter, r *dns.Msg, err error) {
	fmt.Printf("ns=dns at=error error=%s\n", err)
	m := &dns.Msg{}
	m.SetRcode(r, dns.RcodeServerFailure)
	w.WriteMsg(m)
}
