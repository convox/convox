package k8s

func (p *Provider) ResolverHost() (string, error) {
	return p.Resolver, nil
}
