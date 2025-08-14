package kctl

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	tc "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
)

type Controller struct {
	Handler    ControllerHandler
	Identifier string
	Name       string
	Namespace  string

	cancel   context.CancelFunc
	ctx      context.Context
	errch    chan error
	recorder record.EventRecorder
	stopper  chan struct{}
	IsLeader atomic.Bool
}

type ControllerHandler interface {
	Add(interface{}) error
	Informer() cache.SharedInformer
	Client() kubernetes.Interface
	Delete(interface{}) error
	Start() error
	Stop() error
	Update(interface{}, interface{}) error
}

func NewController(namespace, name string, handler ControllerHandler) (*Controller, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	c := &Controller{
		Handler:    handler,
		Identifier: hostname,
		Name:       name,
		Namespace:  namespace,
		IsLeader:   atomic.Bool{},
	}

	return c, nil
}

func (c *Controller) Event(object runtime.Object, eventtype, reason, message string) {
	c.recorder.Event(object, eventtype, reason, message)
}

func (c *Controller) Run(ch chan error) {
	c.errch = ch

	go c.start()
}

func (c *Controller) leaderStart(informer cache.SharedInformer) func(ctx context.Context) {
	return func(ctx context.Context) {
		fmt.Printf("started leading: %s/%s (%s)\n", c.Namespace, c.Name, c.Identifier)

		c.IsLeader.Store(true)

		if err := c.Handler.Start(); err != nil {
			c.errch <- err
		}

		c.stopper = make(chan struct{})

		go informer.Run(c.stopper)
	}
}

func (c *Controller) leaderStop() {
	fmt.Printf("stopped leading: %s/%s (%s)\n", c.Namespace, c.Name, c.Identifier)
	c.IsLeader.Store(false)
	close(c.stopper)
	c.stopper = nil

	if err := c.Handler.Stop(); err != nil {
		c.errch <- err
	}

	c.cancel()
	go c.start()
}

func (c *Controller) addHandler(obj interface{}) {
	if err := c.Handler.Add(obj); err != nil {
		c.errch <- err
	}
}

func (c *Controller) deleteHandler(obj interface{}) {
	if err := c.Handler.Delete(obj); err != nil {
		c.errch <- err
	}
}

func (c *Controller) start() {
	fmt.Printf("starting elector: %s/%s (%s)\n", c.Namespace, c.Name, c.Identifier)

	eb := record.NewBroadcaster()
	eb.StartRecordingToSink(&tc.EventSinkImpl{Interface: c.Handler.Client().CoreV1().Events("")})

	c.recorder = eb.NewRecorder(scheme.Scheme, ac.EventSource{Component: c.Name})

	rl := &resourcelock.LeaseLock{
		LeaseMeta: am.ObjectMeta{Namespace: c.Namespace, Name: c.Name},
		Client:    c.Handler.Client().CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      c.Identifier,
			EventRecorder: c.recorder,
		},
	}

	informer := c.Handler.Informer()

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addHandler,
		DeleteFunc: c.deleteHandler,
		UpdateFunc: c.updateHandler,
	})

	el, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 60 * time.Second,
		RenewDeadline: 15 * time.Second,
		RetryPeriod:   5 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: c.leaderStart(informer),
			OnStoppedLeading: c.leaderStop,
		},
	})
	if err != nil {
		c.errch <- err
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	c.ctx = context.Background()
	c.cancel = cancel

	go el.Run(ctx)
}

func (c *Controller) updateHandler(prev, cur interface{}) {
	if err := c.Handler.Update(prev, cur); err != nil {
		c.errch <- err
	}
}
