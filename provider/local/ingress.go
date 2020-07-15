package local

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	ans := map[string]string{
		"cert-manager.io/cluster-issuer": "self-signed",
	}

	return ans, nil
}

func (p *Provider) IngressClass() string {
	return "nginx"
}
