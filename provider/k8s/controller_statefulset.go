package k8s

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/kctl"
	"github.com/convox/logger"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type StatefulSetController struct {
	Controller *kctl.Controller
	Provider   *Provider

	logger *logger.Logger
	start  time.Time
}

func NewStatefulSetController(p *Provider) (*StatefulSetController, error) {
	sc := &StatefulSetController{
		Provider: p,
		logger:   logger.New("ns=statefulset-controller"),
		start:    time.Now().UTC(),
	}
	c, err := kctl.NewController(p.Namespace, "convox-k8s-statefulset", sc)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	sc.Controller = c
	return sc, nil
}

func (c *StatefulSetController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *StatefulSetController) Informer() cache.SharedInformer {
	return ic.NewFilteredStatefulSetInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *StatefulSetController) ListOptions(opts *am.ListOptions) {
	opts.LabelSelector = fmt.Sprintf("system=convox,rack=%s", c.Provider.Name)
}

func (c *StatefulSetController) Run() {
	ch := make(chan error)
	go c.Controller.Run(ch)
	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *StatefulSetController) Start() error {
	c.start = time.Now().UTC()
	return nil
}

func (c *StatefulSetController) Stop() error { return nil }

func (c *StatefulSetController) Add(obj interface{}) error {
	return c.sync(obj, false)
}

func (c *StatefulSetController) Delete(obj interface{}) error {
	return c.sync(obj, true)
}

func (c *StatefulSetController) Update(_, cur interface{}) error {
	return c.sync(cur, false)
}

func (c *StatefulSetController) sync(obj interface{}, remove bool) error {
	s, err := assertStatefulSet(obj)
	if err != nil {
		return errors.WithStack(err)
	}
	c.logger.Logf("statefulset sync: %s/%s\n", s.Namespace, s.Name)

	d := deploymentFromStatefulSet(s)
	dc := &DeployController{Provider: c.Provider, logger: c.logger}
	if !dc.isConvoxManaged(d) {
		return nil
	}
	return dc.SyncPDB(d, remove)
}

func assertStatefulSet(obj interface{}) (*apps.StatefulSet, error) {
	switch v := obj.(type) {
	case *apps.StatefulSet:
		return v, nil
	case cache.DeletedFinalStateUnknown:
		return assertStatefulSet(v.Obj)
	default:
		return nil, fmt.Errorf("could not assert statefulset for type: %T", obj)
	}
}

func deploymentFromStatefulSet(s *apps.StatefulSet) *apps.Deployment {
	return &apps.Deployment{
		ObjectMeta: *s.ObjectMeta.DeepCopy(),
		Spec: apps.DeploymentSpec{
			Replicas: s.Spec.Replicas,
			Selector: s.Spec.Selector.DeepCopy(),
			Template: *s.Spec.Template.DeepCopy(),
		},
	}
}
