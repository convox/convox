package local

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	ans := map[string]string{
		"kubernetes.io/ingress.class": "convox",
	}

	return ans, nil
}

func (p *Provider) IngressClass() string {
	return "convox"
}
