package atom

import (
	"fmt"
	"time"

	ct "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	cv "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	ic "github.com/convox/convox/pkg/atom/pkg/client/informers/externalversions/atom/v1"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/kctl"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type AtomController struct {
	atom       *Client
	controller *kctl.Controller
	convox     cv.Interface
	kubernetes kubernetes.Interface
}

func NewController(cfg *rest.Config) (*AtomController, error) {
	ac, err := New(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cc, err := cv.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	acc := &AtomController{
		atom:       ac,
		convox:     cc,
		kubernetes: kc,
	}

	c, err := kctl.NewController("kube-system", "convox-atom", acc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	acc.controller = c

	return acc, nil
}

func (c *AtomController) Client() kubernetes.Interface {
	return c.kubernetes
}

func (c *AtomController) Informer() cache.SharedInformer {
	return ic.NewFilteredAtomInformer(c.convox, ac.NamespaceAll, 5*time.Second, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *AtomController) ListOptions(opts *am.ListOptions) {
}

func (c *AtomController) Run() {
	ch := make(chan error)

	go c.controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *AtomController) Start() error {
	return nil
}

func (c *AtomController) Stop() error {
	return nil
}

func (c *AtomController) Add(obj interface{}) error {
	return nil
}

func (c *AtomController) Delete(obj interface{}) error {
	return nil
}

func (c *AtomController) Update(prev, cur interface{}) error {
	pa, err := assertAtom(prev)
	if err != nil {
		return errors.WithStack(err)
	}

	ca, err := assertAtom(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	if pa.Status != ca.Status {
		c.atomEvent(ca, "Status", fmt.Sprintf("%s => %s", common.CoalesceString(string(pa.Status), "None"), ca.Status))
		fmt.Printf("ns=atom.controller at=update atom=\"%s/%s\" event=status from=%q to=%q\n", ca.Namespace, ca.Name, pa.Status, ca.Status)
	}

	switch ca.Status {
	case "Failure", "Reverted", "Running":
		return nil
	case "Failed", "Success": // legacy
		return nil
	}

	switch ca.Status {
	case "Rollback":
		if deadline := am.NewTime(time.Now().UTC().Add(-1 * time.Duration(ca.Spec.ProgressDeadlineSeconds) * time.Second)); ca.Started.Before(&deadline) {
			c.atomStatus(ca, "Failure")
			return nil
		}

		success, err := c.atom.check(ca.Namespace, ca.Spec.CurrentVersion)
		if err != nil {
			c.atomStatus(ca, "Failure")
			return errors.WithStack(err)
		}

		if success {
			c.atomStatus(ca, "Reverted")
		}
	case "Updating":
		if deadline := am.NewTime(time.Now().UTC().Add(-1 * time.Duration(ca.Spec.ProgressDeadlineSeconds) * time.Second)); ca.Started.Before(&deadline) {
			c.atomStatus(ca, "Deadline")
			return nil
		}

		success, err := c.atom.check(ca.Namespace, ca.Spec.CurrentVersion)
		if err != nil {
			c.atomStatus(ca, "Error")
			return errors.WithStack(err)
		}

		if success {
			c.atomStatus(ca, "Running")
		}
	}

	if ca.Status == pa.Status {
		return nil
	}

	switch ca.Status {
	case "Cancelled", "Deadline", "Error":
		if err := c.atom.rollback(ca); err != nil {
			c.atomStatus(ca, "Failure")
			return errors.WithStack(err)
		}

		c.atomStatus(ca, "Rollback")
	case "Pending":
		if err := c.atom.apply(ca); err != nil {
			c.atomStatus(ca, "Error")
			return err
		}

		c.atomStatus(ca, "Updating")
	}

	return nil
}

func (c *AtomController) atomEvent(a *ct.Atom, reason, message string) error {
	ts := am.Now()

	e := &ac.Event{
		Count:          1,
		Message:        message,
		Reason:         reason,
		FirstTimestamp: ts,
		LastTimestamp:  ts,
		Type:           "Normal",
		InvolvedObject: ac.ObjectReference{
			APIVersion: "atom.convox.com/v1",
			Kind:       "Atom",
			Name:       a.Name,
			Namespace:  a.Namespace,
			UID:        a.UID,
		},
		ObjectMeta: am.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", a.Name),
		},
		Source: ac.EventSource{
			Component: "convox.atom",
		},
	}

	if _, err := c.kubernetes.CoreV1().Events(a.Namespace).Create(e); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *AtomController) atomStatus(a *ct.Atom, status string) {
	_, err := c.atom.update(a, func(aa *ct.Atom) {
		aa.Status = ct.AtomStatus(status)
	})
	if err != nil {
		fmt.Printf("ns=atom.controller at=status.update atom=\"%s/%s\" status=%s error=%q\n", a.Namespace, a.Name, status, err)
	}
}

func assertAtom(v interface{}) (*ct.Atom, error) {
	a, ok := v.(*ct.Atom)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert atom for type: %T", v))
	}

	return a, nil
}
