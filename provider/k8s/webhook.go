package k8s

import (
	"context"
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

type Webhook struct {
	Name string
	URL  string
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
		ws = append(ws, Webhook{
			Name: k,
			URL:  v,
		})
	}

	sort.Slice(ws, func(i, j int) bool { return ws[i].Name < ws[j].Name })

	return ws, nil
}
