package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/convox/convox/pkg/structs"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// webhookConfigMapTimeout bounds the synchronous configmap fetch in
// webhookConfigMap. EventSend on non-leader pods now reads via this
// helper on every event emission; without a deadline a slow / partitioned
// kube-apiserver would hang ReleasePromote / AppCreate / scale-override
// indefinitely waiting for a webhook-list that has never delivered.
// 5s is generous for a single configmap GET — typical p99 is sub-50ms.
var webhookConfigMapTimeout = 5 * time.Second

// defaultWebhookTimeout is the per-receiver dispatch deadline applied when an
// operator does NOT supply a JSON-encoded webhook config that overrides it.
// 30s matches the package-scoped webhookClient timeout that has been the
// production default since 3.24.5; plain-URL configmap entries inherit it
// unchanged so existing operators see byte-identical dispatch behavior.
const defaultWebhookTimeout = 30 * time.Second

type Webhook struct {
	Name string
	URL  string
	// Timeout is the per-receiver dispatch deadline. Zero means "use the
	// package default" (defaultWebhookTimeout); non-zero values come from a
	// JSON-encoded configmap entry of the form `{"url":"...", "timeout":"5s"}`.
	Timeout time.Duration
}

// webhookEntry is the parsed in-memory form of one configmap data entry.
// Distinct from Webhook because the API surface area for `webhookList()` is
// stable and Webhook fields land on the wire (struct shape consumers).
// webhookEntry is a private cache row and may be reshaped without breaking
// downstream callers.
type webhookEntry struct {
	Name    string
	URL     string
	Timeout time.Duration
}

// parseWebhookEntry inspects one raw configmap value and decides whether to
// treat it as a plain URL or a JSON-encoded receiver config. Returns
// (entry, skip=false) when the entry should be dispatched, or (zero,
// skip=true) when the entry is malformed and must be silently dropped (with
// a structured warning log line emitted for operator visibility).
//
// Branch semantics (locked by F-R9-5 review):
//
//   - Empty / whitespace-only raw: SKIP (no entry to dispatch).
//   - Raw begins with `{`: attempt JSON parse.
//   - parse succeeds AND url is non-empty: USE parsed entry. Timeout
//     falls back to defaultWebhookTimeout when absent or unparseable.
//   - parse succeeds AND url is empty/whitespace: SKIP. Per F-R9-5 we
//     do NOT fall through to plain-URL semantics — raw is a JSON object
//     (not a URL string), so treating it as a URL would corrupt
//     dispatch. Operators see a structured WARN.
//   - parse fails (malformed JSON beginning with `{`): SKIP. Same
//     reasoning — raw is not parseable as either form.
//   - Raw does NOT begin with `{`: treat as plain URL. Timeout =
//     defaultWebhookTimeout. Existing operator behavior; identical to
//     pre-2B dispatch.
func parseWebhookEntry(name, raw string) (webhookEntry, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		fmt.Printf("ns=webhook_parse at=skip reason=empty_value name=%s\n", name)
		return webhookEntry{}, true
	}
	if strings.HasPrefix(trimmed, "{") {
		var je struct {
			URL     string `json:"url"`
			Timeout string `json:"timeout"`
		}
		if err := json.Unmarshal([]byte(trimmed), &je); err == nil {
			url := strings.TrimSpace(je.URL)
			if url == "" {
				// F-R9-5: JSON object with empty/missing url field. Do NOT
				// fall through to plain-URL — raw is a JSON object, not a URL.
				fmt.Printf("ns=webhook_parse at=skip reason=empty_url_in_json name=%s\n", name)
				return webhookEntry{}, true
			}
			timeout := defaultWebhookTimeout
			if je.Timeout != "" {
				if d, perr := time.ParseDuration(je.Timeout); perr == nil && d > 0 {
					timeout = d
				}
			}
			return webhookEntry{Name: name, URL: url, Timeout: timeout}, false
		}
		// Begins with `{` but cannot be parsed as JSON: malformed. Skip.
		fmt.Printf("ns=webhook_parse at=skip reason=invalid_json name=%s\n", name)
		return webhookEntry{}, true
	}
	// Plain URL form (3.24.5-compatible). Use the package default timeout.
	return webhookEntry{Name: name, URL: trimmed, Timeout: defaultWebhookTimeout}, false
}

// parseWebhookEntries transforms a slice of raw configmap values (from the
// informer's webhookConfigMapURLs cache) into dispatchable webhookEntry
// rows. Names are not available from the URL-only cache; callers that need
// names should source them from webhookList() directly.
func parseWebhookEntries(rawValues []string) []webhookEntry {
	out := make([]webhookEntry, 0, len(rawValues))
	for _, raw := range rawValues {
		entry, skip := parseWebhookEntry("", raw)
		if skip {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (p *Provider) webhookConfigMap() (*ac.ConfigMap, error) {
	cms := p.Cluster.CoreV1().ConfigMaps(p.Namespace)

	ctx, cancel := context.WithTimeout(context.Background(), webhookConfigMapTimeout)
	defer cancel()

	cm, err := cms.Get(ctx, "webhooks", am.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			cm = &ac.ConfigMap{
				ObjectMeta: am.ObjectMeta{
					Namespace: p.Namespace,
					Name:      "webhooks",
				},
			}

			created, cerr := cms.Create(ctx, cm, am.CreateOptions{})
			if cerr != nil {
				// Non-leader pods may race with each other to Create
				// the missing configmap on first traffic; the loser
				// gets AlreadyExists. Re-Get and proceed; the winner
				// already wrote the (empty) configmap so the second
				// Get returns it.
				if ae.IsAlreadyExists(cerr) {
					if reread, rerr := cms.Get(ctx, "webhooks", am.GetOptions{}); rerr == nil {
						cm = reread
					} else {
						return nil, rerr
					}
				} else {
					return nil, cerr
				}
			} else {
				cm = created
			}
		} else {
			return nil, err
		}
	}

	if cm.Data == nil {
		cm.Data = map[string]string{}
	}

	return cm, nil
}

func (p *Provider) webhookCreate(name, url string) error {
	cm, err := p.webhookConfigMap()
	if err != nil {
		return err
	}

	if name == "" {
		return structs.ErrBadRequest("name required")
	}

	if url == "" {
		return structs.ErrBadRequest("url required")
	}

	if _, ok := cm.Data[name]; ok {
		return structs.ErrConflict("webhook already exists: %s", name)
	}

	cm.Data[name] = url

	if _, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.TODO(), cm, am.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (p *Provider) webhookDelete(name string) error {
	cm, err := p.webhookConfigMap()
	if err != nil {
		return err
	}

	if _, ok := cm.Data[name]; !ok {
		return structs.ErrNotFound("webhook does not exist: %s", name)
	}

	delete(cm.Data, name)

	if _, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.TODO(), cm, am.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (p *Provider) webhookList() ([]Webhook, error) {
	cm, err := p.webhookConfigMap()
	if err != nil {
		return nil, err
	}

	ws := []Webhook{}

	for k, v := range cm.Data {
		entry, skip := parseWebhookEntry(k, v)
		if skip {
			continue
		}
		ws = append(ws, Webhook{
			Name:    entry.Name,
			URL:     entry.URL,
			Timeout: entry.Timeout,
		})
	}

	sort.Slice(ws, func(i, j int) bool { return ws[i].Name < ws[j].Name })

	return ws, nil
}
