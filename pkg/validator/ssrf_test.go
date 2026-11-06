package validator_test

import (
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/validator"
)

// stubResolver returns canned IPs (or an error) per hostname. nil
// values map to "did not call resolver"; the tests below assert that
// allowlisted suffixes never reach the resolver.
type stubResolver struct {
	answers map[string][]net.IP
	errs    map[string]error
	calls   map[string]int
}

func newStubResolver() *stubResolver {
	return &stubResolver{
		answers: map[string][]net.IP{},
		errs:    map[string]error{},
		calls:   map[string]int{},
	}
}

func (r *stubResolver) resolve(host string) ([]net.IP, error) {
	r.calls[host]++
	if e, has := r.errs[host]; has {
		return nil, e
	}
	if a, has := r.answers[host]; has {
		return a, nil
	}
	return nil, errors.New("no such host")
}

func ip(s string) net.IP {
	out := net.ParseIP(s)
	if out == nil {
		panic("ssrf_test: bad IP literal " + s)
	}
	return out
}

func TestValidateExternalURL_AcceptCases(t *testing.T) {
	r := newStubResolver()
	r.answers["prom.example.com"] = []net.IP{ip("203.0.113.10")}
	r.answers["prom-dual.example.com"] = []net.IP{ip("203.0.113.10"), ip("2606:4700:4700::1111")}
	r.answers["amp.aps-workspaces.us-east-1.amazonaws.com"] = []net.IP{ip("198.51.100.20")}

	cases := []struct {
		name string
		raw  string
	}{
		{"empty_string_accepted", ""},
		{"http_public_dns", "http://prom.example.com:9090"},
		{"https_public_dns_dual_stack", "https://prom-dual.example.com"},
		{"in_cluster_short_form", "http://prom.kube-system.svc.cluster.local:9090"},
		{"in_cluster_paid_recipe", "http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090"},
		{"in_cluster_uppercase", "http://prom.SVC.CLUSTER.LOCAL"},
		{"public_https_amp_workspace", "https://amp.aps-workspaces.us-east-1.amazonaws.com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validator.ValidateExternalURL(c.raw, r.resolve)
			if err != nil {
				t.Fatalf("expected accept, got error: %v", err)
			}
		})
	}

	// In-cluster suffixes must NOT trigger the resolver — that's the
	// point of the allowlist (avoids latency on the documented recipe).
	if r.calls["prom.kube-system.svc.cluster.local"] != 0 {
		t.Errorf("in-cluster suffix should bypass resolver, got %d calls",
			r.calls["prom.kube-system.svc.cluster.local"])
	}
	if r.calls["prom.SVC.CLUSTER.LOCAL"] != 0 {
		t.Errorf("case-insensitive in-cluster suffix should bypass resolver, got %d calls",
			r.calls["prom.SVC.CLUSTER.LOCAL"])
	}
}

func TestValidateExternalURL_RejectIPLiterals(t *testing.T) {
	r := newStubResolver()

	cases := []struct {
		name string
		raw  string
	}{
		{"private_10", "http://10.0.0.1"},
		{"private_172_16", "http://172.16.0.1"},
		{"private_192_168", "http://192.168.1.1"},
		{"loopback_127", "http://127.0.0.1"},
		{"loopback_ipv6", "http://[::1]"},
		{"link_local_169_254", "https://169.254.169.254/"},
		{"link_local_ipv6", "http://[fe80::1]"},
		{"unspecified_v4", "http://0.0.0.0"},
		{"unspecified_v6", "http://[::]"},
		{"cgnat_100_64", "http://100.64.0.1"},
		{"ipv6_ula_fc00", "http://[fc00::1]"},
		{"localhost_reserved_name", "http://localhost"},
		{"localhost_with_port", "http://localhost:9090"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validator.ValidateExternalURL(c.raw, r.resolve)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", c.raw)
			}
			if !strings.Contains(err.Error(), "non-routable") &&
				!strings.Contains(err.Error(), "did not resolve") {
				t.Errorf("error message should reference non-routable or did-not-resolve; got %q", err.Error())
			}
		})
	}
}

func TestValidateExternalURL_RejectDNSResolvedPrivate(t *testing.T) {
	r := newStubResolver()
	r.answers["host-private.example.com"] = []net.IP{ip("10.0.0.1")}
	r.answers["host-mixed.example.com"] = []net.IP{ip("203.0.113.5"), ip("169.254.169.254")}
	r.answers["host-cgnat.example.com"] = []net.IP{ip("100.64.0.1")}
	r.answers["host-ula.example.com"] = []net.IP{ip("fc00::1")}
	r.answers["host-loopback.example.com"] = []net.IP{ip("127.0.0.1")}

	cases := []struct {
		name string
		raw  string
	}{
		{"dns_resolves_to_private", "http://host-private.example.com:9090"},
		{"dns_resolves_mixed_any_blocked", "http://host-mixed.example.com:9090"},
		{"dns_resolves_to_cgnat", "http://host-cgnat.example.com:9090"},
		{"dns_resolves_to_ipv6_ula", "http://host-ula.example.com:9090"},
		{"dns_resolves_to_loopback", "http://host-loopback.example.com:9090"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validator.ValidateExternalURL(c.raw, r.resolve)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", c.raw)
			}
			if !strings.Contains(err.Error(), "non-routable") {
				t.Errorf("error should reference non-routable; got %q", err.Error())
			}
		})
	}
}

func TestValidateExternalURL_RejectResolutionFailure(t *testing.T) {
	r := newStubResolver()
	r.errs["nxdomain.example.com"] = errors.New("no such host")

	err := validator.ValidateExternalURL("http://nxdomain.example.com", r.resolve)
	if err == nil {
		t.Fatalf("expected error on NXDOMAIN, got nil")
	}
	if !strings.Contains(err.Error(), "did not resolve") {
		t.Errorf("error should reference did-not-resolve; got %q", err.Error())
	}
}

func TestValidateExternalURL_RejectZeroAddresses(t *testing.T) {
	r := newStubResolver()
	r.answers["empty.example.com"] = []net.IP{}

	err := validator.ValidateExternalURL("http://empty.example.com", r.resolve)
	if err == nil {
		t.Fatalf("expected error on zero-address resolve, got nil")
	}
	if !strings.Contains(err.Error(), "did not resolve") {
		t.Errorf("error should reference did-not-resolve; got %q", err.Error())
	}
}

func TestValidateExternalURL_RejectScheme(t *testing.T) {
	r := newStubResolver()

	cases := []struct {
		name string
		raw  string
	}{
		{"file_scheme", "file:///etc/passwd"},
		{"gopher_scheme", "gopher://x"},
		{"ftp_scheme", "ftp://x"},
		{"missing_scheme", "prom.example.com:9090"},
		{"empty_after_scheme_marker", "://nohost"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validator.ValidateExternalURL(c.raw, r.resolve)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", c.raw)
			}
			if !strings.Contains(err.Error(), "http or https") {
				t.Errorf("error should reference http or https scheme; got %q", err.Error())
			}
		})
	}
}

func TestValidateExternalURL_RejectMissingHost(t *testing.T) {
	r := newStubResolver()
	err := validator.ValidateExternalURL("http://", r.resolve)
	if err == nil {
		t.Fatalf("expected error for missing host")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error should reference host; got %q", err.Error())
	}
}

func TestValidateExternalURL_AllowlistSuffixIsAnchored(t *testing.T) {
	// A registered domain that ends in "svc.cluster.local" without the
	// leading dot must NOT be treated as in-cluster — the resolver must
	// still be called.
	r := newStubResolver()
	r.answers["host.attackersvc.cluster.localexample.com"] = []net.IP{ip("203.0.113.99")}
	// Also test a host that contains the suffix but not at the end.
	r.answers["svc.cluster.local.attacker.example.com"] = []net.IP{ip("10.0.0.1")}

	if err := validator.ValidateExternalURL("http://host.attackersvc.cluster.localexample.com", r.resolve); err != nil {
		t.Fatalf("non-suffix-matching host with public IP should accept, got %v", err)
	}
	if r.calls["host.attackersvc.cluster.localexample.com"] == 0 {
		t.Errorf("non-suffix-matching host should hit resolver")
	}

	if err := validator.ValidateExternalURL("http://svc.cluster.local.attacker.example.com", r.resolve); err == nil {
		t.Fatalf("attacker domain resolving to private IP must be rejected")
	}
	if r.calls["svc.cluster.local.attacker.example.com"] == 0 {
		t.Errorf("attacker domain should hit resolver (not match the in-cluster allowlist)")
	}
}

func TestIsBlockedIP_PublicAccepts(t *testing.T) {
	cases := []struct {
		name    string
		ip      string
		blocked bool
	}{
		{"public_v4_1.2.3.4", "1.2.3.4", false},
		{"public_v4_documentation_block", "203.0.113.5", false},
		{"public_v6_cloudflare_dns", "2606:4700:4700::1111", false},
		{"private_10", "10.0.0.1", true},
		{"loopback", "127.0.0.1", true},
		{"link_local_imds", "169.254.169.254", true},
		{"cgnat", "100.64.0.1", true},
		{"ipv6_ula", "fc00::1", true},
		{"unspecified_v4", "0.0.0.0", true},
		{"unspecified_v6", "::", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := validator.IsBlockedIP(ip(c.ip))
			if got != c.blocked {
				t.Errorf("IsBlockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
			}
		})
	}
	// nil IP: the helper short-circuits to false (caller has already
	// failed to parse and handles its own error).
	if validator.IsBlockedIP(nil) {
		t.Errorf("IsBlockedIP(nil) should return false")
	}
}
