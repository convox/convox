package k8s

import (
	"fmt"

	"github.com/convox/convox/pkg/options"
)

func (p *Provider) ResolverHost() (string, error) {
	if options.GetFeatureGates()[options.FeatureGateExternalDnsResolver] {
		return "1.1.1.1", nil
	}

	if p.Resolver == "" {
		return "", fmt.Errorf("resolver host not set")
	}
	return p.Resolver, nil
}
