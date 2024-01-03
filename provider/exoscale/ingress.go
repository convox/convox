package exoscale

func (p *Provider) IngressAnnotations(certDuration string) (map[string]string, error) {
	as := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt",
	}

	if certDuration != "" {
		as["cert-manager.io/duration"] = certDuration
	}

	return as, nil
}

func (p *Provider) IngressClass() string {
	return "nginx"
}

func (p *Provider) IngressInternalClass() string {
	return "nginx-internal"
}
