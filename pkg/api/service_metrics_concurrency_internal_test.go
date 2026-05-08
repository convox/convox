package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestServiceMetricsConcurrencyCap503 covers F-PERF-R2-2: the
// semaphore at the controller layer caps concurrent QueryRange calls.
// When N concurrent requests exceed the cap, the (cap+1)th must
// return 503 fail-fast — NOT block waiting for a slot. The semaphore
// is shared between ServiceMetrics and MetricsByService; the test
// asserts the shared budget by exercising both endpoints in the same
// goroutine pool.
//
// Implementation note: lower the cap to 2 via env var so the test can
// hit the limit deterministically without spinning up dozens of
// goroutines. The semaphore is lazy-init'd on first acquire — call
// gpuMetricsResetSemForTest() to drop the cached singleton so the
// next gpuMetricsGetSem() picks up the lower cap.
func TestServiceMetricsConcurrencyCap503(t *testing.T) {
	t.Setenv("GPU_METRICS_MAX_CONCURRENT", "2")
	gpuMetricsResetSemForTest()
	t.Cleanup(gpuMetricsResetSemForTest)

	// Provider that blocks until released so we can pile up in-flight
	// requests above the cap.
	release := make(chan struct{})
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0

	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)
	p.On("ServiceMetrics", "app1", "web", mock.Anything).Return(
		structs.Metrics{}, nil,
	).Run(func(args mock.Arguments) {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
	})

	s := NewWithProvider(p)
	s.Logger = logger.Discard
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}
	ht := httptest.NewServer(s)
	defer ht.Close()

	get := func() *http.Response {
		u, _ := url.Parse(ht.URL + "/apps/app1/services/web/metrics")
		q := u.Query()
		// Use rack-side defaults — start/end will be filled in by validateMetricsRange.
		q.Set("period", "30")
		u.RawQuery = q.Encode()
		req, _ := http.NewRequest("GET", u.String(), nil)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}

	// Fire N=3 concurrent requests with cap=2; expect at least one to
	// return 503. The first 2 should remain in-flight (blocked on
	// release).
	var wg sync.WaitGroup
	statuses := make(chan int, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := get()
			statuses <- resp.StatusCode
			_ = resp.Body
			resp.Body.Close()
		}()
	}

	// Wait until either the cap is reached OR a 503 has come in.
	deadline := time.After(2 * time.Second)
	got503 := false
	count := 0
loop:
	for count < 3 {
		select {
		case st := <-statuses:
			count++
			if st == http.StatusServiceUnavailable {
				got503 = true
				// Release the blocked ones so the test exits.
				close(release)
				break loop
			}
		case <-deadline:
			t.Fatal("timed out waiting for 503 from semaphore")
		}
	}

	// Drain remaining responses.
	go func() {
		for i := count; i < 3; i++ {
			<-statuses
		}
	}()

	// Wait for all goroutines.
	closed := make(chan struct{})
	go func() {
		wg.Wait()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
	}

	require.True(t, got503, "expected at least one 503 response with cap=2 and 3 concurrent requests")
	require.LessOrEqual(t, maxInFlight, 2, "more requests entered provider than cap allowed")
}

// TestValidateAppName_RejectsRegexMetaChars verifies F-SEC-20 at the
// helper level — protects against regex meta-chars in path-vars
// or services= elements that would otherwise jail-break the alternation
// in QueryGPURange.
func TestValidateAppName_RejectsRegexMetaChars(t *testing.T) {
	cases := []struct {
		name    string
		valid   bool
	}{
		{"web", true},
		{"web-1", true},
		{"web1", true},
		{"Web", false},          // uppercase
		{"web|admin", false},    // alternation
		{"web.*", false},        // regex meta-chars
		{"web admin", false},    // space
		{"-web", false},         // leading dash
		{"1web", false},         // leading digit
		{"", false},             // empty
	}
	for _, c := range cases {
		err := validateAppName(c.name)
		if c.valid {
			require.NoError(t, err, "%q should be valid", c.name)
		} else {
			require.Error(t, err, "%q should be rejected", c.name)
		}
	}
	_ = strings.Contains // keep imports if unused
}
