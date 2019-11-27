package aws

func (p *Provider) RegistryAuth(host, username, password string) (string, string, error) {
	if ecrHostMatcher.MatchString(host) {
		return p.ecrAuth(host, username, password)
	}

	return username, password, nil
}
