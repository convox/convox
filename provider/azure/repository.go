package azure

import (
	"fmt"
)

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	return p.ClientID, p.ClientSecret, nil
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	return fmt.Sprintf("%s/%s", p.Registry, app), true, nil
}

func (p *Provider) RepositoryPrefix() string {
	return ""
}
