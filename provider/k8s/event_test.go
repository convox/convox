package k8s_test

import (
	"context"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	cxhmac "github.com/convox/convox/pkg/hmac"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// captureStdout redirects os.Stdout into a pipe and returns a function that
// restores stdout and yields whatever was written. Used to assert structured
// log lines emitted by dispatchWebhookSafely without depending on a logger
// abstraction (the production code uses fmt.Printf to match
// budget_accumulator's existing ns=... convention).
func captureStdout(t *testing.T) func() string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	return func() string {
		_ = w.Close()
		os.Stdout = orig
		<-done
		return buf.String()
	}
}

// drainPendingDispatches gives goroutines launched by EventSend a brief
// window to land before assertions run. 200ms is generous: in-process
// httptest round-trips complete in <5ms; refused connections fail in <50ms.
func drainPendingDispatches() {
	time.Sleep(200 * time.Millisecond)
}

func TestDispatchWebhook_2xxResponse_ReturnsNil(t *testing.T) {
	var (
		mu        sync.Mutex
		gotMethod string
		gotCT     string
		gotBody   []byte
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotMethod = r.Method
		gotCT = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	body := []byte(`{"action":"app:create","data":{"rack":"rack1"},"status":"success","timestamp":"2026-04-25T12:00:00Z"}`)

	err := k8s.DispatchWebhookForTest(srv.URL, body)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "POST", gotMethod)
	assert.Equal(t, "application/json", gotCT)
	assert.JSONEq(t, string(body), string(gotBody))
}

// TestEventSend_FallsBackToConfigMap_WhenCacheEmpty pins the non-leader
// pod fallback. Pre-fix, EventSend iterated p.webhooks (empty on
// non-leader) and silently dropped every event. Post-fix, when
// p.webhooks is empty, EventSend reads the configmap synchronously
// and dispatches to the URLs stored there. This test seeds the
// configmap directly without touching p.webhooks.
func TestEventSend_FallsBackToConfigMap_WhenCacheEmpty(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		// Seed the webhooks configmap directly (mirrors what
		// webhookCreate does). Do NOT call SetWebhooksForTest so
		// EventSend exercises the configmap-fallback path.
		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{Name: "webhooks", Namespace: p.Namespace},
			Data:       map[string]string{"hook1": srv.URL},
		}
		_, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		err = p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		drainPendingDispatches()

		assert.Equal(t, int32(1), atomic.LoadInt32(&hits),
			"non-leader fallback must read configmap synchronously and dispatch")
	})
}

// TestEventSend_WebhookListError_FailsOpen pins the silent-degrade
// contract. EventSend has always been best-effort fan-out; the new
// configmap-fallback path must NOT propagate transient kube-apiserver
// errors into the caller (release:promote, app:create, scale-override
// have nothing to do with webhook delivery). When webhookList fails,
// EventSend logs and returns nil.
//
// Also asserts the operator-grep log line is emitted with the
// canonical `ns=event_dispatch at=webhook_list_failed` shape — without
// that, a regression that swaps the log statement for a panic or a
// silent suppression would still pass the no-error assertion.
func TestEventSend_WebhookListError_FailsOpen(t *testing.T) {
	testProviderManual(t, func(p *k8s.Provider, c *fake.Clientset) {
		// Make every configmap GET return an error.
		c.PrependReactor("get", "configmaps", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("kube-apiserver unavailable")
		})

		readStdout := captureStdout(t)

		// Do NOT seed p.webhooks; force fallback path.
		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		captured := readStdout()

		assert.NoError(t, err, "EventSend must fail open when webhookList errors — webhook fan-out is best-effort")
		assert.Contains(t, captured, "ns=event_dispatch at=webhook_list_failed",
			"failure path must emit the canonical operator-grep log line")
		assert.Contains(t, captured, "dispatch=skipped",
			"log line must include dispatch=skipped so operators can correlate to dropped event")
	})
}

func TestEventSend_FiresAllConfiguredWebhooks(t *testing.T) {
	var hits1, hits2 int32

	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits1, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits2, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv2.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhooksForTest(p, []string{srv1.URL, srv2.URL})

		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		drainPendingDispatches()

		assert.Equal(t, int32(1), atomic.LoadInt32(&hits1), "srv1 should receive exactly one POST")
		assert.Equal(t, int32(1), atomic.LoadInt32(&hits2), "srv2 should receive exactly one POST")
	})
}

func TestDispatchWebhook_SlowReceiver_TimesOutAt30s(t *testing.T) {
	// Use a context-aware handler that blocks until the test releases or
	// the client gives up so we never actually wait 30s real time. The
	// package-scoped client timeout is shortened to 100ms via the test
	// hook; the production 30s default is restored on defer.
	released := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-released:
		case <-r.Context().Done():
		}
	}))
	// Order matters: close the released channel BEFORE srv.Close, so any
	// in-flight handler exits its select before httptest waits for it.
	defer srv.Close()
	defer close(released)

	restore := k8s.SetWebhookClientTimeoutForTest(100 * time.Millisecond)
	defer restore()

	start := time.Now()
	err := k8s.DispatchWebhookForTest(srv.URL, []byte(`{}`))
	elapsed := time.Since(start)

	require.Error(t, err)
	msg := err.Error()
	assert.Truef(t,
		strings.Contains(msg, "deadline exceeded") ||
			strings.Contains(msg, "Timeout exceeded") ||
			strings.Contains(msg, "context canceled"),
		"expected timeout-shaped error, got: %q", msg)
	assert.Less(t, elapsed, 5*time.Second, "should not block past client timeout")
}

func TestDispatchWebhook_ConnectionRefused_LogsAndReturnsError(t *testing.T) {
	// Port 1 is reliably refused on Linux; if a future kernel ever binds
	// it, switch to a listener that closes immediately.
	getOutput := captureStdout(t)
	k8s.DispatchWebhookSafelyForTest("http://127.0.0.1:1/hook", []byte(`{}`))
	out := getOutput()

	assert.Contains(t, out, "ns=event_dispatch at=error")
	assert.Contains(t, out, "url_host=127.0.0.1:1")
}

func TestDispatchWebhook_Non2xxStatus_LogsWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	getOutput := captureStdout(t)
	err := k8s.DispatchWebhookForTest(srv.URL, []byte(`{}`))
	out := getOutput()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned 500")
	assert.Contains(t, out, "ns=event_dispatch at=non2xx")
	assert.Contains(t, out, "status=500")
}

func TestEventSendGoroutine_PanicInDispatch_RecoveredAndLogged(t *testing.T) {
	// Inject a dispatcher that panics. The recover() scope inside
	// dispatchWebhookSafely must catch the panic and emit
	// `ns=event_dispatch at=recover ...` instead of crashing.
	restore := k8s.SetDispatchWebhookFnForTest(func(url string, body []byte) error {
		panic("boom: simulated transport panic")
	})
	defer restore()

	getOutput := captureStdout(t)
	require.NotPanics(t, func() {
		k8s.DispatchWebhookSafelyForTest("https://hooks.example.com/path", []byte(`{}`))
	})
	out := getOutput()

	assert.Contains(t, out, "ns=event_dispatch at=recover")
	assert.Contains(t, out, "url_host=hooks.example.com")
	assert.Contains(t, out, "boom: simulated transport panic")
	// F-23 fix (catalog F-23): debug.Stack() was dropped from the
	// panic-recovery log line because stack frames may surface internal
	// arg values. The panic value alone is enough operational diagnostic;
	// inner code paths (cxhmac.SignedHeader) maintain their own
	// recover-and-log discipline closer to the failing call site.
	assert.NotContains(t, out, "stack=", "stack trace must be dropped from recover log per F-23 redaction policy")
}

func TestDispatchWebhook_URLWithSecretsInQuery_LogsHostOnly(t *testing.T) {
	const secret = "SECRET_TOKEN_12345"
	url := "http://127.0.0.1:1/path?token=" + secret + "&other=value"

	getOutput := captureStdout(t)
	k8s.DispatchWebhookSafelyForTest(url, []byte(`{}`))
	out := getOutput()

	assert.Contains(t, out, "url_host=127.0.0.1:1")
	assert.NotContains(t, out, secret, "log output must not contain query-string secrets")
	assert.NotContains(t, out, "token=", "log output must not contain raw query-string keys")
}

func TestEventSend_PreservesPayloadJSONShape(t *testing.T) {
	// Capture the body the goroutine POSTs and assert its JSON shape
	// matches the canonical 4-field event contract: action, data, status,
	// timestamp. Field renames or removals would break webhook receivers.
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

		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		select {
		case body := <-bodyCh:
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			canonical := []string{"action", "data", "status", "timestamp"}
			for _, key := range canonical {
				_, ok := payload[key]
				assert.Truef(t, ok, "payload missing required field %q; full payload: %s", key, string(body))
			}
			for key := range payload {
				assert.Containsf(t, canonical, key,
					"unexpected field %q in payload; webhook receivers may not handle it", key)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete within 2s")
		}
	})
}

func TestRedactURLHost_StripsSecrets(t *testing.T) {
	cases := map[string]string{
		"https://hooks.example.com/path?token=abc": "hooks.example.com",
		"http://10.0.0.1:8080/?key=secret":         "10.0.0.1:8080",
		"":                                         "<empty>",
		"://broken":                                "<unparseable>",
	}
	for in, want := range cases {
		got := k8s.RedactURLHostForTest(in)
		assert.Equalf(t, want, got, "redactURLHost(%q)", in)
	}
}

// TestRedactedWebhookURL_PreservesSchemeAndHost — MF-4 fix
// (R4 γ-10 ADV-K8S-12). The payload-tier helper must return scheme+host
// so receivers parsing payload.webhook_url with `new URL(...)` get an
// RFC 3986-valid URL. Distinct from redactURLHost (host-only, log-only).
func TestRedactedWebhookURL_PreservesSchemeAndHost(t *testing.T) {
	cases := map[string]string{
		"https://hooks.slack.com/services/T0/B0/SECRET": "https://hooks.slack.com",
		"https://hooks.example.com/path?token=abc":      "https://hooks.example.com",
		"http://10.0.0.1:8080/?key=secret":              "http://10.0.0.1:8080",
		"https://discord.com/api/webhooks/12345/TOKEN":  "https://discord.com",
		"":          "<empty>",
		"://broken": "<unparseable>",
		"http://":   "<unparseable>", // missing host
		"//host":    "<unparseable>", // missing scheme
	}
	for in, want := range cases {
		got := k8s.RedactedWebhookURLForTest(in)
		assert.Equalf(t, want, got, "redactedWebhookURL(%q)", in)
	}
}

// ---------------------------------------------------------------------------
// D.2: HMAC webhook signing
// ---------------------------------------------------------------------------

const d2FixedKeyHex = "5257a869e7ecebeda32affa62cdca3fa37e8c0a98c3f2db5a8f5da3b2a3e9c4e"
const d2SecondKeyHex = "8c1f3e0b9d4a7f2e6c5b8a3d4f9e2c1a7b6d5f4e3c2b1a9d8f7e6c5b4a3d2e10"

// TestEventSend_HMAC_KeyMissing_NoHeader (R3 mandatory: error)
func TestEventSend_HMAC_KeyMissing_NoHeader(t *testing.T) {
	headerCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case headerCh <- r.Header.Get("Convox-Signature"):
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		// p.WebhookSigningKey defaults to "" — strict 3.24.5 behavior
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		select {
		case got := <-headerCh:
			assert.Equal(t, "", got, "no key configured -> no Convox-Signature header")
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete")
		}
	})
}

// TestEventSend_HMAC_SigninedHeaderPresent (R3 mandatory: happy path)
func TestEventSend_HMAC_SigninedHeaderPresent(t *testing.T) {
	headerCh := make(chan string, 1)
	bodyCh := make(chan []byte, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case headerCh <- r.Header.Get("Convox-Signature"):
		default:
		}
		select {
		case bodyCh <- body:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhookSigningKeyForTest(p, d2FixedKeyHex)
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		before := time.Now().Unix()
		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		select {
		case header := <-headerCh:
			require.NotEmpty(t, header, "Convox-Signature must be set when key is configured")
			assert.Contains(t, header, "t=")
			assert.Contains(t, header, "v1=")

			// Verify with the package — this also confirms HMAC matches body
			body := <-bodyCh
			key, _ := hex.DecodeString(d2FixedKeyHex)
			err := cxhmac.Verify(body, header, [][]byte{key}, 5*time.Minute)
			assert.NoError(t, err, "signature must verify against received body")

			// Timestamp must be near-now
			parts := strings.SplitN(header, ",", 2)
			require.Len(t, parts, 2)
			tField := strings.TrimPrefix(parts[0], "t=")
			tVal, perr := strconv.ParseInt(tField, 10, 64)
			require.NoError(t, perr)
			after := time.Now().Unix()
			assert.GreaterOrEqual(t, tVal, before)
			assert.LessOrEqual(t, tVal, after)
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete")
		}
	})
}

// TestEventSend_HMAC_TamperedPayload_SigDiffers (R3 mandatory: negative)
func TestEventSend_HMAC_TamperedPayload_SigDiffers(t *testing.T) {
	body := []byte(`{"action":"app:create","data":{"app":"demo"},"status":"success","timestamp":"2026-04-25T12:00:00Z"}`)
	tampered := []byte(`{"action":"app:create","data":{"app":"DIFFERENT"},"status":"success","timestamp":"2026-04-25T12:00:00Z"}`)
	require.NotEqual(t, body, tampered)

	key, _ := hex.DecodeString(d2FixedKeyHex)
	now := time.Now().Unix()

	origHeader := cxhmac.SignedHeader(now, body, [][]byte{key})
	tamperedHeader := cxhmac.SignedHeader(now, tampered, [][]byte{key})

	// Same key, same timestamp, different body -> different signature
	require.NotEqual(t, origHeader, tamperedHeader,
		"different bodies must produce different signatures")

	// Cross-verification: orig sig must NOT verify the tampered body.
	require.Error(t, cxhmac.Verify(tampered, origHeader, [][]byte{key}, 5*time.Minute))
	require.Error(t, cxhmac.Verify(body, tamperedHeader, [][]byte{key}, 5*time.Minute))
}

func TestDispatchWebhook_TwoKeys_BothSignaturesPresent(t *testing.T) {
	headerCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case headerCh <- r.Header.Get("Convox-Signature"):
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhookSigningKeyForTest(p, d2FixedKeyHex+","+d2SecondKeyHex)
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		select {
		case header := <-headerCh:
			require.NotEmpty(t, header)
			parts := strings.Split(header, ",")
			var v1count int
			for _, p := range parts {
				if strings.HasPrefix(strings.TrimSpace(p), "v1=") {
					v1count++
				}
			}
			assert.Equal(t, 2, v1count, "two keys -> two v1= segments")
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete")
		}
	})
}

// TestDispatchWebhook_OnlyOneSignatureHeader — assert exactly one
// Convox-Signature header line set. Sentinel test against double-Set.
func TestDispatchWebhook_OnlyOneSignatureHeader(t *testing.T) {
	var (
		mu        sync.Mutex
		gotValues []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		// http.Header.Values returns all values; if header was Set
		// twice or merged from multiple sources, len(values) > 1.
		gotValues = append(gotValues, r.Header.Values("Convox-Signature")...)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	keys := [][]byte{}
	k, _ := hex.DecodeString(d2FixedKeyHex)
	keys = append(keys, k)

	err := k8s.DispatchWebhookSignedForTest(srv.URL, []byte(`{}`), keys)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, gotValues, 1, "exactly one Convox-Signature header expected")
}

// TestDispatchWebhook_NoMiddlewareDoubleSet — RoundTripper assertion that
// no middleware between dispatchWebhookSigned and the wire double-Sets the
// Convox-Signature header. Per spec §8.1 R2 F-T-NEW-1 BLOCK.
func TestDispatchWebhook_NoMiddlewareDoubleSet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rec := &recordingRoundTripper{inner: http.DefaultTransport}

	restore := k8s.SetWebhookClientTransportForTest(rec)
	defer restore()

	keys := [][]byte{}
	k, _ := hex.DecodeString(d2FixedKeyHex)
	keys = append(keys, k)

	err := k8s.DispatchWebhookSignedForTest(srv.URL, []byte(`{}`), keys)
	require.NoError(t, err)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.Len(t, rec.requests, 1)
	got := rec.requests[0].Header.Values("Convox-Signature")
	assert.Len(t, got, 1, "RoundTripper must observe exactly one Convox-Signature header")
}

type recordingRoundTripper struct {
	inner    http.RoundTripper
	mu       sync.Mutex
	requests []*http.Request
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req.Clone(req.Context()))
	r.mu.Unlock()
	return r.inner.RoundTrip(req)
}

// TestEventSend_HmacPackagePanic_DegradesGracefully — sentinel for the
// synchronous defer-recover at EventSend's parse-keys call. A panic in
// the hmac parse path must not crash the caller; webhooks dispatch
// unsigned and a structured WARN is logged. (Spec §8.7.)
func TestEventSend_HmacPackagePanic_DegradesGracefully(t *testing.T) {
	// We can't easily force a panic out of cxhmac.ParseSigningKeys with
	// a valid key — so we use a deliberately-malformed key that hits the
	// degrade-on-error path AND assert no panic propagates. The
	// structured WARN's correctness is asserted via captured stdout.
	headerCh := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case headerCh <- r.Header.Get("Convox-Signature"):
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		// Invalid key (mixed case) -> ParseSigningKeys returns an error;
		// the EventSend wrapper logs WARN and falls through to unsigned.
		k8s.SetWebhookSigningKeyForTest(p, "ABCD"+strings.Repeat("a", 60))
		k8s.SetWebhooksForTest(p, []string{srv.URL})

		require.NotPanics(t, func() {
			err := p.EventSend("app:create", structs.EventSendOptions{
				Data: map[string]string{"app": "demo"},
			})
			require.NoError(t, err)
		})

		select {
		case got := <-headerCh:
			assert.Equal(t, "", got, "invalid key -> degrade to unsigned")
		case <-time.After(2 * time.Second):
			t.Fatal("webhook dispatch did not complete")
		}
	})
}

// Verifies the order-of-test-precedence: a pre-D.2 dispatcher hook
// (installed via SetDispatchWebhookFnForTest) takes priority over the
// signed dispatcher for the duration of the test. This protects existing
// tests that don't know about signingKeys.
func TestDispatchWebhook_LegacyHookOverridesSigned(t *testing.T) {
	var hits int32
	restore := k8s.SetDispatchWebhookFnForTest(func(url string, body []byte) error {
		atomic.AddInt32(&hits, 1)
		return nil
	})
	defer restore()

	k8s.DispatchWebhookSafelyForTest("https://example.invalid/x", []byte(`{}`))
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
}

// ---------------------------------------------------------------------------
// Webhook signing key eviction audit (no key material)
// ---------------------------------------------------------------------------

// TestWebhookSigningKeyEvictionAuditNoMaterial — when EventSend's parse
// returns an "at most" rejection because the configured webhook_signing_key
// has more keys than cxhmac.MaxSigningKeys (4 in 3.24.6), an audit row is
// emitted to stdout. The row's wire format is fixed
// (`audit_type=webhook_signing_key:eviction count=1 reason=key_count_exceeded
// max=N evicted_count=M`) and MUST contain NO key bytes, key hashes, or
// hex/base64/base64url encodings of the key material.
func TestWebhookSigningKeyEvictionAuditNoMaterial(t *testing.T) {
	// Five distinct 64-char high-entropy hex keys: max=4 → evicted_count=1.
	candidates := []string{
		"5257a869e7ecebeda32affa62cdca3fa37e8c0a98c3f2db5a8f5da3b2a3e9c4e",
		"8c1f3e0b9d4a7f2e6c5b8a3d4f9e2c1a7b6d5f4e3c2b1a9d8f7e6c5b4a3d2e10",
		"1a2b3c4d5e6f70819293a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5",
		"9f8e7d6c5b4a39281f0e1d2c3b4a5968778695a4b3c2d1e0f9a8b7c6d5e4f3c2",
		"2c4e6f81a3b5d7e9b1c2d3e4f50617283940a1b2c3d4e5f60718293a4b5c6d7e",
	}

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhookSigningKeyForTest(p, strings.Join(candidates, ","))
		// No webhooks configured — no dispatch goroutines to drain.
		k8s.SetWebhooksForTest(p, []string{})

		getOutput := captureStdout(t)
		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)
		captured := getOutput()

		// Audit line shape (no key material substring permitted).
		assert.Contains(t, captured, "audit_type=webhook_signing_key:eviction",
			"audit row must use the canonical type prefix")
		assert.Contains(t, captured, "reason=key_count_exceeded",
			"reason must be the canonical key_count_exceeded")
		assert.Contains(t, captured, "max=4",
			"max must reflect cxhmac.MaxSigningKeys (4) in 3.24.6")
		assert.Contains(t, captured, "evicted_count=1",
			"5 keys minus max=4 -> evicted_count=1")
		assert.Contains(t, captured, "count=1",
			"count is per-emit (one audit row per EventSend)")

		// No key bytes / no hashes (hex / base64 / base64url) anywhere in stdout.
		for i, c := range candidates {
			assert.NotContainsf(t, captured, c,
				"raw hex key #%d must not appear in audit log", i+1)

			rawBytes, derr := hex.DecodeString(c)
			require.NoError(t, derr)
			assert.NotContainsf(t, captured, string(rawBytes),
				"decoded key bytes #%d must not appear", i+1)

			hash := sha256.Sum256(rawBytes)
			assert.NotContainsf(t, captured, hex.EncodeToString(hash[:]),
				"sha256 hex of key #%d must not appear", i+1)
			assert.NotContainsf(t, captured, b64.StdEncoding.EncodeToString(hash[:]),
				"sha256 base64 of key #%d must not appear", i+1)
			assert.NotContainsf(t, captured, b64.URLEncoding.EncodeToString(hash[:]),
				"sha256 base64url of key #%d must not appear", i+1)
		}
	})
}

// ---------------------------------------------------------------------------
// Per-URL webhook timeout via JSON receiver config
// ---------------------------------------------------------------------------

// TestWebhookConfigmapJsonParseCache — receivers cache populated via
// SetWebhookReceiversForTest is consumed in preference to the urls slice;
// EventSend dispatches to each entry's URL with the entry's per-URL
// timeout.
func TestWebhookConfigmapJsonParseCache(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	testProvider(t, func(p *k8s.Provider) {
		k8s.SetWebhookReceiversForTest(p, []k8s.WebhookEntryForTest{
			{Name: "json_form", URL: srv.URL, Timeout: 5 * time.Second},
		})

		err := p.EventSend("app:create", structs.EventSendOptions{
			Data: map[string]string{"app": "demo"},
		})
		require.NoError(t, err)

		drainPendingDispatches()
		assert.Equal(t, int32(1), atomic.LoadInt32(&hits),
			"receivers-cache entry must be dispatched")
	})
}

// TestWebhookPerUrlTimeout — a 5s per-URL timeout fires before the
// 30s package default would; a 30s entry honors the package default.
// Asserts the dispatch chain threads the per-URL timeout to the
// transient http.Client built inside dispatchWebhookSigned.
func TestWebhookPerUrlTimeout(t *testing.T) {
	// Server blocks until the test releases or the client gives up.
	released := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-released:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(released)

	t.Run("5s per-URL timeout fires before 30s package default", func(t *testing.T) {
		start := time.Now()
		keys := [][]byte{}
		err := k8s.DispatchWebhookSignedWithTimeoutForTest(srv.URL, []byte(`{}`), keys, 100*time.Millisecond)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Less(t, elapsed, 5*time.Second,
			"per-URL 100ms must fire long before the package 30s default")
	})

	t.Run("default timeout uses package webhookClientTimeout", func(t *testing.T) {
		restore := k8s.SetWebhookClientTimeoutForTest(100 * time.Millisecond)
		defer restore()

		start := time.Now()
		keys := [][]byte{}
		err := k8s.DispatchWebhookSignedForTest(srv.URL, []byte(`{}`), keys)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.Less(t, elapsed, 5*time.Second,
			"package default (100ms) must apply when no per-URL timeout supplied")
	})
}

// TestParseFallsThroughOnInvalidJSON — a configmap value that does NOT
// begin with `{` is treated as a plain URL (3.24.5-compatible). A value
// that DOES begin with `{` but is malformed JSON is SKIPPED (not dispatched).
func TestParseFallsThroughOnInvalidJSON(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantSkip    bool
		wantURL     string
		wantTimeout time.Duration
	}{
		{
			name:        "plain url uses default timeout",
			raw:         "https://hooks.example.com/path",
			wantSkip:    false,
			wantURL:     "https://hooks.example.com/path",
			wantTimeout: k8s.DefaultWebhookTimeoutForTest(),
		},
		{
			name:     "malformed json beginning with brace is skipped",
			raw:      `{this is not json at all`,
			wantSkip: true,
		},
		{
			name:        "valid json with url + timeout uses parsed timeout",
			raw:         `{"url":"https://hooks.example.com/path","timeout":"5s"}`,
			wantSkip:    false,
			wantURL:     "https://hooks.example.com/path",
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "valid json with url, no timeout -> default",
			raw:         `{"url":"https://hooks.example.com/path"}`,
			wantSkip:    false,
			wantURL:     "https://hooks.example.com/path",
			wantTimeout: k8s.DefaultWebhookTimeoutForTest(),
		},
		{
			name:        "valid json with url + invalid timeout -> default",
			raw:         `{"url":"https://hooks.example.com/path","timeout":"5xyz"}`,
			wantSkip:    false,
			wantURL:     "https://hooks.example.com/path",
			wantTimeout: k8s.DefaultWebhookTimeoutForTest(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, skip := k8s.ParseWebhookEntryForTest("test-name", tc.raw)
			if tc.wantSkip {
				assert.True(t, skip, "must skip malformed entry")
				return
			}
			require.False(t, skip, "valid entry must not be skipped")
			assert.Equal(t, tc.wantURL, entry.URL, "URL must round-trip")
			assert.Equal(t, tc.wantTimeout, entry.Timeout, "Timeout must match expected")
		})
	}
}

// TestSkipsEntryWithEmptyURL — anti-trap: when a JSON entry parses
// successfully but the url field is empty/whitespace, the entry is SKIPPED
// (a structured WARN is emitted). Critical: the parser must NOT fall
// through to the plain-URL branch — the raw value is a JSON object string,
// not a URL, so dispatch would corrupt the URL field.
func TestSkipsEntryWithEmptyURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty url field", raw: `{"url":"","timeout":"5s"}`},
		{name: "whitespace-only url field", raw: `{"url":"   ","timeout":"5s"}`},
		{name: "missing url field", raw: `{"timeout":"5s"}`},
		{name: "url field with tab and newline", raw: `{"url":"\t\n","timeout":"5s"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry, skip := k8s.ParseWebhookEntryForTest("test-name", tc.raw)
			assert.True(t, skip, "empty/whitespace url in JSON must trigger skip")
			// Skipped entries return zero — the parser MUST NOT have fallen
			// through to plain-URL with the raw JSON string as a URL.
			assert.Empty(t, entry.URL,
				"skipped entry must NOT have a populated URL — falling through to plain-URL "+
					"semantics would dispatch the raw JSON object as a URL")
		})
	}
}

// TestEmptyURLEntrySkipsLogsWarning — assert the structured WARN is emitted
// to stdout so operators can grep for misconfigured webhooks.
func TestEmptyURLEntrySkipsLogsWarning(t *testing.T) {
	getOutput := captureStdout(t)
	_, skip := k8s.ParseWebhookEntryForTest("audit_internal", `{"url":"","timeout":"5s"}`)
	captured := getOutput()

	assert.True(t, skip)
	assert.Contains(t, captured, "ns=webhook_parse",
		"skip path must emit ns=webhook_parse log line")
	assert.Contains(t, captured, "reason=empty_url_in_json",
		"skip reason must surface for operator grep")
	assert.Contains(t, captured, "name=audit_internal",
		"skip log must include the entry name for operator visibility")
}
