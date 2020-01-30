package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/elastic/go-elasticsearch/v6"
)

type Client struct {
	client *elasticsearch.Client
}

type result struct {
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

func New(url string) (*Client, error) {
	ec, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{url},
	})
	if err != nil {
		return nil, err
	}

	c := &Client{
		client: ec,
	}

	return c, nil
}

func (c *Client) Stream(ctx context.Context, w io.WriteCloser, index string, opts structs.LogsOptions) {
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

			res, err := c.client.Search(
				c.client.Search.WithIndex(index),
				c.client.Search.WithSize(5000),
				c.client.Search.WithBody(bytes.NewReader(data)),
			)
			if err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
				return
			}
			defer res.Body.Close()

			data, err = ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
				return
			}

			var sres result

			if err := json.Unmarshal(data, &sres); err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
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

func (c *Client) Write(index string, ts time.Time, message string, tags map[string]string) error {
	body := map[string]interface{}{
		"log":        fmt.Sprintf("%s\n", message),
		"@timestamp": ts.Format(time.RFC3339Nano),
	}

	for k, v := range tags {
		body[k] = v
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	if _, err := c.client.Index(index, bytes.NewReader(data)); err != nil {
		return err
	}

	return nil
}
