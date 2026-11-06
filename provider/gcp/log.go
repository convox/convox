package gcp

import (
	"io"
	"time"

	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	tags := map[string]string{
		"stream": stream,
	}

	if err := p.elastic.Write(p.Name, app, ts, message, tags); err != nil {
		return err
	}

	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.elastic.Stream(p.Context(), w, p.Name, name, opts)

	return r, nil
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.AppLogs("system", opts)
}
