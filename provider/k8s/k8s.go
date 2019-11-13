package k8s

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/metrics"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/logger"
	"github.com/gobuffalo/packr"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Provider struct {
	Atom      atom.Interface
	Config    *rest.Config
	Cluster   kubernetes.Interface
	Domain    string
	Engine    Engine
	Image     string
	Name      string
	Namespace string
	Password  string
	Provider  string
	Router    string
	Socket    string
	Storage   string
	Version   string

	ctx       context.Context
	logger    *logger.Logger
	metrics   *metrics.Metrics
	templater *templater.Templater
}

func FromEnv() (*Provider, error) {
	// hack to make glog stop complaining about flag parsing
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	_ = fs.Parse([]string{})
	flag.CommandLine = fs
	runtime.ErrorHandlers = []func(error){}

	namespace := os.Getenv("NAMESPACE")

	rc, err := restConfig()
	if err != nil {
		return nil, err
	}

	kc, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	ac, err := atom.New(rc)
	if err != nil {
		return nil, err
	}

	ns, err := kc.CoreV1().Namespaces().Get(namespace, am.GetOptions{})
	if err != nil {
		return nil, err
	}

	p := &Provider{
		Atom:      ac,
		Config:    rc,
		Cluster:   kc,
		Domain:    os.Getenv("DOMAIN"),
		Image:     os.Getenv("IMAGE"),
		Name:      ns.Labels["rack"],
		Namespace: ns.Name,
		Password:  os.Getenv("PASSWORD"),
		Provider:  common.CoalesceString(os.Getenv("PROVIDER"), "k8s"),
		Router:    os.Getenv("ROUTER"),
		Socket:    common.CoalesceString(os.Getenv("SOCKET"), "/var/run/docker.sock"),
		Storage:   common.CoalesceString(os.Getenv("STORAGE"), "/var/storage"),
		Version:   common.CoalesceString(os.Getenv("VERSION"), "dev"),
	}

	if err := p.Initialize(structs.ProviderOptions{}); err != nil {
		return nil, err
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

	runtime.ErrorHandlers = []func(error){}

	return nil
}

func (p *Provider) Start() error {
	log := p.logger.At("Initialize")

	dc, err := NewDeploymentController(p)
	if err != nil {
		return log.Error(err)
	}

	ec, err := NewEventController(p)
	if err != nil {
		return log.Error(err)
	}

	nc, err := NewNodeController(p)
	if err != nil {
		return log.Error(err)
	}

	pc, err := NewPodController(p)
	if err != nil {
		return log.Error(err)
	}

	go dc.Run()
	go ec.Run()
	go nc.Run()
	go pc.Run()

	go common.Tick(1*time.Hour, p.heartbeat)

	return log.Success()
}

func (p *Provider) WithContext(ctx context.Context) structs.Provider {
	pp := *p
	pp.ctx = ctx
	return &pp
}

func (p *Provider) heartbeat() error {
	as, err := p.AppList()
	if err != nil {
		return err
	}

	ns, err := p.Cluster.CoreV1().Nodes().List(am.ListOptions{})
	if err != nil {
		return err
	}

	ks, err := p.Cluster.CoreV1().Namespaces().Get("kube-system", am.GetOptions{})
	if err != nil {
		return err
	}

	// "instance_type":  "",
	// "region": ""

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
		return err
	}

	for k, v := range hs {
		ms[k] = v
	}

	if err := p.metrics.Post("heartbeat", ms); err != nil {
		return err
	}

	return nil
}

func restConfig() (*rest.Config, error) {
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}

	data, err := exec.Command("kubectl", "config", "view", "--raw").CombinedOutput()
	if err != nil {
		return nil, err
	}

	cfg, err := clientcmd.NewClientConfigFromBytes(data)
	if err != nil {
		return nil, err
	}

	c, err := cfg.ClientConfig()
	if err != nil {
		return nil, err
	}

	return c, nil
}
