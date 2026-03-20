package azure

import (
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) ManifestValidate(m *manifest.Manifest) error {
	errs := []string{}

	for _, s := range m.Services {
		if len(s.Volumes) > 0 {
			errs = append(errs, fmt.Sprintf("shared volumes are not supported on gcp"))
			break
		}
	}

	if len(errs) > 0 {
		return structs.ErrBadRequest("manifest valiation errors:\n%s", strings.Join(errs, "\n"))
	}

	return nil
}
