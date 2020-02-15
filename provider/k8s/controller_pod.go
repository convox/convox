package k8s

import (
	"fmt"
	"reflect"
	"time"

	"github.com/convox/convox/pkg/kctl"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PodController struct {
	Controller *kctl.Controller
	Provider   *Provider

	// logger *podLogger
	start time.Time
}

func NewPodController(p *Provider) (*PodController, error) {
	pc := &PodController{
		Provider: p,
		// logger:   NewPodLogger(p),
		start: time.Now().UTC(),
	}

	c, err := kctl.NewController(p.Namespace, "convox-k8s-pod", pc)
	if err != nil {
		return nil, err
	}

	pc.Controller = c

	return pc, nil
}

func (c *PodController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *PodController) Informer() cache.SharedInformer {
	return ic.NewFilteredPodInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *PodController) ListOptions(opts *am.ListOptions) {
	opts.LabelSelector = fmt.Sprintf("system=convox,rack=%s", c.Provider.Name)
}

func (c *PodController) Run() {
	ch := make(chan error)

	go c.Controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *PodController) Start() error {
	c.start = time.Now().UTC()

	return nil
}

func (c *PodController) Stop() error {
	return nil
}

func (c *PodController) Add(obj interface{}) error {
	p, err := assertPod(obj)
	if err != nil {
		return err
	}

	fmt.Printf("pod add %s/%s: %s\n", p.ObjectMeta.Namespace, p.ObjectMeta.Name, p.Status.Phase)

	switch p.Status.Phase {
	case "Succeeded", "Failed":
		go c.cleanupPod(p)
	}

	return nil
}

func (c *PodController) Delete(obj interface{}) error {
	p, err := assertPod(obj)
	if err != nil {
		return err
	}

	fmt.Printf("pod delete %s/%s: %s\n", p.ObjectMeta.Namespace, p.ObjectMeta.Name, p.Status.Phase)

	return nil
}

func (c *PodController) Update(prev, cur interface{}) error {
	pp, err := assertPod(prev)
	if err != nil {
		return err
	}

	cp, err := assertPod(cur)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(pp.Status, cp.Status) {
		return nil
	}

	if pp.Status.Phase != cp.Status.Phase {
		fmt.Printf("pod update %s/%s: %s => %s\n", cp.ObjectMeta.Namespace, cp.ObjectMeta.Name, pp.Status.Phase, cp.Status.Phase)
	}

	if cp.Status.Phase != pp.Status.Phase {
		switch cp.Status.Phase {
		case "Succeeded", "Failed":
			go c.cleanupPod(cp)
		}
	}

	return nil
}

func (c *PodController) cleanupPod(p *ac.Pod) error {
	time.Sleep(5 * time.Second)

	if err := c.Client().CoreV1().Pods(p.ObjectMeta.Namespace).Delete(p.ObjectMeta.Name, nil); err != nil {
		return err
	}

	return nil
}

func assertPod(v interface{}) (*ac.Pod, error) {
	p, ok := v.(*ac.Pod)
	if !ok {
		return nil, fmt.Errorf("could not assert pod for type: %T", v)
	}

	return p, nil
}
