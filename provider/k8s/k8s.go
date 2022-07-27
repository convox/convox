package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/metrics"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/convox/logger"
	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"

	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type Provider struct {
	Atom          atom.Interface
	CertManager   bool
	Config        *rest.Config
	Convox        cv.Interface
	Cluster       kubernetes.Interface
	Domain        string
	Engine        Engine
	Image         string
	Name          string
	MetricsClient metricsclientset.Interface
	Namespace     string
	Password      string
	Provider      string
	Resolver      string
	Router        string
	Socket        string
	Storage       string
	Version       string

	ctx       context.Context
	logger    *logger.Logger
	metrics   *metrics.Metrics
	templater *templater.Templater
	webhooks  []string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func FromEnv() (*Provider, error) {
	namespace := os.Getenv("NAMESPACE")

	rc, err := restConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	kc, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ac, err := atom.New(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ns, err := kc.CoreV1().Namespaces().Get(namespace, am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cc, err := cv.NewForConfig(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	mc, err := metricsclientset.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Atom:          ac,
		CertManager:   os.Getenv("CERT_MANAGER") == "true",
		Config:        rc,
		Convox:        cc,
		Cluster:       kc,
		Domain:        os.Getenv("DOMAIN"),
		Image:         os.Getenv("IMAGE"),
		MetricsClient: mc,
		Name:          ns.Labels["rack"],
		Namespace:     ns.Name,
		Password:      os.Getenv("PASSWORD"),
		Provider:      common.CoalesceString(os.Getenv("PROVIDER"), "k8s"),
		Resolver:      os.Getenv("RESOLVER"),
		Router:        os.Getenv("ROUTER"),
		Socket:        common.CoalesceString(os.Getenv("SOCKET"), "/var/run/docker.sock"),
		Storage:       common.CoalesceString(os.Getenv("STORAGE"), "/var/storage"),
		Version:       common.CoalesceString(os.Getenv("VERSION"), "dev"),
	}

	return p, nil
}

func (p *Provider) Context() context.Context {
	return p.ctx
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	p.ctx = context.Background()
	p.logger = logger.New("ns=k8s")
	p.metrics = metrics.New("https://metrics.convox.com/metrics/rack")
	p.templater = templater.New(packr.NewBox("../k8s/template"), p.templateHelpers())
	p.webhooks = []string{}

	if os.Getenv("TEST") == "true" {
		return nil
	}

	if err := atom.Initialize(); err != nil {
		return errors.WithStack(err)
	}

	// if err := p.initializeTemplates(); err != nil {
	// 	return errors.WithStack(err)
	// }

	go p.initializeTemplates()

	if !opts.IgnorePriorityClass {
		if err := p.initializePriorityClass(); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func (p *Provider) Start() error {
	log := p.logger.At("Start")

	ec, err := NewEventController(p)
	if err != nil {
		return errors.WithStack(log.Error(err))
	}

	pc, err := NewPodController(p)
	if err != nil {
		return errors.WithStack(log.Error(err))
	}

	wc, err := NewWebhookController(p)
	if err != nil {
		return errors.WithStack(log.Error(err))
	}

	go ec.Run()
	go pc.Run()
	go wc.Run()

	go common.Tick(1*time.Hour, p.heartbeat)

	go p.startApiProxy()

	return log.Success()
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.ctx = ctx
	return &pp
}

func (p *Provider) applySystemTemplate(name string, params map[string]interface{}) error {
	data, err := p.RenderTemplate(fmt.Sprintf("system/%s", name), params)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := Apply(data); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) deleteSystemTemplate(name string, params map[string]interface{}) error {
	data, err := p.RenderTemplate(fmt.Sprintf("system/%s", name), params)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := Delete(data); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) heartbeat() error {
	as, err := p.AppList()
	if err != nil {
		return errors.WithStack(err)
	}

	ns, err := p.Cluster.CoreV1().Nodes().List(am.ListOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	ks, err := p.Cluster.CoreV1().Namespaces().Get("kube-system", am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	ms := map[string]interface{}{
		"id":             ks.UID,
		"app_count":      len(as),
		"generation":     "3",
		"instance_count": len(ns.Items),
		"provider":       p.Provider,
		"version":        p.Version,
	}

	hs, err := p.Engine.Heartbeat()
	if err != nil {
		return errors.WithStack(err)
	}

	for k, v := range hs {
		ms[k] = v
	}

	if err := p.metrics.Post("heartbeat", ms); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) initializePriorityClass() error {
	patches := []Patch{
		{Op: "add", Path: "/spec/template/spec/priorityClassName", Value: "system-cluster-critical"},
	}

	patch, err := json.Marshal(patches)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := p.Cluster.AppsV1().Deployments(p.Namespace).Patch("api", types.JSONPatchType, patch); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) initializeTemplates() error {
	if err := p.applySystemTemplate("crd", nil); err != nil {
		return errors.WithStack(err)
	}

	d, _ := p.Cluster.AppsV1().Deployments("cert-manager").Get("cert-manager", am.GetOptions{})

	// println(d.Spec.Template.Labels["app.kubernetes.io/version"])

	if p.CertManager {
		if d != nil && d.Spec.Template.Labels["app.kubernetes.io/version"] != "v1.9.0" {
			println("updating cert-manager...")
			p.deleteSystemTemplate("cert-manager", nil)

			// println(">>> waiting for namespace to be deleted")
			// TODO Query the cert-manager namespace and return when it's deleted
			time.Sleep(time.Second * 20)

			// println(">>> re-applying cert-manager resources")
			err := p.applySystemTemplate("cert-manager", nil)
			if err != nil {
				return err
			}

			println("update finished")
		} else if d == nil {
			err := p.applySystemTemplate("cert-manager", nil)
			if err != nil {
				return err
			}

		}

		go p.installCertManagerConfig()
	}

	return nil
}

func (p *Provider) installCertManagerConfig() {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	timeout := time.NewTimer(10 * time.Minute)
	defer timeout.Stop()

	fmt.Printf("waiting for cert manager webhook deployment\n")

	for {
		select {
		case <-tick.C:
			d, err := p.Cluster.AppsV1().Deployments("cert-manager").Get("cert-manager-webhook", am.GetOptions{})
			if err != nil {
				fmt.Printf("could not get cert manager webhook deployment: %s\n", err)
				continue
			}

			for _, c := range d.Status.Conditions {
				if c.Type == "Available" && c.Status == "True" {
					fmt.Printf("installing cert manager letsencrypt config\n")

					if err := p.applySystemTemplate("cert-manager-letsencrypt", nil); err != nil {
						fmt.Printf("could not install cert manager letsencrypt config: %s\n", err)
						break
					}

					if cas, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get("ca", am.GetOptions{}); err == nil {
						params := map[string]interface{}{
							"CaPublic":  base64.StdEncoding.EncodeToString(cas.Data["tls.crt"]),
							"CaPrivate": base64.StdEncoding.EncodeToString(cas.Data["tls.key"]),
						}

						fmt.Printf("installing cert manager letsencrypt config\n")

						if err := p.applySystemTemplate("cert-manager-self-signed", params); err != nil {
							fmt.Printf("could not install cert manager letsencrypt config: %s\n", err)
							break
						}

					}

					return
				}
			}
		case <-timeout.C:
			fmt.Printf("error: timeout installing cluster issuer\n")
			return
		}
	}
}

func (p *Provider) startApiProxy() {
	ap, err := p.apiProxy()
	if err != nil {
		fmt.Printf("error: could not create kubernetes proxy listener: %v\n", err)
		return
	}

	if err := ap.ListenAndServe("0.0.0.0", 8001); err != nil {
		fmt.Printf("error: could not start kubernetes proxy listener: %v\n", err)
		return
	}
}

func restConfig() (*rest.Config, error) {
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}

	data, err := exec.Command("kubectl", "config", "view", "--raw").CombinedOutput()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cfg, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c, err := cfg.ClientConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return c, nil
}
