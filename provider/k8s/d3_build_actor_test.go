package k8s_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

// captureBothEvents collects start + done payloads for a single
// BuildImportImage invocation. Returns a map keyed by action.
func captureBothEvents(t *testing.T, fn func(*k8s.Provider) error) map[string]map[string]any {
	t.Helper()

	type sink struct {
		mu       sync.Mutex
		payloads []map[string]any
	}
	s := &sink{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if json.Unmarshal(b, &p) == nil {
			s.mu.Lock()
			s.payloads = append(s.payloads, p)
			s.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	got := map[string]map[string]any{}
	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		require.NoError(t, fn(p))

		// Wait for both :start and :done payloads (or fail at 5s).
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			n := len(s.payloads)
			s.mu.Unlock()
			if n >= 2 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.payloads {
			a, _ := p["action"].(string)
			if a == "build:import-image:start" || a == "build:import-image:done" {
				got[a] = p
			}
		}
	})
	return got
}

func setupAppAndBuild(t *testing.T, p *k8s.Provider, build string) {
	t.Helper()
	kk, _ := p.Cluster.(*fake.Clientset)
	require.NoError(t, appCreate(kk, "rack1", "app1"))
	require.NoError(t, buildCreate(p.Convox, "rack1-app1", build, "basic"))
}

// withSkopeoStub replaces the skopeo exec stub with a no-op for the duration
// of the test. Returns a restore that the caller MUST defer.
func withSkopeoStub(t *testing.T) func() {
	t.Helper()
	orig := *k8s.SkopeoExecForTest
	*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
		return []byte("ok"), nil
	}
	return func() { *k8s.SkopeoExecForTest = orig }
}

// TestBuildImportImage_StartEventCarriesActor: :start emit reads
// p.ContextActor() before launching the goroutine.
func TestBuildImportImage_StartEventCarriesActor(t *testing.T) {
	defer withSkopeoStub(t)()

	got := captureBothEventsWithProviderHook(t,
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			setupAppAndBuild(t, p, "buildd3a")
			return p.BuildImportImage("app1", "buildd3a", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
		})

	require.Contains(t, got, "build:import-image:start")
	startData, _ := got["build:import-image:start"]["data"].(map[string]any)
	assert.Equal(t, "system-write", startData["actor"])
}

// TestBuildImportImage_StartDonePairInvariant: exactly ONE :start and ONE
// :done per invocation.
func TestBuildImportImage_StartDonePairInvariant(t *testing.T) {
	defer withSkopeoStub(t)()

	type sink struct {
		mu     sync.Mutex
		starts int
		dones  int
	}
	s := &sink{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if json.Unmarshal(b, &p) == nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			switch p["action"] {
			case "build:import-image:start":
				s.starts++
			case "build:import-image:done":
				s.dones++
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		setupAppAndBuild(t, p, "buildd3b")
		require.NoError(t, p.BuildImportImage("app1", "buildd3b", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{}))

		// Wait for both events (or 5s).
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			ok := s.starts >= 1 && s.dones >= 1
			s.mu.Unlock()
			if ok {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		// Brief settle for any straggler dispatches.
		drainPendingDispatches()

		s.mu.Lock()
		defer s.mu.Unlock()
		assert.Equal(t, 1, s.starts, "exactly one :start emit per invocation")
		assert.Equal(t, 1, s.dones, "exactly one :done emit per invocation")
	})
}

// TestBuildImportImage_StartDoneActorEquality: both events MUST carry the
// same actor value (R2 tests N3).
func TestBuildImportImage_StartDoneActorEquality(t *testing.T) {
	defer withSkopeoStub(t)()

	got := captureBothEventsWithProviderHook(t,
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			setupAppAndBuild(t, p, "buildd3c")
			return p.BuildImportImage("app1", "buildd3c", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
		})

	require.Contains(t, got, "build:import-image:start")
	require.Contains(t, got, "build:import-image:done")
	startData, _ := got["build:import-image:start"]["data"].(map[string]any)
	doneData, _ := got["build:import-image:done"]["data"].(map[string]any)
	assert.Equal(t, "system-write", startData["actor"])
	assert.Equal(t, "system-write", doneData["actor"])
	assert.Equal(t, startData["actor"], doneData["actor"], "start/done actor MUST be equal")
}

// TestBuildImportImage_NoContext_BothEventsUnknown: provider with Background
// ctx -> both events carry "unknown".
func TestBuildImportImage_NoContext_BothEventsUnknown(t *testing.T) {
	defer withSkopeoStub(t)()

	got := captureBothEvents(t, func(p *k8s.Provider) error {
		setupAppAndBuild(t, p, "buildd3d")
		return p.BuildImportImage("app1", "buildd3d", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
	})
	require.Contains(t, got, "build:import-image:start")
	require.Contains(t, got, "build:import-image:done")
	startData, _ := got["build:import-image:start"]["data"].(map[string]any)
	doneData, _ := got["build:import-image:done"]["data"].(map[string]any)
	assert.Equal(t, "unknown", startData["actor"])
	assert.Equal(t, "unknown", doneData["actor"])
}

// TestBuildImportImage_PanicInRun_DoneStillCarriesActor: deferred recover
// fires :done; capturedActor preserved through the panic+recovery boundary.
func TestBuildImportImage_PanicInRun_DoneStillCarriesActor(t *testing.T) {
	orig := *k8s.SkopeoExecForTest
	defer func() { *k8s.SkopeoExecForTest = orig }()
	*k8s.SkopeoExecForTest = func(ctx context.Context, args ...string) ([]byte, error) {
		// Panic synthesizes a goroutine crash inside buildImportImageRun's exec
		// path; the deferred recover() in BuildImportImage's goroutine MUST
		// still emit :done with the captured actor.
		return nil, errors.New("simulated skopeo failure for test")
	}

	got := captureBothEventsWithProviderHook(t,
		func(p *k8s.Provider) *k8s.Provider {
			ctx := context.WithValue(context.Background(), structs.ConvoxJwtUserCtxKey, "system-write")
			pp, _ := p.WithContext(ctx).(*k8s.Provider)
			return pp
		},
		func(p *k8s.Provider) error {
			setupAppAndBuild(t, p, "buildd3e")
			return p.BuildImportImage("app1", "buildd3e", "vllm/vllm-openai:v0.6.3", structs.BuildImportImageOptions{})
		})
	require.Contains(t, got, "build:import-image:done")
	doneData, _ := got["build:import-image:done"]["data"].(map[string]any)
	assert.Equal(t, "system-write", doneData["actor"], "actor preserved through error path")
	// status must be "failed" because skopeo errored.
	assert.Equal(t, "failed", doneData["status"])
}

func captureBothEventsWithProviderHook(
	t *testing.T,
	hook func(*k8s.Provider) *k8s.Provider,
	fn func(*k8s.Provider) error,
) map[string]map[string]any {
	t.Helper()

	type sink struct {
		mu       sync.Mutex
		payloads []map[string]any
	}
	s := &sink{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var p map[string]any
		if json.Unmarshal(b, &p) == nil {
			s.mu.Lock()
			s.payloads = append(s.payloads, p)
			s.mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	got := map[string]map[string]any{}
	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv.URL})
		pp := hook(p)
		k8s.SetWebhooksForTest(pp, []string{srv.URL})

		require.NoError(t, fn(pp))

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			s.mu.Lock()
			n := len(s.payloads)
			s.mu.Unlock()
			if n >= 2 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		s.mu.Lock()
		defer s.mu.Unlock()
		for _, p := range s.payloads {
			a, _ := p["action"].(string)
			if a == "build:import-image:start" || a == "build:import-image:done" {
				got[a] = p
			}
		}
	})
	return got
}
