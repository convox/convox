package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/elastic/go-elasticsearch/v6/esapi"
)

const retainMaxAge = 365 * 24 * time.Hour

var retainThreshold = 80

var indexTemplate = []byte(`{
	"index_patterns": ["convox.*"],
	"settings": {"number_of_shards": 1},
	"mappings": {"_doc": {
		"dynamic": false,
		"date_detection": false,
		"properties": {
			"@timestamp": {"type": "date"},
			"app": {"type": "text", "fields": {"keyword": {"type": "keyword", "ignore_above": 256}}}
		}
	}}
}`)

type Retainer struct {
	client    *Client
	rack      string
	daily     *regexp.Regexp
	templated bool
	warned    time.Time
}

type rackIndex struct {
	name  string
	bytes int64
	day   time.Time
}

func (c *Client) Retainer(rack string) *Retainer {
	return &Retainer{
		client: c,
		rack:   rack,
		daily:  regexp.MustCompile(`^convox\.` + regexp.QuoteMeta(rack) + `\.(\d{4}\.\d{2}\.\d{2})$`),
	}
}

func (r *Retainer) Tick(ctx context.Context, now time.Time) error {
	es := r.client.client

	if !r.templated {
		if err := check(es.Indices.PutTemplate("convox", bytes.NewReader(indexTemplate), es.Indices.PutTemplate.WithContext(ctx))); err != nil {
			return err
		}
		r.templated = true
	}

	indices, err := r.indices(ctx)
	if err != nil {
		return err
	}

	today := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)

	kept := []rackIndex{}

	for _, ix := range indices {
		if ix.day.IsZero() || today.Sub(ix.day) <= retainMaxAge {
			kept = append(kept, ix)
			continue
		}

		if err := r.delete(ctx, ix, "age"); err != nil {
			return err
		}
	}

	used, err := r.diskPercent(ctx)
	if err != nil {
		return err
	}

	if used < retainThreshold {
		return r.unblock(ctx)
	}

	if ix, ok := candidate(kept, today); ok {
		return r.delete(ctx, ix, fmt.Sprintf("disk=%d%%", used))
	}

	if now.Sub(r.warned) >= time.Hour {
		fmt.Printf("ns=elastic at=retain warning=%q disk=%d%%\n", "only today's logs are left to delete", used)
		r.warned = now
	}

	return nil
}

func candidate(indices []rackIndex, today time.Time) (rackIndex, bool) {
	cs := []rackIndex{}

	for _, ix := range indices {
		if ix.day.IsZero() || ix.day.Before(today) {
			cs = append(cs, ix)
		}
	}

	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].day.IsZero() != cs[j].day.IsZero() {
			return cs[i].day.IsZero()
		}
		if cs[i].day.IsZero() {
			return cs[i].bytes > cs[j].bytes
		}
		return cs[i].day.Before(cs[j].day)
	})

	if len(cs) == 0 {
		return rackIndex{}, false
	}

	return cs[0], true
}

func (r *Retainer) indices(ctx context.Context) ([]rackIndex, error) {
	es := r.client.client

	var rows []struct {
		Index string `json:"index"`
		Size  string `json:"store.size"`
	}

	if err := decodeJSON(es.Cat.Indices(
		es.Cat.Indices.WithIndex(fmt.Sprintf("convox.%s.*", r.rack)),
		es.Cat.Indices.WithH("index", "store.size"),
		es.Cat.Indices.WithBytes("b"),
		es.Cat.Indices.WithFormat("json"),
		es.Cat.Indices.WithContext(ctx),
	))(&rows); err != nil {
		return nil, err
	}

	indices := []rackIndex{}

	for _, row := range rows {
		ix := rackIndex{name: row.Index}
		ix.bytes, _ = strconv.ParseInt(row.Size, 10, 64)

		if m := r.daily.FindStringSubmatch(row.Index); m != nil {
			day, err := time.Parse(dayFormat, m[1])
			if err != nil {
				continue
			}
			ix.day = day
		}

		indices = append(indices, ix)
	}

	return indices, nil
}

func (r *Retainer) diskPercent(ctx context.Context) (int, error) {
	es := r.client.client

	var rows []struct {
		Node    string  `json:"node"`
		Percent *string `json:"disk.percent"`
	}

	if err := decodeJSON(es.Cat.Allocation(
		es.Cat.Allocation.WithH("node", "disk.percent"),
		es.Cat.Allocation.WithFormat("json"),
		es.Cat.Allocation.WithContext(ctx),
	))(&rows); err != nil {
		return 0, err
	}

	used := 0

	for _, row := range rows {
		if row.Node == "UNASSIGNED" || row.Percent == nil {
			continue
		}

		if p, err := strconv.Atoi(*row.Percent); err == nil && p > used {
			used = p
		}
	}

	return used, nil
}

func (r *Retainer) delete(ctx context.Context, ix rackIndex, reason string) error {
	es := r.client.client

	res, err := es.Indices.Delete([]string{ix.name}, es.Indices.Delete.WithContext(ctx))
	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusNotFound {
		drain(res)
		return nil
	}

	if err := check(res, nil); err != nil {
		return err
	}

	fmt.Printf("ns=elastic at=retain index=%s bytes=%d reason=%s\n", ix.name, ix.bytes, reason)

	return nil
}

func (r *Retainer) unblock(ctx context.Context) error {
	es := r.client.client
	pattern := fmt.Sprintf("convox.%s.*", r.rack)

	var settings map[string]struct {
		Settings struct {
			Index struct {
				Blocks struct {
					ReadOnlyAllowDelete string `json:"read_only_allow_delete"`
				} `json:"blocks"`
			} `json:"index"`
		} `json:"settings"`
	}

	if err := decodeJSON(es.Indices.GetSettings(
		es.Indices.GetSettings.WithIndex(pattern),
		es.Indices.GetSettings.WithName("index.blocks.read_only_allow_delete"),
		es.Indices.GetSettings.WithContext(ctx),
	))(&settings); err != nil {
		return err
	}

	blocked := false

	for _, s := range settings {
		if s.Settings.Index.Blocks.ReadOnlyAllowDelete == "true" {
			blocked = true
		}
	}

	if !blocked {
		return nil
	}

	if err := check(es.Indices.PutSettings(
		bytes.NewReader([]byte(`{"index.blocks.read_only_allow_delete": null}`)),
		es.Indices.PutSettings.WithIndex(pattern),
		es.Indices.PutSettings.WithContext(ctx),
	)); err != nil {
		return err
	}

	fmt.Printf("ns=elastic at=unblock indices=%s\n", pattern)

	return nil
}

func check(res *esapi.Response, err error) error {
	if err != nil {
		return err
	}

	if res.IsError() {
		return errors.New(res.String())
	}

	drain(res)

	return nil
}

func decodeJSON(res *esapi.Response, err error) func(interface{}) error {
	return func(v interface{}) error {
		if err != nil {
			return err
		}

		if res.IsError() {
			return errors.New(res.String())
		}

		defer res.Body.Close()

		data, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		return json.Unmarshal(data, v)
	}
}
