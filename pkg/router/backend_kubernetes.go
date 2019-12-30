package router

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	ae "k8s.io/api/extensions/v1beta1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type BackendKubernetes struct {
	cluster   kubernetes.Interface
	idled     map[string]bool
	idledLock sync.Mutex
	ip        string
	namespace string
	prefix    string
	router    BackendRouter
	service   string
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

	if parts := strings.Split(os.Getenv("POD_IP"), "."); len(parts) > 2 {
		b.prefix = fmt.Sprintf("%s.%s.", parts[0], parts[1])
	}

	fmt.Printf("ns=backend.k8s fn=new at=host.resolve\n")

	if host := os.Getenv("SERVICE_HOST"); host != "" {
		for {
			if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
				b.service = ips[0].String()
				break
			}

			time.Sleep(1 * time.Second)
		}
	}

	fmt.Printf("ns=backend.k8s fn=new at=router.ip\n")

	s, err := kc.CoreV1().Services(b.namespace).Get("router", am.GetOptions{})
	if err != nil {
		return nil, err
	}

	if len(s.Status.LoadBalancer.Ingress) > 0 && s.Status.LoadBalancer.Ingress[0].Hostname == "localhost" {
		b.ip = "127.0.0.1"
	} else {
		b.ip = s.Spec.ClusterIP
	}

	fmt.Printf("ns=backend.k8s fn=new at=done ip=%s prefix=%s service=%s\n", b.ip, b.prefix, b.service)

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

func (b *BackendKubernetes) ExternalIP() string {
	return b.ip
}

func (b *BackendKubernetes) InternalIP() string {
	return b.service
}

func (b *BackendKubernetes) IdleGet(target string) (bool, error) {
	fmt.Printf("ns=backend.k8s at=idle.get target=%q\n", target)

	if service, namespace, ok := parseTarget(target); ok {
		key := fmt.Sprintf("%s/%s", namespace, service)

		if idle, ok := b.idled[key]; ok {
			return idle, nil
		}

		d, err := b.cluster.ExtensionsV1beta1().Deployments(namespace).Get(service, am.GetOptions{})
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
		scale := &ae.Scale{
			ObjectMeta: am.ObjectMeta{
				Namespace: namespace,
				Name:      service,
			},
			Spec: ae.ScaleSpec{Replicas: 0},
		}

		if _, err := b.cluster.ExtensionsV1beta1().Deployments(namespace).UpdateScale(service, scale); err != nil {
			fmt.Printf("ns=backend.k8s at=idle target=%q error=%q\n", target, err)
		}
	}

	return nil
}

func (b *BackendKubernetes) unidle(target string) error {
	fmt.Printf("ns=backend.k8s at=unidle target=%q state=unidling\n", target)

	if service, namespace, ok := parseTarget(target); ok {
		scale := &ae.Scale{
			ObjectMeta: am.ObjectMeta{
				Namespace: namespace,
				Name:      service,
			},
			Spec: ae.ScaleSpec{Replicas: 1},
		}

		if _, err := b.cluster.ExtensionsV1beta1().Deployments(namespace).UpdateScale(service, scale); err != nil {
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
