package local

import "fmt"

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	return "docker", p.Secret, nil
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	return fmt.Sprintf("%s/%s", p.Registry, app), true, nil
}

func (p *Provider) RepositoryPrefix() string {
	return ""
}
