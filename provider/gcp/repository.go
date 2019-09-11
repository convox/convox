package gcp

import (
	"fmt"
)

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	return "_json_key", string(p.Key), nil
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	return fmt.Sprintf("%s/%s", p.Registry, app), true, nil
}
