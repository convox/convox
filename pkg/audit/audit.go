package audit

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/convox/convox/pkg/jwt"
)

const (
	Marker   = "rackaudit"
	fieldCap = 512
)

var Output io.Writer = os.Stdout

type Entry struct {
	Type    string `json:"rackaudit"`
	Time    string `json:"time"`
	ID      string `json:"id,omitempty"`
	Op      string `json:"op"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status,omitempty"`
	Auth    string `json:"auth"`
	Role    string `json:"role,omitempty"`
	Actor   string `json:"actor,omitempty"`
	Token   string `json:"token,omitempty"`
	IP      string `json:"ip"`
	Peer    string `json:"peer"`
	TID     string `json:"tid,omitempty"`
	Pid     string `json:"pid,omitempty"`
	Command string `json:"command,omitempty"`
	Minted  string `json:"minted,omitempty"`
	Granted string `json:"granted,omitempty"`
	Hours   int    `json:"hours,omitempty"`
}

func Write(e *Entry) {
	if e.Time == "" {
		e.Time = time.Now().UTC().Format(time.RFC3339)
	}
	e.Method = truncate(e.Method, fieldCap)
	e.Path = truncate(e.Path, fieldCap)
	e.TID = truncate(e.TID, fieldCap)
	e.Command = truncate(e.Command, fieldCap)

	var buf bytes.Buffer
	buf.WriteByte('\n')
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(e); err != nil {
		return
	}

	_, _ = Output.Write(buf.Bytes())
}

func NewEntry(r *http.Request) *Entry {
	e := &Entry{Method: r.Method, Path: r.URL.Path}
	e.IP, e.Peer = ClientAddr(r)

	switch username, password, ok := r.BasicAuth(); {
	case !ok:
		e.Auth = "none"
	case username == "jwt":
		e.Auth, e.Token = "jwt", jwt.Fingerprint(password)
	default:
		e.Auth = "password"
	}

	return e
}

// ClientAddr returns the address the rack's router saw and the TCP peer.
func ClientAddr(r *http.Request) (ip, peer string) {
	peer = r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		peer = host
	}
	if a, ok := parseAddr(peer); ok {
		peer = a
	}

	ip = peer
	if vs := r.Header.Values("X-Forwarded-For"); len(vs) > 0 {
		entries := strings.Split(vs[len(vs)-1], ",")
		if a, ok := parseAddr(strings.TrimSpace(entries[len(entries)-1])); ok {
			ip = a
		}
	}

	return ip, peer
}

// Actor returns the actor a password caller claims in its headers, else rack-password.
func Actor(h http.Header) string {
	for _, name := range []string{"X-Convox-Actor", "Convox-Actor"} {
		raw := strings.TrimSpace(h.Get(name))
		if raw == "" {
			continue
		}
		if a := SanitizeActor(raw); a != "unknown" {
			return a
		}
	}

	return "rack-password"
}

func parseAddr(s string) (string, bool) {
	a, err := netip.ParseAddr(s)
	if err != nil {
		ap, perr := netip.ParseAddrPort(s)
		if perr != nil {
			return "", false
		}
		a = ap.Addr()
	}

	return a.WithZone("").Unmap().String(), true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}

	return s[:n]
}
