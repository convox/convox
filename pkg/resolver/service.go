package resolver

import (
	"fmt"
	"sync"

	ac "k8s.io/api/core/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Service struct {
	errch    chan error
	hosts    map[string]string
	informer cache.SharedIndexInformer
	lock     sync.Mutex
	stopch   chan struct{}
}

func NewService(kc *kubernetes.Clientset) (*Service, error) {
	s := &Service{
		errch:  make(chan error),
		hosts:  map[string]string{},
		stopch: make(chan struct{}),
	}

	s.informer = ic.NewFilteredServiceInformer(kc, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, listOptions)

	s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.add,
		DeleteFunc: s.delete,
		UpdateFunc: s.update,
	})

	return s, nil
}

func (s *Service) Start() {
	s.informer.Run(s.stopch)

	for err := range s.errch {
		fmt.Printf("err: %+v\n", err)
	}
}

func (s *Service) Stop() {
	close(s.stopch)
}

func (s *Service) IP(host string) (string, bool) {
	if ip, ok := s.hosts[host]; ok {
		return ip, true
	}
	return "", false
}

func (s *Service) add(obj interface{}) {
	cs, err := assertService(obj)
	if err != nil {
		s.errch <- err
		return
	}

	fmt.Printf("ns=service at=add namespace=%s name=%s\n", cs.ObjectMeta.Namespace, cs.ObjectMeta.Name)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.addService(cs)
}

func (s *Service) delete(obj interface{}) {
	cs, err := assertService(obj)
	if err != nil {
		s.errch <- err
		return
	}

	fmt.Printf("ns=service at=delete namespace=%s name=%s\n", cs.ObjectMeta.Namespace, cs.ObjectMeta.Name)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.deleteService(cs)
}

func (s *Service) update(prev, cur interface{}) {
	ps, err := assertService(prev)
	if err != nil {
		s.errch <- err
		return
	}

	cs, err := assertService(cur)
	if err != nil {
		s.errch <- err
		return
	}

	fmt.Printf("ns=service at=update namespace=%s name=%s\n", cs.ObjectMeta.Namespace, cs.ObjectMeta.Name)

	s.lock.Lock()
	defer s.lock.Unlock()

	s.deleteService(ps)
	s.addService(cs)
}

func (s *Service) addService(svc *ac.Service) {
	if host := svc.ObjectMeta.Annotations["convox.com/alias"]; host != "" {
		s.hosts[host] = svc.Spec.ClusterIP
	}
}

func (s *Service) deleteService(svc *ac.Service) {
	if host := svc.ObjectMeta.Annotations["convox.com/alias"]; host != "" {
		delete(s.hosts, host)
	}
}

func assertService(v interface{}) (*ac.Service, error) {
	i, ok := v.(*ac.Service)
	if !ok {
		return nil, fmt.Errorf("could not assert service for type: %T", v)
	}

	return i, nil
}
