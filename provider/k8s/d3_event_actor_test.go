package k8s_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: extract a single posted event payload.
func captureOnePayload(t *testing.T, fn func(*k8s.Provider) error) map[string]any {
	t.Helper()

	bodyCh := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		select {
		case bodyCh <- b:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var captured map[string]any
	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		require.NoError(t, fn(p))

		select {
		case body := <-bodyCh:
			require.NoError(t, json.Unmarshal(body, &captured))
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete within 2s")
		}
	})
	return captured
}

// TestEventSend_PopulatesActorFromContext verifies central injection: a
// Provider with WithContext(ctx-with-system-write) populates "actor"
// automatically.
func TestEventSend_PopulatesActorFromContext(t *testing.T) {
	payload := captureOnePayload(t, func(p *k8s.Provider) error {
		ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
		pp := p.WithContext(ctx)
		return pp.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
	})

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "system-write", data["actor"])
}

// TestEventSend_RespectsExplicitActor: caller pre-sets "actor" so EventSend
// MUST NOT overwrite (per-call-site override path; "system" emit sites).
func TestEventSend_RespectsExplicitActor(t *testing.T) {
	payload := captureOnePayload(t, func(p *k8s.Provider) error {
		ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
		pp := p.WithContext(ctx)
		return pp.EventSend("app:budget:cap", structs.EventSendOptions{
			Data: map[string]string{"actor": "system", "app": "demo"},
		})
	})

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "system", data["actor"], "explicit actor must survive central injection")
}

// TestEventSend_NoContext_PopulatesUnknown: Provider with Background ctx -> "unknown".
func TestEventSend_NoContext_PopulatesUnknown(t *testing.T) {
	payload := captureOnePayload(t, func(p *k8s.Provider) error {
		return p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
	})

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "unknown", data["actor"])
}

// TestEventSend_OptsDataMapCopy_NoMutation: the caller's map must remain
// unmodified (no rack/actor/timestamp written to the input). Run under -race
// to catch any partial conversion that left an e.Data write targeting the
// original opts.Data slice.
func TestEventSend_OptsDataMapCopy_NoMutation(t *testing.T) {
	bodyCh := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		select {
		case bodyCh <- b:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		caller := map[string]string{"app": "demo"}

		// Snapshot the caller map keys before EventSend.
		preKeys := make(map[string]string, len(caller))
		for k, v := range caller {
			preKeys[k] = v
		}

		errMsg := "boom"
		err := p.EventSend("app:create", structs.EventSendOptions{
			Data:  caller,
			Error: &errMsg,
		})
		require.NoError(t, err)

		// Wait for dispatch to finalize.
		select {
		case <-bodyCh:
		case <-time.After(2 * time.Second):
			t.Fatal("dispatch did not complete")
		}

		assert.Equal(t, preKeys, caller, "caller's Data map must be unmodified")
		_, hasActor := caller["actor"]
		assert.False(t, hasActor, "actor key must not appear in caller's map")
		_, hasRack := caller["rack"]
		assert.False(t, hasRack, "rack key must not appear in caller's map")
		_, hasMessage := caller["message"]
		assert.False(t, hasMessage, "error message must not appear in caller's map")
	})
}

// TestEventSend_ConcurrentCallers_NoRace: 50 goroutines each call EventSend
// with a SHARED Data map; assert no data race under go test -race and that
// the shared map is never mutated.
func TestEventSend_ConcurrentCallers_NoRace(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		const goroutines = 50
		shared := map[string]string{"app": "demo"}

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				_ = p.EventSend("app:create", structs.EventSendOptions{
					Data: shared,
				})
			}()
		}
		wg.Wait()

		// Wait briefly for fan-out to settle.
		drainPendingDispatches()

		assert.NotContains(t, shared, "actor", "shared caller map must not be mutated")
		assert.NotContains(t, shared, "rack", "shared caller map must not be mutated")
	})
}

// TestEventSend_LegacyPayloadParses asserts that an event payload missing
// "actor" still decodes cleanly into the canonical 4-field struct (3.24.5
// receivers must process 3.24.6 events; 3.24.6 receivers must process 3.24.5
// events under Go zero-value semantics).
func TestEventSend_LegacyPayloadParses(t *testing.T) {
	legacy := []byte(`{"action":"app:create","data":{"app":"demo","rack":"rack1"},"status":"success","timestamp":"2026-04-25T12:00:00Z"}`)

	type receiverEvent struct {
		Action    string            `json:"action"`
		Data      map[string]string `json:"data"`
		Status    string            `json:"status"`
		Timestamp time.Time         `json:"timestamp"`
	}
	var got receiverEvent
	require.NoError(t, json.Unmarshal(legacy, &got))
	assert.Equal(t, "app:create", got.Action)
	assert.Equal(t, "demo", got.Data["app"])
	// Absent "actor" reads as empty string (Go zero-value semantics).
	assert.Equal(t, "", got.Data["actor"], "absent actor key reads as empty string")
}
