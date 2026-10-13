package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type routeES struct {
	mu         sync.Mutex
	indices    string
	allocation string
	settings   string
	deleteCode int
	requests   []string
}

func newRouteES(t *testing.T, indices, allocation, settings string) (*Retainer, *routeES) {
	f := &routeES{indices: indices, allocation: allocation, settings: settings, deleteCode: http.StatusOK}

	s := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(s.Close)

	c, err := New(s.URL)
	require.NoError(t, err)

	return c.Retainer("rack"), f
}

func (f *routeES) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	f.mu.Lock()
	defer f.mu.Unlock()

	f.requests = append(f.requests, strings.TrimSpace(fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, body)))

	switch {
	case strings.HasPrefix(r.URL.Path, "/_cat/indices"):
		fmt.Fprint(w, f.indices)
	case strings.HasPrefix(r.URL.Path, "/_cat/allocation"):
		fmt.Fprint(w, f.allocation)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/_settings/index.blocks.read_only_allow_delete"):
		fmt.Fprint(w, f.settings)
	case r.Method == http.MethodDelete:
		w.WriteHeader(f.deleteCode)
		fmt.Fprint(w, `{"acknowledged":true}`)
	default:
		fmt.Fprint(w, `{"acknowledged":true}`)
	}
}

func (f *routeES) writes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	ws := []string{}

	for _, r := range f.requests {
		if !strings.HasPrefix(r, "GET ") {
			ws = append(ws, r)
		}
	}

	return ws
}

const (
	unassigned  = `{"node":"UNASSIGNED","disk.percent":null}`
	rackIndices = `[
		{"index":"convox.rack.2026.10.11","store.size":"900"},
		{"index":"convox.rack.web","store.size":"5000"},
		{"index":"convox.rack.2fa","store.size":"7000"},
		{"index":"convox.rack.2026.10.13","store.size":"100"},
		{"index":"convox.rack.2026.10.12","store.size":"800"}
	]`
	unblocked = `{"convox.rack.web":{"settings":{}}}`
)

var noon = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func TestRetainOrder(t *testing.T) {
	r, es := newRouteES(t, rackIndices, `[{"node":"es-0","disk.percent":"86"},`+unassigned+`]`, unblocked)

	for i := 0; i < 5; i++ {
		require.NoError(t, r.Tick(context.Background(), noon))
	}

	deletes := []string{}

	for _, w := range es.writes() {
		if strings.HasPrefix(w, "DELETE ") {
			deletes = append(deletes, w)
		}
	}

	require.Equal(t, []string{"DELETE /convox.rack.2fa", "DELETE /convox.rack.2fa", "DELETE /convox.rack.2fa", "DELETE /convox.rack.2fa", "DELETE /convox.rack.2fa"}, deletes)

	cands := []string{}
	indices, err := r.indices(context.Background())
	require.NoError(t, err)

	for len(indices) > 0 {
		ix, ok := candidate(indices, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC))
		if !ok {
			break
		}
		cands = append(cands, ix.name)

		rest := []rackIndex{}
		for _, o := range indices {
			if o.name != ix.name {
				rest = append(rest, o)
			}
		}
		indices = rest
	}

	require.Equal(t, []string{"convox.rack.2fa", "convox.rack.web", "convox.rack.2026.10.11", "convox.rack.2026.10.12"}, cands)
}

func TestRetainTemplateOnce(t *testing.T) {
	r, es := newRouteES(t, `[]`, `[{"node":"es-0","disk.percent":"10"}]`, `{}`)

	require.NoError(t, r.Tick(context.Background(), noon))
	require.NoError(t, r.Tick(context.Background(), noon))

	puts := 0

	for _, w := range es.writes() {
		if strings.HasPrefix(w, "PUT /_template/convox ") {
			puts++
			require.Contains(t, w, `"number_of_shards": 1`)
			require.Contains(t, w, `"dynamic": false`)
		}
	}

	require.Equal(t, 1, puts)
}

func TestRetainBelowThreshold(t *testing.T) {
	for name, tt := range map[string]struct {
		settings string
		unblock  bool
	}{
		"Blocked":   {`{"convox.rack.web":{"settings":{"index":{"blocks":{"read_only_allow_delete":"true"}}}}}`, true},
		"Unblocked": {unblocked, false},
	} {
		t.Run(name, func(t *testing.T) {
			r, es := newRouteES(t, rackIndices, `[{"node":"es-0","disk.percent":"79"},{"node":"es-1","disk.percent":"40"}]`, tt.settings)

			require.NoError(t, r.Tick(context.Background(), noon))

			ws := es.writes()
			require.Equal(t, "PUT /_template/convox", strings.SplitN(ws[0], " {", 2)[0])

			if tt.unblock {
				require.Len(t, ws, 2)
				require.Equal(t, `PUT /convox.rack.*/_settings {"index.blocks.read_only_allow_delete": null}`, ws[1])
			} else {
				require.Len(t, ws, 1)
			}
		})
	}
}

func TestRetainDeleteNotFound(t *testing.T) {
	r, es := newRouteES(t, rackIndices, `[{"node":"es-0","disk.percent":"95"}]`, unblocked)
	es.deleteCode = http.StatusNotFound

	require.NoError(t, r.Tick(context.Background(), noon))
}

func TestRetainOnlyToday(t *testing.T) {
	r, es := newRouteES(t, `[{"index":"convox.rack.2026.10.13","store.size":"100"}]`, `[{"node":"es-0","disk.percent":"90"}]`, unblocked)

	require.NoError(t, r.Tick(context.Background(), noon))
	require.NoError(t, r.Tick(context.Background(), noon.Add(time.Minute)))

	for _, w := range es.writes() {
		require.False(t, strings.HasPrefix(w, "DELETE "), w)
	}
	require.Equal(t, noon, r.warned)
}

func TestRetainAgeCeiling(t *testing.T) {
	r, es := newRouteES(t, `[
		{"index":"convox.rack.2025.10.12","store.size":"1"},
		{"index":"convox.rack.2025.10.13","store.size":"1"},
		{"index":"convox.rack.2026.10.13","store.size":"1"}
	]`, `[{"node":"es-0","disk.percent":"20"}]`, unblocked)

	require.NoError(t, r.Tick(context.Background(), noon))

	deletes := []string{}

	for _, w := range es.writes() {
		if strings.HasPrefix(w, "DELETE ") {
			deletes = append(deletes, w)
		}
	}

	require.Equal(t, []string{"DELETE /convox.rack.2025.10.12"}, deletes)
}

func TestRetainAgeThenDisk(t *testing.T) {
	r, es := newRouteES(t, `[
		{"index":"convox.rack.2025.10.12","store.size":"1"},
		{"index":"convox.rack.2026.10.12","store.size":"1"},
		{"index":"convox.rack.2026.10.13","store.size":"1"}
	]`, `[{"node":"es-0","disk.percent":"90"}]`, unblocked)

	require.NoError(t, r.Tick(context.Background(), noon))

	deletes := []string{}

	for _, w := range es.writes() {
		if strings.HasPrefix(w, "DELETE ") {
			deletes = append(deletes, w)
		}
	}

	require.Equal(t, []string{"DELETE /convox.rack.2025.10.12", "DELETE /convox.rack.2026.10.12"}, deletes)
}

func TestRetainElasticsearch(t *testing.T) {
	addr := os.Getenv("ES_URL")
	if addr == "" {
		t.Skip("ES_URL not set")
	}

	c, err := New(addr)
	require.NoError(t, err)

	rack := fmt.Sprintf("t%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if res, err := c.client.Indices.Delete([]string{fmt.Sprintf("convox.%s.*", rack)}); err == nil {
			res.Body.Close()
		}
	})

	now := time.Now().UTC()

	bulk(t, c, appIndex(rack, "small"), "", nil, sequence(now.Add(-72*time.Hour), 0, 10)...)
	bulk(t, c, appIndex(rack, "big"), "", nil, sequence(now.Add(-72*time.Hour), 0, 3000)...)
	bulk(t, c, dayIndex(rack, now.Add(-48*time.Hour)), "web", nil, sequence(now.Add(-48*time.Hour), 0, 10)...)
	bulk(t, c, dayIndex(rack, now.Add(-24*time.Hour)), "web", nil, sequence(now.Add(-24*time.Hour), 0, 10)...)
	bulk(t, c, dayIndex(rack, now), "web", nil, sequence(now.Add(-time.Minute), 0, 10)...)

	require.NoError(t, check(c.client.Indices.Refresh()))

	r := c.Retainer(rack)

	defer func(n int) { retainThreshold = n }(retainThreshold)
	retainThreshold = 0

	left := func() []string {
		indices, err := r.indices(context.Background())
		require.NoError(t, err)

		names := []string{}
		for _, ix := range indices {
			names = append(names, ix.name)
		}
		return names
	}

	for _, gone := range []string{appIndex(rack, "big"), appIndex(rack, "small"), dayIndex(rack, now.Add(-48*time.Hour)), dayIndex(rack, now.Add(-24*time.Hour))} {
		require.NoError(t, r.Tick(context.Background(), now))
		require.NotContains(t, left(), gone)
	}

	require.NoError(t, r.Tick(context.Background(), now))
	require.Equal(t, []string{dayIndex(rack, now)}, left())

	require.NoError(t, check(c.client.Indices.PutSettings(
		bytes.NewReader([]byte(`{"index.blocks.read_only_allow_delete": true}`)),
		c.client.Indices.PutSettings.WithIndex(dayIndex(rack, now)),
	)))

	retainThreshold = 101
	require.NoError(t, r.Tick(context.Background(), now))

	res, err := c.client.Indices.GetSettings(c.client.Indices.GetSettings.WithIndex(dayIndex(rack, now)))
	require.NoError(t, err)
	defer res.Body.Close()

	var settings map[string]interface{}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&settings))
	require.NotContains(t, fmt.Sprint(settings), "read_only_allow_delete")
}
