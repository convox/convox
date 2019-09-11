package gcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging/logadmin"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"google.golang.org/api/iterator"
)

var sequenceTokens sync.Map

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.logFilter(p.Context(), w, p.logFilters(name), opts)

	return r, nil
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.AppLogs("system", opts)
}

func (p *Provider) logFilter(ctx context.Context, w io.WriteCloser, filter string, opts structs.LogsOptions) {
	defer w.Close()
	defer fmt.Println("finishing logs")

	var since time.Time

	if opts.Since != nil {
		since = time.Now().UTC().Add(-1 * *opts.Since)
	}

Iteration:

	for {
		fmt.Printf("since: %+v\n", since)
		it := p.LogAdmin.Entries(ctx, logadmin.Filter(fmt.Sprintf("%s AND timestamp > %q", filter, since.Format("2006-01-02T15:04:05.999999999Z"))))

		for {
			// check for closed writer
			if _, err := w.Write([]byte{}); err != nil {
				fmt.Println("closed writer")
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
				entry, err := it.Next()
				if err == iterator.Done {
					fmt.Println("iterator done")
					time.Sleep(2 * time.Second)
					continue Iteration
				}
				if err != nil {
					fmt.Fprintf(w, "ERROR: %s\n", err)
					return
				}

				prefix := ""

				if common.DefaultBool(opts.Prefix, false) {
					prefix = fmt.Sprintf("%s service/%s/%s ", entry.Timestamp.Format(time.RFC3339), "unknown", entry.Labels["container.googleapis.com/pod_name"])
				}

				switch t := entry.Payload.(type) {
				case string:
					if _, err := w.Write([]byte(fmt.Sprintf("%s%s\n", prefix, strings.TrimSuffix(t, "\n")))); err != nil {
						fmt.Printf("err: %+v\n", err)
					}
				}

				if entry.Timestamp.After(since) {
					since = entry.Timestamp
				}
			}
		}
	}
}

func (p *Provider) logFilters(app string) string {
	return fmt.Sprintf(`labels."container.googleapis.com/namespace_name" = %q`, p.AppNamespace(app))
}
