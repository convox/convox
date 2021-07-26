package metal

import (
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/loki"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
)

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	tags := map[string]string{
		"rack":   p.Name,
		"app":    app,
		"stream": stream,
	}

	if err := p.loki.Post(tags, ts, message); err != nil {
		return err
	}

	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	topts := loki.TailOptions{}

	if opts.Since != nil {
		topts.Start = options.Time(time.Now().UTC().Add(*opts.Since * -1))
	}

	return p.loki.Tail(fmt.Sprintf(`{app=%q}`, name), topts)
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.AppLogs("system", opts)
}
