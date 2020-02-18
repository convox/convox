package router

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/kctl"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/api/extensions/v1beta1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ie "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type IngressController struct {
	controller *kctl.Controller
	kc         kubernetes.Interface
	router     BackendRouter
}

func NewIngressController(kc kubernetes.Interface, router BackendRouter, namespace string) (*IngressController, error) {
	ic := &IngressController{kc: kc, router: router}

	c, err := kctl.NewController(namespace, "convox-router-ingress", ic)
	if err != nil {
		return nil, err
	}

	ic.controller = c

	return ic, nil
}

func (c *IngressController) Client() kubernetes.Interface {
	return c.kc
}

func (c *IngressController) Informer() cache.SharedInformer {
	return ie.NewFilteredIngressInformer(c.kc, ac.NamespaceAll, 10*time.Second, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *IngressController) ListOptions(opts *am.ListOptions) {
	opts.LabelSelector = fmt.Sprintf("system=convox")
	opts.ResourceVersion = ""
}

func (c *IngressController) Run() {
	ch := make(chan error)

	go c.controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *IngressController) Start() error {
	return nil
}

func (c *IngressController) Stop() error {
	return nil
}

func (c *IngressController) Add(obj interface{}) error {
	i, err := assertIngress(obj)
	if err != nil {
		return err
	}

	fmt.Printf("ns=controller.ingress at=add ingress=%s\n", i.ObjectMeta.Name)

	if err := c.syncIngress(i); err != nil {
		return err
	}

	if err := c.updateIngressIP(i, "127.0.0.1"); err != nil {
		return err
	}

	return nil
}

func (c *IngressController) Delete(obj interface{}) error {
	i, err := assertIngress(obj)
	if err != nil {
		return err
	}

	fmt.Printf("ns=controller.ingress at=delete ingress=%s\n", i.ObjectMeta.Name)

	if i.ObjectMeta.Annotations["kubernetes.io/ingress.class"] != "convox" {
		return nil
	}

	for _, r := range i.Spec.Rules {
		for _, port := range r.IngressRuleValue.HTTP.Paths {
			target := rulePathTarget(port, i.ObjectMeta)
			// c.controller.Event(i, ac.EventTypeNormal, "TargetDelete", fmt.Sprintf("%s => %s", r.Host, target))
			c.router.TargetRemove(r.Host, target)
		}
	}

	return nil
}

func (c *IngressController) Update(prev, cur interface{}) error {
	ci, err := assertIngress(cur)
	if err != nil {
		return err
	}

	fmt.Printf("ns=controller.ingress at=update ingress=%s\n", ci.ObjectMeta.Name)

	if err := c.syncIngress(ci); err != nil {
		return err
	}

	return nil
}

func (c *IngressController) syncIngress(i *ae.Ingress) error {
	if i.ObjectMeta.Annotations["kubernetes.io/ingress.class"] != "convox" {
		return nil
	}

	for _, r := range i.Spec.Rules {
		for _, port := range r.IngressRuleValue.HTTP.Paths {
			target := rulePathTarget(port, i.ObjectMeta)
			// c.controller.Event(i, ac.EventTypeNormal, "TargetAdd", fmt.Sprintf("%s => %s", r.Host, target))
			c.router.TargetAdd(r.Host, target, i.ObjectMeta.Annotations["convox.com/idles"] == "true")
		}
	}

	return nil
}

func (c *IngressController) updateIngressIP(i *ae.Ingress, ip string) error {
	if is := i.Status.LoadBalancer.Ingress; len(is) == 1 && is[0].IP == ip {
		return nil
	}

	i.Status.LoadBalancer.Ingress = []ac.LoadBalancerIngress{
		{IP: ip},
	}

	if _, err := c.kc.ExtensionsV1beta1().Ingresses(i.ObjectMeta.Namespace).UpdateStatus(i); err != nil {
		return err
	}

	return nil
}

func assertIngress(v interface{}) (*ae.Ingress, error) {
	i, ok := v.(*ae.Ingress)
	if !ok {
		return nil, fmt.Errorf("could not assert ingress for type: %T", v)
	}

	return i, nil
}

func rulePathTarget(port ae.HTTPIngressPath, meta am.ObjectMeta) string {
	proto := "http"

	if p := meta.Annotations["convox.com/backend-protocol"]; p != "" {
		proto = p
	}

	return fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d", proto, port.Backend.ServiceName, meta.Namespace, port.Backend.ServicePort.IntVal)
}
