package do

import (
	"fmt"
	"io"
	"net/url"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) BuildExport(app, id string, w io.Writer) error {
	return p.Provider.BuildExport(app, id, w)
}

func (p *Provider) BuildImport(app string, r io.Reader) (*structs.Build, error) {
	return p.Provider.BuildImport(app, r)
}

func (p *Provider) BuildLogs(app, id string, opts structs.LogsOptions) (io.ReadCloser, error) {
	b, err := p.BuildGet(app, id)
	if err != nil {
		return nil, err
	}

	opts.Since = nil

	switch b.Status {
	case "running":
		return p.ProcessLogs(app, b.Process, opts)
	default:
		u, err := url.Parse(b.Logs)
		if err != nil {
			return nil, err
		}

		switch u.Scheme {
		case "object":
			return p.ObjectFetch(u.Hostname(), u.Path)
		default:
			return nil, fmt.Errorf("unable to read logs for build: %s", id)
		}
	}
}
