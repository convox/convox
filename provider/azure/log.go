package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
)

var sequenceTokens sync.Map

type elasticSearchResult struct {
	Hits struct {
		Hits []struct {
			Index  string `json:"_index"`
			Source struct {
				Log       string
				Stream    string
				Timestamp time.Time `json:"@timestamp"`
			} `json:"_source"`
		}
	}
}

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	index := fmt.Sprintf("convox.%s.%s", p.Name, app)

	body := map[string]interface{}{
		"log":        fmt.Sprintf("%s\n", message),
		"stream":     stream,
		"@timestamp": ts.Format(time.RFC3339Nano),
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	if _, err := p.elastic.Index(index, bytes.NewReader(data)); err != nil {
		return err
	}

	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.streamLogs(p.Context(), w, fmt.Sprintf("convox.%s.%s", p.Name, name), opts)

	return r, nil
}

func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
	return p.AppLogs("system", opts)
}

func (p *Provider) streamLogs(ctx context.Context, w io.WriteCloser, index string, opts structs.LogsOptions) {
	defer w.Close()

	follow := common.DefaultBool(opts.Follow, true)
	now := time.Now().UTC()
	since := time.Time{}

	if opts.Since != nil {
		since = time.Now().UTC().Add(*opts.Since * -1)
	}

	for {
		// check for closed writer
		if _, err := w.Write([]byte{}); err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			timestamp := map[string]interface{}{
				"gt": since.UTC().Format(time.RFC3339),
			}

			if !follow {
				timestamp["lt"] = now.Format(time.RFC3339)
			}

			body := map[string]interface{}{
				"query": map[string]interface{}{
					"range": map[string]interface{}{
						"@timestamp": timestamp,
					},
				},
			}

			data, err := json.Marshal(body)
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				return
			}

			res, err := p.elastic.Search(
				p.elastic.Search.WithIndex(index),
				p.elastic.Search.WithSize(5000),
				p.elastic.Search.WithBody(bytes.NewReader(data)),
			)
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				return
			}
			defer res.Body.Close()

			data, err = ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Printf("err: %+v\n", err)
				return
			}

			var sres elasticSearchResult

			if err := json.Unmarshal(data, &sres); err != nil {
				fmt.Printf("err: %+v\n", err)
				return
			}

			sort.Slice(sres.Hits.Hits, func(i, j int) bool {
				return sres.Hits.Hits[i].Source.Timestamp.Before(sres.Hits.Hits[j].Source.Timestamp)
			})

			if len(sres.Hits.Hits) == 0 && !follow {
				return
			}

			for _, log := range sres.Hits.Hits {
				prefix := ""

				if common.DefaultBool(opts.Prefix, false) {
					prefix = fmt.Sprintf("%s %s ", log.Source.Timestamp.Format(time.RFC3339), strings.ReplaceAll(log.Source.Stream, ".", "/"))
				}

				fmt.Fprintf(w, "%s%s", prefix, log.Source.Log)

				since = log.Source.Timestamp
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"strings"
// 	"time"

// 	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
// 	"github.com/convox/convox/pkg/common"
// 	"github.com/convox/convox/pkg/options"
// 	"github.com/convox/convox/pkg/structs"
// )

// // var sequenceTokens sync.Map

// func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
// 	return nil
// }

// func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
// 	r, w := io.Pipe()

// 	go p.insightContainerLogs(p.Context(), w, p.AppNamespace(name), opts)

// 	return r, nil
// }

// func (p *Provider) SystemLogs(opts structs.LogsOptions) (io.ReadCloser, error) {
// 	return p.AppLogs("system", opts)
// }

// func (p *Provider) insightContainerLogs(ctx context.Context, w io.WriteCloser, namespace string, opts structs.LogsOptions) {
// 	defer w.Close()

// 	since := common.DefaultDuration(opts.Since, 0)
// 	start := time.Now().Add(-1 * since)

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		default:
// 			// check for closed writer
// 			if _, err := w.Write([]byte{}); err != nil {
// 				return
// 			}

// 			query := operationalinsights.QueryBody{
// 				Query:    options.String(fmt.Sprintf("KubePodInventory | join kind=innerunique ContainerLog on ContainerID | project Timestamp=TimeGenerated1,Message=LogEntry,Namespace,Pod=Name,Labels=PodLabel | where Namespace==%q and Timestamp > datetime(%s) | order by Timestamp asc | limit 100", namespace, start.Format("2006-01-02 15:04:05.000"))),
// 				Timespan: options.String("P7D"),
// 			}

// 			res, err := p.insightLogs.Execute(context.Background(), p.Workspace, query)
// 			if err != nil {
// 				fmt.Printf("err: %+v\n", err)
// 				return
// 			}
// 			if len(*res.Tables) < 1 {
// 				fmt.Println("no tables")
// 				return
// 			}

// 			t := (*res.Tables)[0]

// 			if len(*t.Rows) == 0 && !common.DefaultBool(opts.Follow, true) {
// 				return
// 			}

// 			for _, row := range *t.Rows {
// 				attrs := parseRow(row, *t.Columns)

// 				ts, err := time.Parse("2006-01-02T15:04:05.999Z", attrs["Timestamp"])
// 				if err != nil {
// 					fmt.Printf("err: %+v\n", err)
// 					continue
// 				}

// 				if ts.After(start) {
// 					start = ts
// 				}

// 				var labels map[string]string

// 				if err := json.Unmarshal([]byte(strings.Trim(attrs["Labels"], "[]")), &labels); err != nil {
// 					fmt.Printf("err: %+v\n", err)
// 					continue
// 				}

// 				service := labels["service"]
// 				pod := attrs["Pod"]

// 				prefix := ""

// 				if common.DefaultBool(opts.Prefix, false) {
// 					prefix = fmt.Sprintf("%s service/%s/%s ", ts.Format(time.RFC3339), service, pod)
// 				}

// 				if _, err := w.Write([]byte(fmt.Sprintf("%s%s\n", prefix, attrs["Message"]))); err != nil {
// 					fmt.Printf("err: %+v\n", err)
// 				}
// 			}

// 			time.Sleep(5 * time.Second)
// 		}
// 	}
// }

// func parseRow(row []interface{}, cols []operationalinsights.Column) map[string]string {
// 	attrs := map[string]string{}

// 	for i, c := range cols {
// 		if v, ok := row[i].(string); ok && c.Name != nil {
// 			attrs[*c.Name] = v
// 		}
// 	}

// 	return attrs
// }
