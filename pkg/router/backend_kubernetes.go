package router

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	aa "k8s.io/api/autoscaling/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type BackendKubernetes struct {
	cluster   kubernetes.Interface
	idled     map[string]bool
	idledLock sync.Mutex
	namespace string
	router    BackendRouter
}

func NewBackendKubernetes(router BackendRouter, namespace string) (*BackendKubernetes, error) {
	fmt.Printf("ns=backend.k8s fn=new namespace=%s\n", namespace)

	b := &BackendKubernetes{namespace: namespace, router: router}

	fmt.Printf("ns=backend.k8s fn=new at=rest.config\n")

	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	fmt.Printf("ns=backend.k8s fn=new at=k8s.config\n")

	kc, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	b.cluster = kc
	b.idled = map[string]bool{}

	return b, nil
}

func (b *BackendKubernetes) Start() error {
	dc, err := NewDeploymentController(b.cluster, b, b.namespace)
	if err != nil {
		return err
	}

	go dc.Run()

	ic, err := NewIngressController(b.cluster, b.router, b.namespace)
	if err != nil {
		return err
	}

	go ic.Run()

	return nil
}

func (b *BackendKubernetes) CA() (*tls.Certificate, error) {
	c, err := b.cluster.CoreV1().Secrets(b.namespace).Get("ca", am.GetOptions{})
	if err != nil {
		return nil, err
	}

	ca, err := tls.X509KeyPair(c.Data["tls.crt"], c.Data["tls.key"])
	if err != nil {
		return nil, err
	}

	return &ca, nil
}

func (b *BackendKubernetes) IdleGet(target string) (bool, error) {
	fmt.Printf("ns=backend.k8s at=idle.get target=%q\n", target)

	if service, namespace, ok := parseTarget(target); ok {
		key := fmt.Sprintf("%s/%s", namespace, service)

		if idle, ok := b.idled[key]; ok {
			return idle, nil
		}

		d, err := b.cluster.AppsV1().Deployments(namespace).Get(service, am.GetOptions{})
		if err != nil {
			return false, err
		}

		b.idled[key] = (d.Spec.Replicas == nil || int(*d.Spec.Replicas) == 0)

		return b.idled[key], nil
	}

	return true, nil
}

func (b *BackendKubernetes) IdleSet(target string, idle bool) error {
	if idle {
		return b.idle(target)
	} else {
		return b.unidle(target)
	}
}

func (b *BackendKubernetes) IdleUpdate(namespace, service string, idle bool) error {
	b.idledLock.Lock()
	defer b.idledLock.Unlock()

	fmt.Printf("ns=backend.k8s at=idle.update namespace=%q service=%q idle=%t\n", namespace, service, idle)

	b.idled[fmt.Sprintf("%s/%s", namespace, service)] = idle

	return nil
}

func (b *BackendKubernetes) idle(target string) error {
	fmt.Printf("ns=backend.k8s at=idle target=%q\n", target)

	if service, namespace, ok := parseTarget(target); ok {
		scale := &aa.Scale{
			ObjectMeta: am.ObjectMeta{
				Namespace: namespace,
				Name:      service,
			},
			Spec: aa.ScaleSpec{Replicas: 0},
		}

		if _, err := b.cluster.AppsV1().Deployments(namespace).UpdateScale(service, scale); err != nil {
			fmt.Printf("ns=backend.k8s at=idle target=%q error=%q\n", target, err)
		}
	}

	return nil
}

func (b *BackendKubernetes) unidle(target string) error {
	fmt.Printf("ns=backend.k8s at=unidle target=%q state=unidling\n", target)

	if service, namespace, ok := parseTarget(target); ok {
		scale := &aa.Scale{
			ObjectMeta: am.ObjectMeta{
				Namespace: namespace,
				Name:      service,
			},
			Spec: aa.ScaleSpec{Replicas: 1},
		}

		if _, err := b.cluster.AppsV1().Deployments(namespace).UpdateScale(service, scale); err != nil {
			fmt.Printf("ns=backend.k8s at=unidle target=%q error=%q\n", target, err)
		}

		for {
			time.Sleep(200 * time.Millisecond)
			if rs, err := b.cluster.AppsV1().Deployments(namespace).Get(service, am.GetOptions{}); err == nil {
				if rs.Status.AvailableReplicas > 0 {
					break
				}
			}
		}
	}

	fmt.Printf("ns=backend.k8s at=unidle target=%q state=ready\n", target)

	return nil
}
