package gcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"google.golang.org/api/iterator"
)

const (
	logTimeFormat = "2006-01-02T15:04:05.999999999Z"
)

var sequenceTokens sync.Map

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	logger := p.Logging.Logger("system")

	logger.Log(logging.Entry{
		Labels: map[string]string{
			"container.googleapis.com/namespace_name": p.AppNamespace(app),
			"stream": stream,
		},
		Payload:  message,
		Severity: logging.Info,
	})

	if err := logger.Flush(); err != nil {
		fmt.Printf("err: %+v\n", err)
		return err
	}

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

	var since time.Time

	if opts.Since != nil {
		since = time.Now().UTC().Add(-1 * *opts.Since)
	}

	now := time.Now().UTC()
	follow := common.DefaultBool(opts.Follow, true)

Iteration:

	for {
		where := fmt.Sprintf("%s AND timestamp > %q", filter, since.Format(logTimeFormat))

		if !follow {
			where = fmt.Sprintf("%s AND timestamp < %q", where, now.Format(logTimeFormat))
		}

		it := p.LogAdmin.Entries(ctx, logadmin.Filter(where))

		for {
			// check for closed writer
			if _, err := w.Write([]byte{}); err != nil {
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
				entry, err := it.Next()
				if err == iterator.Done {
					if !follow {
						return
					}
					time.Sleep(2 * time.Second)
					continue Iteration
				}
				if err != nil {
					fmt.Fprintf(w, "ERROR: %s\n", err)
					return
				}

				prefix := ""

				labels := entry.Resource.GetLabels()

				typ := common.CoalesceString(entry.Labels["k8s-pod/type"], "unknown")
				name := common.CoalesceString(entry.Labels["k8s-pod/name"], "unknown")

				if common.DefaultBool(opts.Prefix, false) {
					prefix = fmt.Sprintf("%s %s/%s/%s ", entry.Timestamp.Format(time.RFC3339), typ, name, labels["pod_name"])
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
	filters := []string{
		`resource.type="k8s_container"`,
		fmt.Sprintf(`resource.labels.cluster_name=%q`, p.Name),
		fmt.Sprintf(`resource.labels.namespace_name=%q`, p.AppNamespace(app)),
	}

	return strings.Join(filters, " AND ")
}
