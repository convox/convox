package gcp

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	return map[string]string{"kubernetes.io/ingress.class": "convox"}, nil
}
