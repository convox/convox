package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/validator"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SSRF guard — applied at both create and dispatch time. Tests override
// via SetWebhookSSRFValidatorForTest for loopback httptest URLs.
var webhookSSRFValidator = func(raw string) error {
	return validator.ValidateExternalURL(raw, nil)
}

// webhookSSRFLogged deduplicates SSRF block log lines (one per URL per pod lifetime).
var webhookSSRFLogged sync.Map

var webhookConfigMapTimeout = 5 * time.Second

const defaultWebhookTimeout = 30 * time.Second

type Webhook struct {
	Name    string
	URL     string
	Timeout time.Duration
}

type webhookEntry struct {
	Name    string
	URL     string
	Timeout time.Duration
}

// parseWebhookEntry parses a configmap value as either JSON `{"url":..., "timeout":...}`
// or a plain URL string. Returns (entry, skip). JSON values with empty/missing url
// or parse errors are skipped; plain strings use defaultWebhookTimeout.
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
		fmt.Printf("ns=webhook_parse at=skip reason=invalid_json name=%s\n", name)
		return webhookEntry{}, true
	}
	return webhookEntry{Name: name, URL: trimmed, Timeout: defaultWebhookTimeout}, false
}

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
				// Race: another pod created it first.
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
	if name == "" {
		return structs.ErrBadRequest("name required")
	}

	if url == "" {
		return structs.ErrBadRequest("url required")
	}

	// SSRF guard — rejects internal/IMDS/loopback IPs; allows .svc.cluster.local.
	if err := webhookSSRFValidator(url); err != nil {
		return structs.ErrBadRequest("url: %s", err)
	}

	cm, err := p.webhookConfigMap()
	if err != nil {
		return err
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
