package local

import (
	"io"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) BuildExport(app, id string, w io.Writer) error {
	return p.Provider.BuildExport(app, id, w)
}

func (p *Provider) BuildImport(app string, r io.Reader) (*structs.Build, error) {
	return p.Provider.BuildImport(app, r)
}
