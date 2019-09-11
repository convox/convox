package gcp

import (
	"fmt"

	"github.com/convox/convox/pkg/manifest"
)

func (p *Provider) ServiceHost(app string, s manifest.Service) string {
	return fmt.Sprintf("%s.%s.%s", s.Name, app, p.Domain)
}
