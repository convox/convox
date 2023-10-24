package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/convox/convox/pkg/kctl"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type EventController struct {
	Controller *kctl.Controller
	Provider   *Provider

	start am.Time
}

func NewEventController(p *Provider) (*EventController, error) {
	pc := &EventController{
		Provider: p,
		start:    am.NewTime(time.Now().UTC()),
	}

	c, err := kctl.NewController(p.Namespace, "convox-k8s-event", pc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	pc.Controller = c

	return pc, nil
}

func (c *EventController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *EventController) Informer() cache.SharedInformer {
	return ic.NewFilteredEventInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *EventController) ListOptions(opts *am.ListOptions) {
}

func (c *EventController) Run() {
	ch := make(chan error)

	go c.Controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *EventController) Start() error {
	return nil
}

func (c *EventController) Stop() error {
	return nil
}

func (c *EventController) Add(obj interface{}) error {
	e, err := assertEvent(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	if e.LastTimestamp.Before(&c.start) {
		return nil
	}

	o := e.InvolvedObject

	kind := fmt.Sprintf("%s/%s", e.InvolvedObject.APIVersion, e.InvolvedObject.Kind)

	switch kind {
	case "apps/v1/Deployment":
		app, err := c.Provider.NamespaceApp(o.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := c.Provider.systemLog(app, o.Name, e.LastTimestamp.Time, e.Message); err != nil {
			return errors.WithStack(err)
		}
	case "apps/v1/ReplicaSet":
		app, err := c.Provider.NamespaceApp(o.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := c.Provider.systemLog(app, o.Name, e.LastTimestamp.Time, e.Message); err != nil {
			return errors.WithStack(err)
		}
	case "atom.convox.com/v1/Atom":
		app, err := c.Provider.NamespaceApp(o.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := c.Provider.systemLog(app, fmt.Sprintf("atom/%s", strings.ReplaceAll(e.InvolvedObject.Name, ".", "/")), e.LastTimestamp.Time, fmt.Sprintf("%s: %s", e.Reason, e.Message)); err != nil {
			return errors.WithStack(err)
		}
	case "autoscaling/v2beta2/HorizontalPodAutoscaler":
		app, err := c.Provider.NamespaceApp(o.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := c.Provider.systemLog(app, o.Name, e.LastTimestamp.Time, e.Message); err != nil {
			return errors.WithStack(err)
		}
	case "autoscaling/v2/HorizontalPodAutoscaler":
		app, err := c.Provider.NamespaceApp(o.Namespace)
		if err != nil {
			return errors.WithStack(err)
		}

		if err := c.Provider.systemLog(app, o.Name, e.LastTimestamp.Time, e.Message); err != nil {
			return errors.WithStack(err)
		}
	case "v1/ConfigMap":
	case "v1/Pod":
		switch e.Reason {
		case "Killing":
		default:
			app, err := c.Provider.NamespaceApp(o.Namespace)
			if err != nil {
				return errors.WithStack(err)
			}

			if err := c.Provider.systemLog(app, o.Name, e.LastTimestamp.Time, e.Message); err != nil {
				return errors.WithStack(err)
			}
		}
	default:
		fmt.Printf("  unhandled event: %s: %s\n", kind, e.Message)
	}

	return nil
}

func (c *EventController) Delete(obj interface{}) error {
	return nil
}

func (c *EventController) Update(prev, cur interface{}) error {
	return nil
}

func assertEvent(v interface{}) (*ac.Event, error) {
	e, ok := v.(*ac.Event)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert deployment for type: %T", v))
	}

	return e, nil
}
