package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Webhook struct {
	Name string
	URL  string
}

func (p *Provider) webhookConfigMap() (*ac.ConfigMap, error) {
	cms := p.Cluster.CoreV1().ConfigMaps(p.Namespace)

	cm, err := cms.Get(context.Background(), "webhooks", am.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			cm = &ac.ConfigMap{
				ObjectMeta: am.ObjectMeta{
					Namespace: p.Namespace,
					Name:      "webhooks",
				},
			}

			if _, err := cms.Create(context.Background(), cm, am.CreateOptions{}); err != nil {
				return nil, err
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
		return fmt.Errorf("name required")
	}

	if url == "" {
		return fmt.Errorf("url required")
	}

	if _, ok := cm.Data[name]; ok {
		return fmt.Errorf("webhook already exists: %s", name)
	}

	cm.Data[name] = url

	if _, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.Background(), cm, am.UpdateOptions{}); err != nil {
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
		return fmt.Errorf("webhook does not exist: %s", name)
	}

	delete(cm.Data, name)

	if _, err := p.Cluster.CoreV1().ConfigMaps(p.Namespace).Update(context.Background(), cm, am.UpdateOptions{}); err != nil {
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
