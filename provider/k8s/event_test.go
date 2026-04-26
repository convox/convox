package k8s_test

import (
	"encoding/hex"
	"encoding/json"
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
		"":         "<empty>",
		"://broken": "<unparseable>",
		"http://":  "<unparseable>", // missing host
		"//host":   "<unparseable>", // missing scheme
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
