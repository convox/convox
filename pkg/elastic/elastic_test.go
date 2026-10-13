package elastic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

const fluentdTime = "2006-01-02T15:04:05.000000000-07:00"

type logLine struct {
	id string
	ts time.Time
}

type esPage struct {
	status int
	body   string
}

type esRequest struct {
	method string
	path   string
	query  url.Values
	body   map[string]interface{}
}

type fakeES struct {
	mu       sync.Mutex
	pages    []esPage
	requests []esRequest
	conns    int32
}

func newFakeES(t *testing.T, pages ...esPage) (*Client, *fakeES) {
	f := &fakeES{pages: pages}

	s := httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	s.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			atomic.AddInt32(&f.conns, 1)
		}
	}
	s.Start()
	t.Cleanup(s.Close)

	c, err := New(s.URL)
	require.NoError(t, err)

	return c, f
}

func (f *fakeES) serve(w http.ResponseWriter, r *http.Request) {
	req := esRequest{method: r.Method, path: r.URL.Path, query: r.URL.Query()}

	if data, err := io.ReadAll(r.Body); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &req.body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.requests = append(f.requests, req)

	if r.Method == http.MethodDelete {
		fmt.Fprint(w, `{"succeeded":true,"num_freed":5}`)
		return
	}

	p := page("s0", 0)

	if len(f.pages) > 0 {
		p, f.pages = f.pages[0], f.pages[1:]
	}

	w.WriteHeader(p.status)
	fmt.Fprint(w, p.body)
}

func (f *fakeES) recorded() []esRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]esRequest{}, f.requests...)
}

func (f *fakeES) searches() []esRequest {
	rs := []esRequest{}

	for _, r := range f.recorded() {
		if strings.HasPrefix(r.path, "/convox.rack.app,") && strings.HasSuffix(r.path, "/_search") {
			rs = append(rs, r)
		}
	}

	return rs
}

func page(scrollID string, failed int, lines ...logLine) esPage {
	hits := []map[string]interface{}{}

	for _, l := range lines {
		hits = append(hits, map[string]interface{}{
			"_index": "convox.rack.app",
			"_type":  "_doc",
			"_id":    l.id,
			"_source": map[string]string{
				"log":        l.id + "\n",
				"stream":     "service.web.web-1",
				"@timestamp": l.ts.Format(fluentdTime),
			},
		})
	}

	data, err := json.Marshal(map[string]interface{}{
		"_scroll_id": scrollID,
		"_shards":    map[string]int{"total": 5, "successful": 5 - failed, "failed": failed},
		"hits":       map[string]interface{}{"total": len(lines), "hits": hits},
	})
	if err != nil {
		panic(err)
	}

	return esPage{status: http.StatusOK, body: string(data)}
}

func sequence(base time.Time, from, to int) []logLine {
	ls := []logLine{}

	for i := from; i < to; i++ {
		ls = append(ls, logLine{id: fmt.Sprintf("seq-%05d", i), ts: base.Add(time.Duration(i) * time.Millisecond)})
	}

	return ls
}

func ids(ls []logLine) []string {
	s := []string{}

	for _, l := range ls {
		s = append(s, l.id)
	}

	return s
}

func stream(c *Client, rack, app string, opts structs.LogsOptions) (chan string, io.Closer, chan struct{}) {
	r, w := io.Pipe()
	done := make(chan struct{})
	lines := make(chan string, 10000)

	go func() {
		c.Stream(context.Background(), w, rack, app, opts)
		close(done)
	}()

	go func() {
		s := bufio.NewScanner(r)

		for s.Scan() {
			lines <- s.Text()
		}

		close(lines)
	}()

	return lines, r, done
}

func collect(lines chan string) []string {
	s := []string{}

	for l := range lines {
		s = append(s, l)
	}

	return s
}

func next(t *testing.T, lines chan string) string {
	t.Helper()

	select {
	case l, ok := <-lines:
		require.True(t, ok, "stream closed")
		return l
	case <-time.After(15 * time.Second):
		require.FailNow(t, "timed out waiting for a line")
		return ""
	}
}

func timestampRange(t *testing.T, r esRequest) map[string]interface{} {
	t.Helper()

	q, ok := r.body["query"].(map[string]interface{})
	require.True(t, ok)
	b, ok := q["bool"].(map[string]interface{})
	require.True(t, ok)
	f, ok := b["filter"].([]interface{})
	require.True(t, ok)
	first, ok := f[0].(map[string]interface{})
	require.True(t, ok)
	rg, ok := first["range"].(map[string]interface{})
	require.True(t, ok)
	ts, ok := rg["@timestamp"].(map[string]interface{})
	require.True(t, ok)

	return ts
}

func noFollow(since time.Duration) structs.LogsOptions {
	return structs.LogsOptions{Follow: options.Bool(false), Since: options.Duration(since)}
}

func TestStreamNoFollowReadsEveryPage(t *testing.T) {
	base := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	all := sequence(base, 0, 6000)

	c, es := newFakeES(t, page("s1", 0, all[:5000]...), page("s1", 0, all[5000:]...))

	start := time.Now()
	lines, _, done := stream(c, "rack", "app", noFollow(time.Hour))

	require.Equal(t, ids(all), collect(lines))
	<-done

	rs := es.recorded()
	require.Len(t, rs, 3)

	require.True(t, strings.HasPrefix(rs[0].path, "/convox.rack.app,convox.rack."), rs[0].path)
	require.True(t, strings.HasSuffix(rs[0].path, "/_search"), rs[0].path)
	require.Equal(t, "true", rs[0].query.Get("ignore_unavailable"))
	require.Equal(t, "true", rs[0].query.Get("allow_no_indices"))
	require.Equal(t, "300000ms", rs[0].query.Get("scroll"))
	require.Equal(t, "5000", rs[0].query.Get("size"))
	require.Equal(t, []interface{}{map[string]interface{}{"@timestamp": "asc"}}, rs[0].body["sort"])

	ts := timestampRange(t, rs[0])

	bound := func(key string) time.Time {
		s, ok := ts[key].(string)
		require.True(t, ok)
		v, err := time.Parse(time.RFC3339Nano, s)
		require.NoError(t, err)
		return v
	}

	require.WithinDuration(t, start.Add(-time.Hour), bound("gte"), 5*time.Second)
	require.WithinDuration(t, start, bound("lt"), 5*time.Second)

	require.Equal(t, "/_search/scroll", rs[1].path)
	require.Empty(t, rs[1].query.Get("scroll_id"))
	require.Equal(t, "s1", rs[1].body["scroll_id"])
	require.Equal(t, "300000ms", rs[1].query.Get("scroll"))

	require.Equal(t, http.MethodDelete, rs[2].method)
	require.Equal(t, "/_search/scroll", rs[2].path)
	require.Equal(t, []interface{}{"s1"}, rs[2].body["scroll_id"])
}

func TestStreamFollowPrintsLateLinesOnce(t *testing.T) {
	second := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Second)
	first := logLine{id: "first", ts: second.Add(100*time.Millisecond + 123456*time.Nanosecond)}
	older := logLine{id: "older", ts: second.Add(50 * time.Millisecond)}
	later := logLine{id: "later", ts: second.Add(900 * time.Millisecond)}

	c, es := newFakeES(t,
		page("s1", 0, first),
		page("s2", 0, older, first),
		page("s3", 0, older, first, later),
	)

	lines, r, done := stream(c, "rack", "app", structs.LogsOptions{Since: options.Duration(time.Minute)})

	require.Equal(t, "first", next(t, lines))
	require.Equal(t, "older", next(t, lines))
	require.Equal(t, "later", next(t, lines))

	r.Close()
	<-done

	floor := first.ts.Add(-5 * time.Second).Truncate(time.Millisecond).Format(time.RFC3339Nano)

	ss := es.searches()
	require.GreaterOrEqual(t, len(ss), 3)

	gte, ok := timestampRange(t, ss[0])["gte"].(string)
	require.True(t, ok)
	since, err := time.Parse(time.RFC3339Nano, gte)
	require.NoError(t, err)
	require.Equal(t, since.Truncate(time.Millisecond), since)

	require.Equal(t, floor, timestampRange(t, ss[1])["gte"])
	require.Equal(t, floor, timestampRange(t, ss[2])["gte"])
	require.NotContains(t, timestampRange(t, ss[1]), "lt")

	clears := 0

	for _, r := range es.recorded() {
		if r.method == http.MethodDelete {
			clears++
		}
	}

	require.GreaterOrEqual(t, clears, 3)
	require.Equal(t, int32(1), atomic.LoadInt32(&es.conns))
}

func TestStreamOrdersWithinMillisecond(t *testing.T) {
	ms := time.Date(2026, 10, 4, 12, 0, 0, 123000000, time.UTC)
	lines := []logLine{
		{id: "trace-2", ts: ms.Add(40 * time.Microsecond)},
		{id: "trace-0", ts: ms.Add(10 * time.Microsecond)},
		{id: "trace-1", ts: ms.Add(20 * time.Microsecond)},
	}

	c, _ := newFakeES(t, page("s1", 0, lines...))

	out, _, done := stream(c, "rack", "app", noFollow(time.Hour))

	require.Equal(t, []string{"trace-0", "trace-1", "trace-2"}, collect(out))
	<-done
}

func TestWriteReusesConnection(t *testing.T) {
	c, es := newFakeES(t)

	for i := 0; i < 3; i++ {
		require.NoError(t, c.Write("rack", "app", time.Date(2026, 10, 13, 23, 30, 0, 0, time.FixedZone("x", -2*3600)), "event", map[string]string{"stream": "system"}))
	}

	rs := es.recorded()
	require.Len(t, rs, 3)
	require.Equal(t, "/convox.rack.2026.10.14/_doc", rs[0].path)
	require.Equal(t, "app", rs[0].body["app"])
	require.Equal(t, "system", rs[0].body["stream"])
	require.Equal(t, int32(1), atomic.LoadInt32(&es.conns))
}

func TestStreamMissingIndex(t *testing.T) {
	c, es := newFakeES(t, esPage{status: http.StatusNotFound, body: `{"error":{"type":"index_not_found_exception"},"status":404}`})

	lines, _, done := stream(c, "rack", "app", noFollow(time.Minute))

	require.Empty(t, collect(lines))
	<-done
	require.Len(t, es.recorded(), 1)
}

func TestStreamClearsEmptyScroll(t *testing.T) {
	c, es := newFakeES(t, page("s9", 0))

	lines, _, done := stream(c, "rack", "app", noFollow(time.Minute))

	require.Empty(t, collect(lines))
	<-done

	rs := es.recorded()
	require.Len(t, rs, 2)
	require.Equal(t, http.MethodDelete, rs[1].method)
	require.Equal(t, []interface{}{"s9"}, rs[1].body["scroll_id"])
}

func TestStreamScrollFailure(t *testing.T) {
	all := sequence(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), 0, 5010)

	for name, failure := range map[string]esPage{
		"Expired":      {status: http.StatusNotFound, body: `{"error":{"type":"search_context_missing_exception"},"status":404}`},
		"ShardsFailed": page("s1", 1, all[5000:]...),
	} {
		t.Run(name, func(t *testing.T) {
			c, es := newFakeES(t, page("s1", 0, all[:5000]...), failure)

			lines, _, done := stream(c, "rack", "app", noFollow(time.Hour))

			got := collect(lines)
			<-done

			require.Len(t, got, 5001)
			require.Equal(t, ids(all[:5000]), got[:5000])
			require.True(t, strings.HasPrefix(got[5000], "error: "), got[5000])

			rs := es.recorded()
			require.Equal(t, http.MethodDelete, rs[len(rs)-1].method)
			require.Equal(t, []interface{}{"s1"}, rs[len(rs)-1].body["scroll_id"])
		})
	}
}

func TestStreamStopsWhenReaderCloses(t *testing.T) {
	all := sequence(time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC), 0, 6000)

	c, es := newFakeES(t, page("s1", 0, all[:5000]...), page("s1", 0, all[5000:]...))

	r, w := io.Pipe()
	done := make(chan struct{})

	go func() {
		c.Stream(context.Background(), w, "rack", "app", noFollow(time.Hour))
		close(done)
	}()

	br := bufio.NewReader(r)

	for i := 0; i < 10; i++ {
		_, err := br.ReadString('\n')
		require.NoError(t, err)
	}

	r.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "stream did not stop")
	}

	rs := es.recorded()
	require.Len(t, rs, 2)
	require.Equal(t, http.MethodDelete, rs[1].method)
	require.Equal(t, []interface{}{"s1"}, rs[1].body["scroll_id"])
}

func bulk(t *testing.T, c *Client, index, app string, labels map[string]string, lines ...logLine) {
	t.Helper()

	var buf bytes.Buffer

	for _, l := range lines {
		fmt.Fprintf(&buf, "{\"index\":{\"_index\":%q,\"_type\":\"_doc\"}}\n", index)

		doc := map[string]interface{}{"log": l.id + "\n", "stream": "service.web.web-1", "@timestamp": l.ts.Format(fluentdTime)}
		if app != "" {
			doc["app"] = app
		}
		if labels != nil {
			doc["kubernetes"] = map[string]interface{}{"labels": labels}
		}

		data, err := json.Marshal(doc)
		require.NoError(t, err)

		buf.Write(data)
		buf.WriteByte('\n')
	}

	res, err := c.client.Bulk(&buf, c.client.Bulk.WithRefresh("true"))
	require.NoError(t, err)
	defer res.Body.Close()

	var body struct {
		Errors bool
	}

	require.False(t, res.IsError())
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	require.False(t, body.Errors)
}

func TestStreamIndices(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 30, 0, 0, time.UTC)

	require.Equal(t, []string{"convox.rack.app", "convox.rack.2026.10.12", "convox.rack.2026.10.13"},
		streamIndices("rack", "app", now.Add(-time.Hour), now))
	require.Equal(t, []string{"convox.rack.app", "convox.rack.2026.10.10", "convox.rack.2026.10.11", "convox.rack.2026.10.12", "convox.rack.2026.10.13"},
		streamIndices("rack", "app", now.Add(-72*time.Hour), now))
	require.Equal(t, []string{"convox.rack.app", "convox.rack.2*"}, streamIndices("rack", "app", now.Add(-200*24*time.Hour), now))
	require.Equal(t, []string{"convox.rack.app", "convox.rack.2*"}, streamIndices("rack", "app", time.Time{}, now))
	require.Equal(t, []string{"convox.rack.app", "convox.rack.2026.10.13"},
		streamIndices("rack", "app", time.Date(2026, 10, 12, 21, 0, 0, 0, time.FixedZone("x", -4*3600)), now))
}

func TestStreamFirstSearchError(t *testing.T) {
	c, _ := newFakeES(t, esPage{status: http.StatusBadRequest, body: `{"error":{"type":"too_long_frame_exception"},"status":400}`})

	lines, _, done := stream(c, "rack", "app", noFollow(time.Hour))

	got := collect(lines)
	<-done

	require.Len(t, got, 1)
	require.True(t, strings.HasPrefix(got[0], "error: "), got[0])
	require.Contains(t, got[0], "too_long_frame_exception")
}

func TestStreamFiltersByApp(t *testing.T) {
	c, es := newFakeES(t, page("s1", 0))

	_, _, done := stream(c, "rack", "app", noFollow(time.Hour))
	<-done

	rs := es.recorded()
	require.NotEmpty(t, rs)

	data, err := json.Marshal(rs[0].body["query"])
	require.NoError(t, err)
	require.Contains(t, string(data), `{"term":{"_index":"convox.rack.app"}}`)
	require.Contains(t, string(data), `{"term":{"app.keyword":"app"}}`)
	require.Contains(t, string(data), `"minimum_should_match":1`)
}

func TestStreamElasticsearch(t *testing.T) {
	addr := os.Getenv("ES_URL")
	if addr == "" {
		t.Skip("ES_URL not set")
	}

	c, err := New(addr)
	require.NoError(t, err)

	rack := func(t *testing.T) string {
		name := fmt.Sprintf("t%d", time.Now().UnixNano())

		t.Cleanup(func() {
			if res, err := c.client.Indices.Delete([]string{fmt.Sprintf("convox.%s.*", name)}); err == nil {
				res.Body.Close()
			}
		})

		return name
	}

	require.NoError(t, check(c.client.Indices.PutTemplate("convox", bytes.NewReader(indexTemplate))))

	t.Run("NoFollow", func(t *testing.T) {
		r := rack(t)
		now := time.Now().UTC()
		all := sequence(now.Add(-time.Minute), 0, 6000)

		bulk(t, c, dayIndex(r, now), "web", nil, all...)

		lines, _, done := stream(c, r, "web", noFollow(2*time.Minute))

		require.Equal(t, ids(all), collect(lines))
		<-done
	})

	t.Run("Follow", func(t *testing.T) {
		r := rack(t)
		second := time.Now().UTC().Truncate(time.Second)
		first := logLine{id: "first", ts: second.Add(100 * time.Millisecond)}

		bulk(t, c, dayIndex(r, second), "web", nil, first)

		lines, rc, done := stream(c, r, "web", structs.LogsOptions{Since: options.Duration(10 * time.Second)})

		require.Equal(t, "first", next(t, lines))

		bulk(t, c, dayIndex(r, second), "web", nil,
			logLine{id: "later", ts: second.Add(900 * time.Millisecond)},
			logLine{id: "older", ts: second.Add(50 * time.Millisecond)},
		)

		require.ElementsMatch(t, []string{"older", "later"}, []string{next(t, lines), next(t, lines)})

		select {
		case l := <-lines:
			require.FailNow(t, "printed again", l)
		case <-time.After(2500 * time.Millisecond):
		}

		rc.Close()
		<-done
	})

	t.Run("BothLayoutsAndAppsApart", func(t *testing.T) {
		r := rack(t)
		now := time.Now().UTC()
		day := now.Add(-48 * time.Hour)

		bulk(t, c, appIndex(r, "web"), "", nil, logLine{id: "web-old", ts: now.Add(-50 * time.Hour)})
		bulk(t, c, appIndex(r, "api"), "", nil, logLine{id: "api-old", ts: now.Add(-50 * time.Hour)})
		bulk(t, c, dayIndex(r, day), "web", nil, logLine{id: "web-day", ts: day})
		bulk(t, c, dayIndex(r, day), "api", nil, logLine{id: "api-day", ts: day.Add(time.Millisecond)})
		bulk(t, c, dayIndex(r, now), "web", nil, logLine{id: "web-now", ts: now.Add(-time.Second)})

		lines, _, done := stream(c, r, "web", noFollow(72*time.Hour))

		require.Equal(t, []string{"web-old", "web-day", "web-now"}, collect(lines))
		<-done
	})

	t.Run("ConflictingLabelsSameDay", func(t *testing.T) {
		r := rack(t)
		now := time.Now().UTC()

		bulk(t, c, dayIndex(r, now), "one", map[string]string{"version": "2026-10-01", "team": "x"}, logLine{id: "one", ts: now.Add(-2 * time.Second)})
		bulk(t, c, dayIndex(r, now), "two", map[string]string{"version": "v1.2", "team.example.com/owner": "y"}, logLine{id: "two", ts: now.Add(-time.Second)})

		lines, _, done := stream(c, r, "two", noFollow(time.Minute))

		require.Equal(t, []string{"two"}, collect(lines))
		<-done
	})

	t.Run("ManyDailyIndicesAndPages", func(t *testing.T) {
		defer func(n int) { pageSize = n }(pageSize)
		pageSize = 50

		r := rack(t)
		now := time.Now().UTC()
		want := []string{}

		for d := 119; d >= 0; d-- {
			ts := now.Add(-time.Duration(d) * 24 * time.Hour).Add(-time.Minute)
			id := fmt.Sprintf("day-%03d", 119-d)
			bulk(t, c, dayIndex(r, ts), "web", nil, logLine{id: id, ts: ts})
			want = append(want, id)
		}

		lines, _, done := stream(c, r, "web", noFollow(121*24*time.Hour))

		require.Equal(t, want, collect(lines))
		<-done
	})
}
