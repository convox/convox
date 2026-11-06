package gcp

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) RepositoryAuth(app string) (string, string, error) {
	return "_json_key", string(p.Key), nil
}

func (p *Provider) RepositoryHost(app string) (string, bool, error) {
	return fmt.Sprintf("%s/%s", p.Registry, app), true, nil
}

// The registry proxy forwards only the pushed path, so it must include Artifact Registry's project and repository.
func (p *Provider) RepositoryPrefix() string {
	if _, path, ok := strings.Cut(p.Registry, "/"); ok {
		return path + "/"
	}
	return ""
}

func (p *Provider) RepositoryImagesBatchDelete(app string, tags []string) error {
	return structs.ErrNotImplemented("not implemented")
}
