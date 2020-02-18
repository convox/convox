package aws

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	as := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt",
	}

	return as, nil
}

func (p *Provider) IngressClass() string {
	return "nginx"
}
