package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
)

const (
	pageSize  = 5000
	keepAlive = 5 * time.Minute
	overlap   = 5 * time.Second
)

type Client struct {
	client *elasticsearch.Client
}

type result struct {
	ScrollID string `json:"_scroll_id"`
	Shards   struct {
		Failed int
	} `json:"_shards"`
	Hits struct {
		Hits []struct {
			ID     string `json:"_id"`
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
	floor := time.Time{}

	if opts.Since != nil {
		floor = now.Add(*opts.Since * -1).Truncate(time.Millisecond)
	}

	timestamp := map[string]interface{}{}

	if !follow {
		timestamp["lt"] = now.Format(time.RFC3339Nano)
	}

	body := map[string]interface{}{
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": timestamp,
			},
		},
		"sort": []map[string]string{{"@timestamp": "asc"}},
	}

	seen := map[string]time.Time{}

	for {
		// check for closed writer
		if _, err := w.Write([]byte{}); err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		timestamp["gte"] = floor.Format(time.RFC3339Nano)

		err := c.search(index, body, func(sres *result) error {
			sort.SliceStable(sres.Hits.Hits, func(i, j int) bool {
				return sres.Hits.Hits[i].Source.Timestamp.Before(sres.Hits.Hits[j].Source.Timestamp)
			})

			for _, log := range sres.Hits.Hits {
				if _, ok := seen[log.ID]; ok {
					continue
				}

				seen[log.ID] = log.Source.Timestamp

				if f := log.Source.Timestamp.Add(-overlap).Truncate(time.Millisecond); f.After(floor) {
					floor = f
				}

				prefix := ""

				if common.DefaultBool(opts.Prefix, false) {
					prefix = fmt.Sprintf("%s %s ", log.Source.Timestamp.Format(time.RFC3339), strings.ReplaceAll(log.Source.Stream, ".", "/"))
				}

				if _, err := fmt.Fprintf(w, "%s%s", prefix, log.Source.Log); err != nil {
					return err
				}
			}

			for id, ts := range seen {
				if ts.Before(floor) {
					delete(seen, id)
				}
			}

			return nil
		})
		if err != nil {
			fmt.Fprintf(w, "error: %v\n", err)
			return
		}

		if !follow {
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *Client) search(index string, body map[string]interface{}, fn func(*result) error) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	res, err := c.client.Search(
		c.client.Search.WithIndex(index),
		c.client.Search.WithSize(pageSize),
		c.client.Search.WithScroll(keepAlive),
		c.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return err
	}

	sres, err := decode(res)
	if err != nil {
		return err
	}

	scrollID := sres.ScrollID

	defer func() {
		if scrollID != "" {
			c.clearScroll(scrollID)
		}
	}()

	for {
		if err := fn(sres); err != nil {
			return err
		}

		if len(sres.Hits.Hits) < pageSize {
			return nil
		}

		res, err := c.client.Scroll(
			c.client.Scroll.WithScrollID(scrollID),
			c.client.Scroll.WithScroll(keepAlive),
		)
		if err != nil {
			return err
		}

		if res.IsError() {
			return errors.New(res.String())
		}

		if sres, err = decode(res); err != nil {
			return err
		}

		if sres.Shards.Failed > 0 {
			return fmt.Errorf("%d shards failed", sres.Shards.Failed)
		}

		if sres.ScrollID != "" {
			scrollID = sres.ScrollID
		}
	}
}

func (c *Client) clearScroll(id string) {
	if res, err := c.client.ClearScroll(c.client.ClearScroll.WithScrollID(id)); err == nil {
		drain(res)
	}
}

func drain(res *esapi.Response) {
	_, _ = io.Copy(io.Discard, res.Body)
	res.Body.Close()
}

func decode(res *esapi.Response) (*result, error) {
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var sres result

	if err := json.Unmarshal(data, &sres); err != nil {
		return nil, err
	}

	return &sres, nil
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

	res, err := c.client.Index(index, bytes.NewReader(data))
	if err != nil {
		return err
	}

	drain(res)

	return nil
}
