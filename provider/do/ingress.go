package do

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	ans := map[string]string{
		"cert-manager.io/cluster-issuer": "letsencrypt-http",
	}

	return ans, nil
}

func (p *Provider) IngressClass() string {
	return "nginx"
}
