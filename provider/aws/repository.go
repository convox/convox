package aws

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	host, _, err := p.RepositoryHost(app)
	if err != nil {
		return "", "", err
	}

	return p.ecrAuth(host, "", "")
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	registry, err := p.appRegistry(app)
	if err != nil {
		return "", false, err
	}

	return registry, true, nil
}
