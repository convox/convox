package azure

func (p *Provider) IngressAnnotations(certDuration string) (map[string]string, error) {
	ans := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt",
	}

	if certDuration != "" {
		ans["cert-manager.io/duration"] = certDuration
	}

	return ans, nil
}

func (p *Provider) IngressClass() string {
	return "nginx"
}

func (p *Provider) IngressInternalClass() string {
	return "nginx-internal"
}
