package k8s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	neturl "net/url"
	"strings"
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

var webhookClientTimeout = 30 * time.Second
var webhookClient = &http.Client{Timeout: webhookClientTimeout}

// Test seams — tests override via Set*ForTest in export_test.go.
var dispatchWebhookFn = dispatchWebhook
var dispatchWebhookSignedFn = dispatchWebhookSigned
var dispatchHookOverridden = false

func isTestDispatchHookActive() bool {
	return dispatchHookOverridden
}

func (p *Provider) EventSend(action string, opts structs.EventSendOptions) error {
	// Defensive copy — concurrent callers may share opts.Data.
	local := make(map[string]string, len(opts.Data)+2)
	for k, v := range opts.Data {
		local[k] = v
	}
	// Prefer explicit actor > ack_by > ContextActor (request-scoped fallback).
	if _, ok := local["actor"]; !ok {
		if ackBy, ok := local["ack_by"]; ok && ackBy != "" {
			local["actor"] = ackBy
		} else {
			local["actor"] = p.ContextActor()
		}
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

	msg, err := json.Marshal(e)
	if err != nil {
		return err
	}

	// Parse signing keys with panic recovery — degrade to unsigned on failure.
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
			if strings.Contains(parseErr.Error(), "at most") {
				count := 0
				trimmed := strings.TrimSpace(p.WebhookSigningKey)
				if trimmed != "" {
					count = strings.Count(trimmed, ",") + 1
				}
				if count > cxhmac.MaxSigningKeys {
					fmt.Printf("audit_type=webhook_signing_key:eviction count=1 reason=key_count_exceeded max=%d evicted_count=%d\n",
						cxhmac.MaxSigningKeys, count-cxhmac.MaxSigningKeys)
				}
			}
			signingKeys = nil
		}
	}

	// Snapshot the informer cache under RLock. Non-leader pods fall back
	// to a synchronous configmap read when the cache is unpopulated.
	var cached []string
	var cachedReceivers []webhookEntry
	var cachePopulated bool
	if p.webhookState != nil {
		p.webhookState.mu.RLock()
		cachePopulated = p.webhookState.populated
		cached = append([]string(nil), p.webhookState.urls...)
		cachedReceivers = append([]webhookEntry(nil), p.webhookState.receivers...)
		p.webhookState.mu.RUnlock()
	}

	var entries []webhookEntry
	switch {
	case cachePopulated && len(cachedReceivers) > 0:
		entries = cachedReceivers
	case cachePopulated:
		entries = parseWebhookEntries(cached)
	default:
		whs, err := p.webhookList()
		if err != nil {
			// Fail open — webhook delivery is best-effort; don't 5xx callers.
			fmt.Printf("ns=event_dispatch at=webhook_list_failed error=%q dispatch=skipped\n", err)
			return nil
		}
		for _, wh := range whs {
			timeout := wh.Timeout
			if timeout <= 0 {
				timeout = defaultWebhookTimeout
			}
			entries = append(entries, webhookEntry{Name: wh.Name, URL: wh.URL, Timeout: timeout})
		}
	}

	for _, e := range entries {
		timeout := e.Timeout
		if timeout <= 0 {
			timeout = defaultWebhookTimeout
		}
		go dispatchWebhookSafely(e.URL, msg, signingKeys, timeout)
	}

	return nil
}

// dispatchWebhookSafely wraps dispatch in panic recovery so a bad receiver
// cannot crash the api pod. Logs are host-only redacted.
func dispatchWebhookSafely(url string, body []byte, signingKeys [][]byte, timeout time.Duration) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("ns=event_dispatch at=recover url_host=%s panic=%q\n", redactURLHost(url), r)
		}
	}()

	// SSRF guard — also catches pre-3.24.6 configmap entries with internal IPs.
	if err := webhookSSRFValidator(url); err != nil {
		if _, loaded := webhookSSRFLogged.LoadOrStore(url, true); !loaded {
			fmt.Printf("ns=event_dispatch at=ssrf_blocked url_host=%s reason=%q\n",
				redactURLHost(url), err.Error())
		}
		return
	}

	// Legacy test stub path — unsigned (url, body) signature.
	if isTestDispatchHookActive() {
		if err := dispatchWebhookFn(url, body); err != nil {
			fmt.Printf("ns=event_dispatch at=error url_host=%s error=%q\n", redactURLHost(url), redactErrorURL(err, url))
		}
		return
	}

	if err := dispatchWebhookSignedFn(url, body, signingKeys, timeout); err != nil {
		fmt.Printf("ns=event_dispatch at=error url_host=%s error=%q\n", redactURLHost(url), redactErrorURL(err, url))
	}
}

// redactErrorURL unwraps *url.Error to avoid leaking query strings in logs.
func redactErrorURL(err error, raw string) string {
	if err == nil {
		return ""
	}
	if ue, ok := err.(*neturl.Error); ok {
		return fmt.Sprintf("%s %s: %s", ue.Op, redactURLHost(raw), ue.Err)
	}
	return err.Error()
}

func dispatchWebhook(url string, body []byte) error {
	return dispatchWebhookSigned(url, body, nil, webhookClientTimeout)
}

func dispatchWebhookSigned(url string, body []byte, signingKeys [][]byte, timeout time.Duration) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if len(signingKeys) > 0 {
		sig := cxhmac.SignedHeader(time.Now().Unix(), body, signingKeys)
		if sig != "" {
			req.Header.Set("Convox-Signature", sig)
		}
	}

	// Per-URL timeout override; inherits Transport for test observability.
	client := webhookClient
	if timeout > 0 && timeout != webhookClientTimeout {
		client = &http.Client{Timeout: timeout, Transport: webhookClient.Transport}
	}

	res, err := client.Do(req)
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

// redactedWebhookURL preserves scheme+host (RFC 3986-valid) unlike
// redactURLHost which returns host-only for log lines.
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
