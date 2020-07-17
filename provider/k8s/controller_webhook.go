package k8s

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/convox/convox/pkg/kctl"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type WebhookController struct {
	Controller *kctl.Controller
	Provider   *Provider
}

func NewWebhookController(p *Provider) (*WebhookController, error) {
	pc := &WebhookController{
		Provider: p,
	}

	c, err := kctl.NewController(p.Namespace, "convox-k8s-webhook", pc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pc.Controller = c

	return pc, nil
}

func (c *WebhookController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *WebhookController) Informer() cache.SharedInformer {
	return ic.NewFilteredConfigMapInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *WebhookController) ListOptions(opts *am.ListOptions) {
	opts.FieldSelector = "metadata.name=webhooks"
}

func (c *WebhookController) Run() {
	ch := make(chan error)

	go c.Controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *WebhookController) Start() error {
	return nil
}

func (c *WebhookController) Stop() error {
	return nil
}

func (c *WebhookController) Add(obj interface{}) error {
	cm, err := assertConfigMap(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Provider.webhooks = webhookConfigMapURLs(cm)

	return nil
}

func (c *WebhookController) Delete(obj interface{}) error {
	cm, err := assertConfigMap(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	c.Provider.webhooks = webhookConfigMapURLs(cm)

	return nil
}

func (c *WebhookController) Update(prev, cur interface{}) error {
	pcm, err := assertConfigMap(prev)
	if err != nil {
		return errors.WithStack(err)
	}

	ccm, err := assertConfigMap(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	if reflect.DeepEqual(pcm, ccm) {
		return nil
	}

	c.Provider.webhooks = webhookConfigMapURLs(ccm)

	return nil
}

func assertConfigMap(v interface{}) (*ac.ConfigMap, error) {
	p, ok := v.(*ac.ConfigMap)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert pod for type: %T", v))
	}

	return p, nil
}

func webhookConfigMapURLs(cm *ac.ConfigMap) []string {
	us := []string{}

	if cm.Data == nil {
		return us
	}

	for _, v := range cm.Data {
		us = append(us, v)
	}

	sort.Strings(us)

	return us
}
