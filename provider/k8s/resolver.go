package k8s

import "fmt"

func (p *Provider) ResolverHost() (string, error) {
	if p.Resolver == "" {
		return "", fmt.Errorf("resolver host not set")
	}
	return p.Resolver, nil
}
