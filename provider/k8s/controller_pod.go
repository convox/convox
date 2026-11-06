package k8s

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/kctl"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	AnnotationProcessRetain = "convox.com/retain"

	podCleanupDelay     = 5 * time.Second
	podRetainMaxSeconds = 600
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
		return nil, errors.WithStack(err)
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
		return errors.WithStack(err)
	}

	fmt.Printf("pod add: %s/%s (%s)\n", p.ObjectMeta.Namespace, p.ObjectMeta.Name, p.Status.Phase)

	switch p.Status.Phase {
	case "Succeeded", "Failed":
		go c.cleanupPod(p)
	}

	return nil
}

func (c *PodController) Delete(obj interface{}) error {
	p, err := assertPod(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("pod delete: %s/%s\n", p.ObjectMeta.Namespace, p.ObjectMeta.Name)

	return nil
}

func (c *PodController) Update(prev, cur interface{}) error {
	pp, err := assertPod(prev)
	if err != nil {
		return errors.WithStack(err)
	}

	cp, err := assertPod(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	if reflect.DeepEqual(pp.Status, cp.Status) {
		return nil
	}

	if pp.Status.Phase != cp.Status.Phase {
		fmt.Printf("pod update: %s/%s (%s => %s)\n", cp.ObjectMeta.Namespace, cp.ObjectMeta.Name, pp.Status.Phase, cp.Status.Phase)
	}

	if cp.Status.Phase != pp.Status.Phase {
		switch cp.Status.Phase {
		case "Succeeded", "Failed":
			go c.cleanupPod(cp)
		}
	}

	return nil
}

func cleanupDelay(p *ac.Pod, now time.Time) time.Duration {
	retain, err := strconv.Atoi(p.Annotations[AnnotationProcessRetain])
	if err != nil || retain <= 0 {
		return podCleanupDelay
	}

	if retain > podRetainMaxSeconds {
		retain = podRetainMaxSeconds
	}

	// a rack api restart replays Add for pods that are already terminal, so the
	// window has to run from the container's exit rather than from this observation
	from := now
	if css := p.Status.ContainerStatuses; len(css) > 0 && css[0].State.Terminated != nil {
		if t := css[0].State.Terminated.FinishedAt.Time; !t.IsZero() {
			from = t
		}
	}

	if d := from.Add(time.Duration(retain) * time.Second).Sub(now); d > podCleanupDelay {
		return d
	}

	return podCleanupDelay
}

func (c *PodController) cleanupPod(p *ac.Pod) error {
	time.Sleep(cleanupDelay(p, time.Now().UTC()))

	// the pod observed here may already have been replaced by one reusing its
	// name, which statefulsets do, so delete only this exact object
	uid := p.UID

	err := c.Client().CoreV1().Pods(p.Namespace).Delete(context.TODO(), p.Name, am.DeleteOptions{
		Preconditions: &am.Preconditions{UID: &uid},
	})
	switch {
	case err == nil, kerr.IsNotFound(err):
		return nil
	case kerr.IsConflict(err):
		fmt.Printf("pod cleanup skipped: %s/%s: %s\n", p.Namespace, p.Name, err)
		return nil
	default:
		fmt.Printf("pod cleanup failed: %s/%s: %s\n", p.Namespace, p.Name, err)
		return errors.WithStack(err)
	}
}

func assertPod(v interface{}) (*ac.Pod, error) {
	p, ok := v.(*ac.Pod)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert pod for type: %T", v))
	}

	return p, nil
}
