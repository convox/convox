package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
)

const (
	keepAlive  = 5 * time.Minute
	overlap    = 5 * time.Second
	dayFormat  = "2006.01.02"
	listedDays = 31
)

var pageSize = 5000

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

func appIndex(rack, app string) string {
	return fmt.Sprintf("convox.%s.%s", rack, app)
}

func dayIndex(rack string, t time.Time) string {
	return fmt.Sprintf("convox.%s.%s", rack, t.UTC().Format(dayFormat))
}

// streamIndices lists the per-app index from before daily indices and the daily indices from floor's day
// through now's, falling back to a wildcard when the list would not fit in a request line.
func streamIndices(rack, app string, floor, now time.Time) []string {
	indices := []string{appIndex(rack, app)}

	from := floor.UTC()
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)

	if floor.IsZero() || now.Sub(day) > listedDays*24*time.Hour {
		return append(indices, fmt.Sprintf("convox.%s.2*", rack))
	}

	for ; !day.After(now); day = day.AddDate(0, 0, 1) {
		indices = append(indices, dayIndex(rack, day))
	}

	return indices
}

func (c *Client) Stream(ctx context.Context, w io.WriteCloser, rack, app string, opts structs.LogsOptions) {
	defer w.Close()

	tail := -1
	if opts.Tail != nil {
		tail = *opts.Tail
	}

	if tail > structs.LogsTailMax {
		fmt.Fprintf(w, "error: tail cannot be more than %d lines\n", structs.LogsTailMax)
		return
	}

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
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{"range": map[string]interface{}{"@timestamp": timestamp}},
					map[string]interface{}{"bool": map[string]interface{}{
						"should": []interface{}{
							map[string]interface{}{"term": map[string]interface{}{"_index": appIndex(rack, app)}},
							map[string]interface{}{"term": map[string]interface{}{"app.keyword": app}},
						},
						"minimum_should_match": 1,
					}},
				},
			},
		},
		"sort": []map[string]string{{"@timestamp": "asc"}},
	}

	seen := map[string]time.Time{}
	holding := tail >= 0
	held := []string{}

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

		err := c.search(streamIndices(rack, app, floor, time.Now()), body, func(sres *result) error {
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

				if holding {
					held = append(held, prefix+log.Source.Log)
					if len(held) > tail {
						held = held[len(held)-tail:]
					}
					continue
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

		if holding {
			fmt.Fprint(w, strings.Join(held, ""))
			holding = false
			held = nil
		}

		if !follow {
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *Client) search(indices []string, body map[string]interface{}, fn func(*result) error) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	res, err := c.client.Search(
		c.client.Search.WithIndex(indices...),
		c.client.Search.WithIgnoreUnavailable(true),
		c.client.Search.WithAllowNoIndices(true),
		c.client.Search.WithSize(pageSize),
		c.client.Search.WithScroll(keepAlive),
		c.client.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusNotFound {
		drain(res)
		return nil
	}

	if res.IsError() {
		return errors.New(res.String())
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
			c.client.Scroll.WithBody(scrollBody(scrollID)),
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

func scrollBody(id string) io.Reader {
	data, _ := json.Marshal(map[string]string{"scroll_id": id})
	return bytes.NewReader(data)
}

func (c *Client) clearScroll(id string) {
	data, _ := json.Marshal(map[string][]string{"scroll_id": {id}})

	if res, err := c.client.ClearScroll(c.client.ClearScroll.WithBody(bytes.NewReader(data))); err == nil {
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

func (c *Client) Write(rack, app string, ts time.Time, message string, tags map[string]string) error {
	body := map[string]interface{}{
		"log":        fmt.Sprintf("%s\n", message),
		"@timestamp": ts.Format(time.RFC3339Nano),
	}

	for k, v := range tags {
		body[k] = v
	}

	body["app"] = app

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	res, err := c.client.Index(dayIndex(rack, ts), bytes.NewReader(data))
	if err != nil {
		return err
	}

	drain(res)

	return nil
}
