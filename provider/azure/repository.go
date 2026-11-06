package azure

import (
	"fmt"

	"github.com/convox/convox/pkg/structs"
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

func (p *Provider) RepositoryImagesBatchDelete(app string, tags []string) error {
	return structs.ErrNotImplemented("not implemented")
}
