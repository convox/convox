package azure

func (p *Provider) IngressAnnotations(app string) (map[string]string, error) {
	a, err := p.AppGet(app)
	if err != nil {
		return nil, err
	}

	ans := map[string]string{
		"kubernetes.io/ingress.class": "convox",
	}

	switch a.Parameters["Router"] {
	case "dedicated":
		ans["kubernetes.io/ingress.class"] = "gce"
	}

	return ans, nil
}

func (p *Provider) IngressSecrets(app string) ([]string, error) {
	return []string{}, nil
}
