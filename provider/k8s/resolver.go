package k8s

import (
	"fmt"
	"net"

	"github.com/convox/convox/pkg/options"
)

func (p *Provider) ResolverHost() (string, error) {
	if dnsip := options.GetFeatureGateValue(options.FeatureGateExternalDnsResolver); dnsip != "" {
		if net.ParseIP(dnsip) != nil {
			return dnsip, nil
		}
	}

	if p.Resolver == "" {
		return "", fmt.Errorf("resolver host not set")
	}
	return p.Resolver, nil
}
