package resolver

import (
	"fmt"
	"sync"

	ac "k8s.io/api/core/v1"
	ae "k8s.io/api/extensions/v1beta1"
	ie "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Ingress struct {
	errch    chan error
	hosts    map[string]bool
	informer cache.SharedIndexInformer
	lock     sync.Mutex
	stopch   chan struct{}
}

func NewIngress(kc *kubernetes.Clientset) (*Ingress, error) {
	i := &Ingress{
		errch:  make(chan error),
		hosts:  map[string]bool{},
		stopch: make(chan struct{}),
	}

	i.informer = ie.NewFilteredIngressInformer(kc, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, listOptions)

	i.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    i.add,
		DeleteFunc: i.delete,
		UpdateFunc: i.update,
	})

	return i, nil
}

func (i *Ingress) Start() {
	i.informer.Run(i.stopch)

	for err := range i.errch {
		fmt.Printf("err: %+v\n", err)
	}
}

func (i *Ingress) Stop() {
	close(i.stopch)
}

func (i *Ingress) HostExists(host string) bool {
	i.lock.Lock()
	defer i.lock.Unlock()

	return i.hosts[host]
}

func (i *Ingress) add(obj interface{}) {
	ci, err := assertIngress(obj)
	if err != nil {
		i.errch <- err
		return
	}

	fmt.Printf("ns=ingress at=add namespace=%s name=%s\n", ci.ObjectMeta.Namespace, ci.ObjectMeta.Name)

	i.lock.Lock()
	defer i.lock.Unlock()

	i.addIngress(ci)
}

func (i *Ingress) delete(obj interface{}) {
	ci, err := assertIngress(obj)
	if err != nil {
		i.errch <- err
		return
	}

	fmt.Printf("ns=ingress at=delete namespace=%s name=%s\n", ci.ObjectMeta.Namespace, ci.ObjectMeta.Name)

	i.lock.Lock()
	defer i.lock.Unlock()

	i.deleteIngress(ci)
}

func (i *Ingress) update(prev, cur interface{}) {
	pi, err := assertIngress(prev)
	if err != nil {
		i.errch <- err
		return
	}

	ci, err := assertIngress(cur)
	if err != nil {
		i.errch <- err
		return
	}

	fmt.Printf("ns=ingress at=update namespace=%s name=%s\n", ci.ObjectMeta.Namespace, ci.ObjectMeta.Name)

	i.lock.Lock()
	defer i.lock.Unlock()

	i.deleteIngress(pi)
	i.addIngress(ci)
}

func (i *Ingress) addIngress(in *ae.Ingress) {
	for _, r := range in.Spec.Rules {
		i.hosts[r.Host] = true
	}
}

func (i *Ingress) deleteIngress(in *ae.Ingress) {
	for _, r := range in.Spec.Rules {
		delete(i.hosts, r.Host)
	}
}

func assertIngress(v interface{}) (*ae.Ingress, error) {
	i, ok := v.(*ae.Ingress)
	if !ok {
		return nil, fmt.Errorf("could not assert ingress for type: %T", v)
	}

	return i, nil
}
