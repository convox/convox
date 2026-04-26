package k8s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/convox/convox/pkg/common"
	cxhmac "github.com/convox/convox/pkg/hmac"
	"github.com/convox/convox/pkg/structs"
)

type event struct {
	Action    string            `json:"action"`
	Data      map[string]string `json:"data"`
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
}

// webhookClientTimeout is the total deadline for one webhook POST (DNS, dial,
// TLS, request, response). 30s is generous: typical ALB idle timeout is 60s
// and healthy receivers ack in <1s. Tests may override via
// SetWebhookClientTimeoutForTest in export_test.go.
var webhookClientTimeout = 30 * time.Second

// webhookClient is the package-scoped client used for all webhook POSTs. It
// is rebuilt by SetWebhookClientTimeoutForTest when tests need a shorter
// deadline. Production callers always observe the 30s default.
var webhookClient = &http.Client{Timeout: webhookClientTimeout}

// dispatchWebhookFn is the inner dispatcher invoked by dispatchWebhookSafely
// inside the recover() scope. Tests substitute a panicking stub via
// SetDispatchWebhookFnForTest in export_test.go to assert that recover()
// catches goroutine panics. Production code paths must NOT touch this var.
var dispatchWebhookFn = dispatchWebhook

// dispatchWebhookSignedFn is the per-dispatch hook that lets EventSend
// pass parsed signing keys down to dispatchWebhook without changing the
// (url, body) signature that older test stubs install. Tests may override
// to assert the keys-passed-through path. Production code MUST NOT touch.
var dispatchWebhookSignedFn = dispatchWebhookSigned

// dispatchHookOverridden is set by SetDispatchWebhookFnForTest so the
// safely-wrapper knows to route through the legacy (url, body) hook
// instead of the signed dispatcher. Production never touches this.
var dispatchHookOverridden = false

// isTestDispatchHookActive reports whether a test has installed a
// (url, body) dispatcher via SetDispatchWebhookFnForTest. Used by the
// safely-wrapper to preserve test-stub semantics for pre-D.2 callers.
func isTestDispatchHookActive() bool {
	return dispatchHookOverridden
}

func (p *Provider) EventSend(action string, opts structs.EventSendOptions) error {
	// Copy opts.Data into a local map so EventSend never mutates the caller's
	// map. Concurrent callers may share a Data instance (e.g. ranges over a
	// shared template); without this copy the rack/actor/message writes below
	// would race. The local map also lets central injection populate "actor"
	// from the request-scoped ctx without touching the caller's slice.
	local := make(map[string]string, len(opts.Data)+2)
	for k, v := range opts.Data {
		local[k] = v
	}
	// Central injection: derive the audit actor from the provider's
	// request-scoped ctx unless the caller pre-set it (per-call-site
	// override at "system"-emit sites: budget accumulator, release
	// advisories, service patch notes). ContextActor is panic-safe and
	// returns "unknown" when no actor is available.
	if _, ok := local["actor"]; !ok {
		local["actor"] = p.ContextActor()
	}

	e := event{
		Action:    action,
		Data:      local,
		Status:    common.DefaultString(opts.Status, "success"),
		Timestamp: time.Now().UTC(),
	}

	if e.Data["timestamp"] != "" {
		t, err := time.Parse(time.RFC3339, e.Data["timestamp"])
		if err == nil {
			e.Timestamp = t
		}
	}

	if opts.Error != nil {
		e.Status = "error"
		e.Data["message"] = *opts.Error
	}

	e.Data["rack"] = p.Name

	// Marshal the populated event (including the actor field). The HMAC
	// signature below covers these bytes verbatim, so receivers that
	// validate signatures see the actor field as part of the signed
	// payload.
	msg, err := json.Marshal(e)
	if err != nil {
		return err
	}

	// Parse signing keys ONCE per EventSend call, wrapped in a
	// synchronous defer-recover so a hmac panic does not crash the
	// caller (rack-param controller, budget-cap accumulator). On any
	// parse failure we degrade to unsigned dispatch — the api pod
	// continues operating; webhook delivery succeeds; receivers
	// configured to require signatures will reject (operator-facing
	// degrade is intentional and surfaced via the WARN log below).
	var signingKeys [][]byte
	if p.WebhookSigningKey != "" {
		var parseErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					parseErr = fmt.Errorf("hmac.ParseSigningKeys panic: %v", r)
				}
			}()
			signingKeys, parseErr = cxhmac.ParseSigningKeys(p.WebhookSigningKey)
		}()
		if parseErr != nil {
			fmt.Printf("ns=event_dispatch at=parse_keys_failed error=%q signing=disabled\n", parseErr)
			signingKeys = nil
		}
	}

	for _, wh := range p.webhooks {
		go dispatchWebhookSafely(wh, msg, signingKeys)
	}

	return nil
}

// dispatchWebhookSafely wraps dispatchWebhook in a panic-recovery scope so a
// hung receiver, a transport panic, or any other dispatch error is logged
// instead of crashing the api pod. Errors are emitted as structured stdout
// log lines (ns=event_dispatch) with host-only redaction so URL query
// strings never leak into logs. The signingKeys slice is nil when the rack
// param webhook_signing_key is unset; in that case the wire format is
// byte-identical to 3.24.5.
func dispatchWebhookSafely(url string, body []byte, signingKeys [][]byte) {
	defer func() {
		if r := recover(); r != nil {
			// F-23 fix: drop debug.Stack() from the panic recovery log.
			// The stack frames may surface internal arg values; the panic
			// value `r` is enough operational diagnostic (host + cause).
			// See pkg/hmac.SignedHeader for the inner panic-recovery scope
			// that captures details closer to the failing call site.
			fmt.Printf("ns=event_dispatch at=recover url_host=%s panic=%q\n", redactURLHost(url), r)
		}
	}()

	// Test stubs that pre-date D.2 install via SetDispatchWebhookFnForTest
	// and use the unsigned (url, body) signature. If a test stub has been
	// installed that replaces dispatchWebhookFn, route through it so the
	// stub's behavior (panic, error, count) is preserved. Production
	// always reaches the signed path because dispatchWebhookFn is the
	// production dispatcher and signingKeys is honored.
	if isTestDispatchHookActive() {
		if err := dispatchWebhookFn(url, body); err != nil {
			fmt.Printf("ns=event_dispatch at=error url_host=%s error=%q\n", redactURLHost(url), redactErrorURL(err, url))
		}
		return
	}

	if err := dispatchWebhookSignedFn(url, body, signingKeys); err != nil {
		fmt.Printf("ns=event_dispatch at=error url_host=%s error=%q\n", redactURLHost(url), redactErrorURL(err, url))
	}
}

// redactErrorURL strips the raw URL from net/http transport error messages
// before they reach log output. The Go stdlib wraps every transport error in
// *url.Error{Op, URL, Err} and Error() embeds the full URL — query strings
// included — into the formatted message. This bypasses redactURLHost, so
// logs would leak ?token=... despite the host-only convention. We unwrap to
// the inner error and reformat using only the redacted host.
func redactErrorURL(err error, raw string) string {
	if err == nil {
		return ""
	}
	if ue, ok := err.(*neturl.Error); ok {
		return fmt.Sprintf("%s %s: %s", ue.Op, redactURLHost(raw), ue.Err)
	}
	return err.Error()
}

// dispatchWebhook is the unsigned production dispatcher kept for tests
// that pre-date D.2 (they install via SetDispatchWebhookFnForTest using
// the (url, body) signature). It delegates to dispatchWebhookSigned with
// nil keys — so wire-format is byte-identical to 3.24.5.
func dispatchWebhook(url string, body []byte) error {
	return dispatchWebhookSigned(url, body, nil)
}

// dispatchWebhookSigned posts body to url and, when signingKeys is
// non-empty, sets the Convox-Signature header. B.1's defer-recover scope
// is owned by dispatchWebhookSafely above; HMAC sign runs here AFTER the
// recover engages and BEFORE client.Do, so a hmac panic is caught.
func dispatchWebhookSigned(url string, body []byte, signingKeys [][]byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if len(signingKeys) > 0 {
		sig := cxhmac.SignedHeader(time.Now().Unix(), body, signingKeys)
		if sig != "" {
			// Set (not Add): exactly one Convox-Signature header line on the
			// wire. Header collision verified by RoundTripper test in
			// event_test.go (TestDispatchWebhook_NoMiddlewareDoubleSet).
			req.Header.Set("Convox-Signature", sig)
		}
	}

	res, err := webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		fmt.Printf("ns=event_dispatch at=non2xx url_host=%s status=%d\n", redactURLHost(url), res.StatusCode)
		return fmt.Errorf("webhook %s returned %d", redactURLHost(url), res.StatusCode)
	}

	return nil
}

// redactURLHost returns the host portion of a webhook URL so log lines never
// include query-string secrets (e.g. ?token=...). Returns "<unparseable>" if
// the URL cannot be parsed; returns "<empty>" for blank input.
func redactURLHost(raw string) string {
	if raw == "" {
		return "<empty>"
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Host == "" {
		return "<unparseable>"
	}
	return u.Host
}

// redactedWebhookURL returns a payload-safe redacted webhook URL preserving
// scheme + host so the result is RFC 3986-valid. Distinct from redactURLHost
// (host-only, for log lines) — receivers parsing payload.webhook_url with
// new URL(...) need a scheme to avoid a parse error. Returns "<unparseable>"
// on parse failure or missing scheme/host; returns "<empty>" for blank input.
func redactedWebhookURL(raw string) string {
	if raw == "" {
		return "<empty>"
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return "<unparseable>"
	}
	return u.Scheme + "://" + u.Host
}
