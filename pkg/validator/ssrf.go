// Package validator provides SSRF guard helpers for user-supplied URLs.
package validator

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const InClusterSuffix = ".svc.cluster.local"

var cgnatBlock = mustCIDR("100.64.0.0/10") // RFC 6598 (CGNAT)
var ipv6ULA = mustCIDR("fc00::/7")         // RFC 4193 (ULA)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("validator: bad CIDR constant %q: %v", s, err))
	}
	return n
}

// IsBlockedIP returns true for RFC 1918, loopback, link-local, CGNAT, and ULA addresses.
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

type ResolveFunc func(host string) ([]net.IP, error)

// ValidateExternalURL rejects URLs targeting internal/non-routable IPs.
// Pass net.LookupIP for resolveHost in production; nil defaults to it.
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

	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("URL resolves to a non-routable address; if you intended an in-cluster Prometheus, use a *%s hostname", InClusterSuffix)
	}

	// dot-prefixed suffix prevents attacker-registered domains matching (DNS rebinding)
	if strings.HasSuffix(strings.ToLower(host), InClusterSuffix) {
		return nil
	}

	if ip := net.ParseIP(host); ip != nil {
		if IsBlockedIP(ip) {
			return fmt.Errorf("URL resolves to a non-routable address; if you intended an in-cluster Prometheus, use a *%s hostname", InClusterSuffix)
		}
		return nil
	}

	// DNS: check every resolved A/AAAA record against deny-set
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
