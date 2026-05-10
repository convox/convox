// Package validator provides SSRF guard helpers for user-supplied URLs.
//
// ValidateExternalURL rejects URLs that target internal IP ranges, both
// when the host is an IP literal and when a DNS name resolves to one.
// The intent: a config setter who points the rack at an internal service
// (k8s API server, IMDS, in-cluster admin panels) gets a deterministic
// validation failure rather than a runtime SSRF.
//
// Behaviour:
//   - Empty URL: accepted (callers handle "unset means disabled" upstream).
//   - Non-http/https scheme: rejected.
//   - Hostname matches *.svc.cluster.local (case-insensitive): accepted
//     without DNS resolution. This is the documented in-cluster recipe.
//   - "localhost" reserved name: rejected.
//   - IP literal in deny-set: rejected.
//   - DNS hostname: every resolved A/AAAA record is checked; any address
//     in the deny-set rejects the URL.
//   - DNS resolution failure or zero addresses: rejected with a clear
//     "could not resolve" error so the caller gets a deterministic
//     outcome rather than passing validation but failing at runtime.
package validator

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// InClusterSuffix is the documented Kubernetes in-cluster DNS suffix.
// Hosts ending in this suffix bypass DNS-based deny checks because the
// rack operator is trusted to point at an in-cluster service (e.g. the
// kube-prometheus-stack chart deployed by Console).
const InClusterSuffix = ".svc.cluster.local"

// cgnatBlock is RFC 6598 (carrier-grade NAT). Go's net.IP.IsPrivate()
// covers RFC 1918 only; EKS sometimes uses 100.64/10 for pod CIDRs and
// CGNAT-mapped internal services exist in the wild, so add it
// explicitly to the deny-set.
var cgnatBlock = mustCIDR("100.64.0.0/10")

// ipv6ULA covers RFC 4193 unique local addresses (fc00::/7).
var ipv6ULA = mustCIDR("fc00::/7")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("validator: bad CIDR constant %q: %v", s, err))
	}
	return n
}

// IsBlockedIP returns true if the IP is in any range we treat as
// internal / non-routable for SSRF purposes:
//
//   - net.IP.IsPrivate (RFC 1918)
//   - net.IP.IsLoopback (127/8, ::1)
//   - net.IP.IsLinkLocalUnicast (169.254/16, fe80::/10)
//   - net.IP.IsLinkLocalMulticast (224.0.0/24, ff02::)
//   - net.IP.IsUnspecified (0.0.0.0, ::)
//   - 100.64.0.0/10 (CGNAT, RFC 6598)
//   - fc00::/7 (IPv6 ULA, RFC 4193)
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if cgnatBlock.Contains(ip) {
		return true
	}
	if ipv6ULA.Contains(ip) {
		return true
	}
	return false
}

// ResolveFunc resolves a hostname to a slice of IPs. Production callers
// pass net.LookupIP; tests pass a stub. Kept as a type alias rather
// than embedding net.LookupIP directly so test files can construct a
// resolver without importing the net package for its signature alone.
type ResolveFunc func(host string) ([]net.IP, error)

// ValidateExternalURL parses raw and applies SSRF guards. Returns nil
// on accept; an error suitable for surfacing to a CLI user otherwise.
//
// resolveHost is injectable so tests can stub the resolver. Production
// callers pass net.LookupIP. If nil is passed, net.LookupIP is used.
func ValidateExternalURL(raw string, resolveHost ResolveFunc) error {
	if raw == "" {
		return nil
	}
	if resolveHost == nil {
		resolveHost = net.LookupIP
	}

	parsed, parseErr := url.Parse(raw)
	scheme := ""
	if parsed != nil {
		scheme = strings.ToLower(parsed.Scheme)
	}
	if parseErr != nil || scheme == "" || (scheme != "http" && scheme != "https") {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL must include a host (e.g. http://prom.example.com:9090)")
	}

	host := parsed.Hostname()

	// Reject the "localhost" reserved name; it can resolve to 127.0.0.1
	// or ::1 depending on /etc/hosts ordering, both of which are blocked
	// for SSRF purposes.
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("URL resolves to a non-routable address; if you intended an in-cluster Prometheus, use a *%s hostname", InClusterSuffix)
	}

	// Allowlist: documented in-cluster recipe. Use HasSuffix on a
	// dot-prefixed suffix so an attacker-registered domain ending in
	// "svc.cluster.local" (without the leading dot) cannot match.
	if strings.HasSuffix(strings.ToLower(host), InClusterSuffix) {
		return nil
	}

	// IP literal: re-apply deny-set directly.
	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("URL resolves to a non-routable address; if you intended an in-cluster Prometheus, use a *%s hostname", InClusterSuffix)
		}
		return nil
	}

	// DNS hostname: resolve and apply deny-set to ALL resolved IPs.
	ips, err := resolveHost(host)
	if err != nil {
		return fmt.Errorf("hostname did not resolve (%v); set a publicly resolvable hostname or use a *%s hostname for in-cluster targets", err, InClusterSuffix)
	}
	if len(ips) == 0 {
		return fmt.Errorf("hostname did not resolve to any addresses; configure DNS or use a *%s hostname for in-cluster targets", InClusterSuffix)
	}
	for _, ip := range ips {
		if IsBlockedIP(ip) {
			return fmt.Errorf("URL resolves to a non-routable address; if you intended an in-cluster Prometheus, use a *%s hostname", InClusterSuffix)
		}
	}
	return nil
}
