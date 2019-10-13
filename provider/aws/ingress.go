package aws

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (p *Provider) IngressSecrets(app string) ([]string, error) {
	return []string{}, nil
}
