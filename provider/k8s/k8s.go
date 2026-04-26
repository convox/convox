package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	cxhmac "github.com/convox/convox/pkg/hmac"
	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/metrics"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/pkg/templater"
	"github.com/convox/convox/provider/aws/provisioner/elasticache"
	"github.com/convox/convox/provider/aws/provisioner/rds"
	cv "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned"
	"github.com/convox/convox/provider/k8s/template"
	"github.com/convox/logger"
	"github.com/pkg/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	cmclient "github.com/cert-manager/cert-manager/pkg/client/clientset/versioned"
	cinformer "github.com/convox/convox/provider/k8s/pkg/client/informers/externalversions/convox/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	informerappsv1 "k8s.io/client-go/informers/apps/v1"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	CURRENT_CM_VERSION     = "v1.10.3"
	MAX_RETRIES_UPDATE_CM  = 10
	CERT_MANAGER_NAMESPACE = "cert-manager"
)

type Provider struct {
	Atom                                atom.Interface
	BuildkitEnabled                     string
	BuildNodeEnabled                    string
	CertManager                         bool
	CertManagerRoleArn                  string
	Cluster                             kubernetes.Interface
	Config                              *rest.Config
	Convox                              cv.Interface
	ConvoxDomainTLSCertDisable          bool
	CertManagerClient                   cmclient.Interface
	DiscoveryClient                     discovery.DiscoveryInterface
	DockerUsername                      string
	DockerPassword                      string
	EcrDockerHubCachePrefix             string
	Domain                              string
	DomainInternal                      string
	DynamicClient                       dynamic.Interface
	EfsFileSystemId                     string
	AzureFilesEnabled                   string
	Engine                              Engine
	Image                               string
	JwtMngr                             *jwt.JwtManager
	IsKedaEnabled                       bool
	IsVpaEnabled                        bool
	Name                                string
	ReleasesToRetainAfterActive         int
	ReleasesToRetainTaskRunIntervalHour int
	MetricScraper                       *MetricScraperClient
	MetricsClient                       metricsclientset.Interface
	Namespace                           string
	Password                            string
	PdbDefaultMinAvailablePercentage    string
	Provider                            string
	RackName                            string
	Resolver                            string
	BuildDisableResolver                bool
	RestClient                          rest.Interface
	Router                              string
	Socket                              string
	Storage                             string
	SubnetIDs                           string
	Version                             string
	VpcID                               string
	WebhookSigningKey                   string
	FeatureGates                        map[string]bool

	nc                 *NodeController
	namespaceInformer  informerv1.NamespaceInformer
	nodeInformer       informerv1.NodeInformer
	podInformer        informerv1.PodInformer
	deploymentInformer informerappsv1.DeploymentInformer
	buildInformer      cinformer.BuildInformer
	releaseInformer    cinformer.ReleaseInformer

	RdsProvisioner         *rds.Provisioner
	ElasticacheProvisioner *elasticache.Provisioner

	ctx       context.Context
	logger    *logger.Logger
	metrics   *metrics.Metrics
	templater *templater.Templater
	webhooks  []string
}

func init() {
	rand.Seed(time.Now().Unix())
}

func FromEnv() (*Provider, error) {
	namespace := os.Getenv("NAMESPACE")

	rc, err := restConfig()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rc.QPS = 25
	rc.Burst = 50

	kc, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ac, err := atom.New(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	ns, err := kc.CoreV1().Namespaces().Get(context.TODO(), namespace, am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	dc, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	cc, err := cv.NewForConfig(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	cm, err := cmclient.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	mc, err := metricsclientset.NewForConfig(rc)
	if err != nil {
		return nil, err
	}

	ms := NewMetricScraperClient(kc, os.Getenv("METRICS_SCRAPER_HOST"))

	rn := common.CoalesceString(os.Getenv("RACK_NAME"), ns.Labels["rack"])

	p := &Provider{
		Atom:                             ac,
		BuildkitEnabled:                  "true",
		BuildNodeEnabled:                 os.Getenv("BUILD_NODE_ENABLED"),
		BuildDisableResolver:             os.Getenv("BUILD_DISABLE_CONVOX_RESOLVER") == "true",
		CertManager:                      os.Getenv("CERT_MANAGER") == "true",
		CertManagerRoleArn:               os.Getenv("CERT_MANAGER_ROLE_ARN"),
		Cluster:                          kc,
		Config:                           rc,
		Convox:                           cc,
		ConvoxDomainTLSCertDisable:       os.Getenv("CONVOX_DOMAIN_TLS_CERT_DISABLE") == "true",
		CertManagerClient:                cm,
		DiscoveryClient:                  kc.Discovery(),
		Domain:                           os.Getenv("DOMAIN"),
		DomainInternal:                   os.Getenv("DOMAIN_INTERNAL"),
		DynamicClient:                    dc,
		EfsFileSystemId:                  os.Getenv("EFS_FILE_SYSTEM_ID"),
		AzureFilesEnabled:                os.Getenv("AZURE_FILES_ENABLED"),
		Image:                            os.Getenv("IMAGE"),
		MetricScraper:                    ms,
		MetricsClient:                    mc,
		Name:                             ns.Labels["rack"],
		Namespace:                        ns.Name,
		PdbDefaultMinAvailablePercentage: common.CoalesceString(os.Getenv("PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE"), "50"),
		Password:                         os.Getenv("PASSWORD"),
		Provider:                         common.CoalesceString(os.Getenv("PROVIDER"), "k8s"),
		RackName:                         rn,
		Resolver:                         os.Getenv("RESOLVER"),
		RestClient:                       kc.RESTClient(),
		Router:                           os.Getenv("ROUTER"),
		Socket:                           common.CoalesceString(os.Getenv("SOCKET"), "/var/run/docker.sock"),
		Storage:                          common.CoalesceString(os.Getenv("STORAGE"), "/var/storage"),
		SubnetIDs:                        os.Getenv("SUBNET_IDS"),
		Version:                          common.CoalesceString(os.Getenv("VERSION"), "dev"),
		VpcID:                            os.Getenv("VPC_ID"),
		DockerUsername:                   os.Getenv("DOCKER_HUB_USERNAME"),
		DockerPassword:                   os.Getenv("DOCKER_HUB_PASSWORD"),
		EcrDockerHubCachePrefix:          os.Getenv("ECR_DOCKER_HUB_CACHE_PREFIX"),
		IsKedaEnabled:                    os.Getenv("KEDA_ENABLED") == "true",
		IsVpaEnabled:                     os.Getenv("VPA_ENABLED") == "true",
	}

	p.ReleasesToRetainAfterActive, _ = strconv.Atoi(os.Getenv("RELEASES_TO_RETAIN_AFTER_ACTIVE"))
	p.ReleasesToRetainTaskRunIntervalHour, _ = strconv.Atoi(os.Getenv("RELEASES_TO_RETAIN_TASK_RUN_INTERVAL_HOUR"))
	p.FeatureGates = options.GetFeatureGates()

	// webhook_signing_key is optional. If set and valid, outbound webhook
	// POSTs carry a Convox-Signature header. If set but invalid (typo,
	// short key, weak entropy, mixed case), validate at boot and degrade
	// to unsigned dispatch — crashing the api pod over a typo is hostile
	// to customers. The structured WARN gives the operator an actionable
	// pointer (regenerate with openssl rand -hex 32).
	if rawKey := os.Getenv("WEBHOOK_SIGNING_KEY"); rawKey != "" {
		if err := cxhmac.ValidateSigningKeys(rawKey); err != nil {
			fmt.Printf("ns=k8s at=webhook_signing_key result=invalid error=%q signing=disabled remediation=%q\n",
				err.Error(),
				"regenerate with: openssl rand -hex 32; see https://docs.convox.com/configuration/webhooks#signing")
		} else {
			p.WebhookSigningKey = rawKey
			n := 1
			for _, c := range rawKey {
				if c == ',' {
					n++
				}
			}
			fmt.Printf("ns=k8s at=webhook_signing_key result=configured keys=%d signing=enabled\n", n)
		}
	}

	p.RdsProvisioner = rds.NewProvisioner(p)
	p.ElasticacheProvisioner = elasticache.NewProvisioner(p)

	ms.SetProvider(p)

	return p, nil
}

func (p *Provider) Context() context.Context {
	return p.ctx
}

func (p *Provider) ContextTID() string {
	if p.ctx == nil {
		return ""
	}

	if tid, ok := p.ctx.Value(structs.ConvoxTIDCtxKey).(string); ok {
		return tid
	}
	return ""
}

// ContextActor returns the JWT-authenticated user (e.g. "system-read",
// "system-write", "system-admin") propagated through the provider's
// request-scoped ctx, or "unknown" when no actor identity is available.
// It is panic-safe and nil-safe by design: a nil receiver, nil ctx, missing
// claim, or empty-string claim all collapse to "unknown" so callers can
// emit an audit-event "actor" field without nil-checking. Whitespace is
// propagated verbatim — receivers see whatever the JWT minted.
func (p *Provider) ContextActor() string {
	if p == nil || p.ctx == nil {
		return "unknown"
	}
	if v, ok := p.ctx.Value(structs.ConvoxJwtUserCtxKey).(string); ok && v != "" {
		return v
	}
	return "unknown"
}

func (p *Provider) Initialize(opts structs.ProviderOptions) error {
	p.ctx = context.Background()
	p.logger = logger.New("ns=k8s")
	p.metrics = metrics.New("https://metrics.convox.com/metrics/rack")
	p.templater = templater.New(template.TemplatesFS)
	p.webhooks = []string{}

	if os.Getenv("TEST") == "true" {
		return nil
	}

	if err := atom.Initialize(); err != nil {
		return errors.WithStack(err)
	}

	go p.initializeTemplates()

	if !opts.IgnorePriorityClass {
		if err := p.initializePriorityClass(); err != nil {
			return errors.WithStack(err)
		}
	}

	signKey, err := p.SystemJwtSignKey()
	if err != nil {
		return err
	}

	p.JwtMngr = jwt.NewJwtManager(signKey)

	if err := p.createOrUpdateFlowSchema(p.Namespace, []string{
		"api", "atom", "resolver",
	}); err != nil {
		p.logger.Errorf("error creating flow schema: %v", err)
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

	nc, err := NewNodeController(p)
	if err != nil {
		return errors.WithStack(log.Error(err))
	}
	p.nc = nc

	dc, err := NewDeploymentController(p)
	if err != nil {
		return errors.WithStack(log.Error(err))
	}

	sc, err := NewSecretController(p)
	if err != nil {
		return errors.WithStack(err)
	}

	atomCtrl, err := NewAtomController(p)
	if err != nil {
		return errors.WithStack(err)
	}

	go ec.Run()
	go pc.Run()
	go wc.Run()
	go nc.Run()
	go dc.Run()
	go sc.Run()
	go atomCtrl.Run()

	go common.Tick(1*time.Hour, p.heartbeat)

	go p.startApiProxy()

	if os.Getenv("TEST") != "true" {
		go p.RunSharedInformer(make(chan struct{}))
	}

	if p.costTrackingEnabled() {
		// Lease lives in the api pod's own namespace (rack-system). Using
		// p.Namespace guarantees the namespace exists and avoids a broken
		// RACK_NAME env leading to a "-system" bogus namespace.
		leaseNs := p.Namespace
		if err := RunUsingLeaderElection(context.Background(), leaseNs, budgetLeaseName, p.Cluster, p.runBudgetAccumulator, func() {
			// Lifecycle observability (B.3): include the timestamp and
			// pod identity so an operator scanning api-pod logs across
			// a rack rotation can correlate a leadership loss with the
			// specific pod that gave up the lease and when. Identifier
			// matches the elector's resourcelock identity (os.Hostname).
			identity, _ := os.Hostname()
			fmt.Printf("ns=budget_accumulator at=lost_leadership at_time=%s identity=%q\n",
				time.Now().UTC().Format(time.RFC3339), identity)
		}); err != nil {
			_ = log.Errorf("budget accumulator elector failed to start: %v", err)
		}
	}

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

	ns, err := p.ListNodesFromInformer("")
	if err != nil {
		return errors.WithStack(err)
	}

	ks, err := p.GetNamespaceFromInformer("kube-system")
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

	rp := p.RackParams()
	if rp != nil {
		ms["rack_params"] = rp
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

	if _, err := p.Cluster.AppsV1().Deployments(p.Namespace).Patch(context.TODO(), "api", types.JSONPatchType, patch, am.PatchOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) initializeTemplates() {
	if err := p.applySystemTemplate("crd", nil); err != nil {
		panic(errors.WithStack(err))
	}

	if !p.CertManager {
		fmt.Println("Installing cert-manager skipped")
		return
	}

	d, _ := p.Cluster.AppsV1().Deployments(CERT_MANAGER_NAMESPACE).Get(context.TODO(), "cert-manager", am.GetOptions{})
	if d == nil {
		if err := p.applySystemTemplate("cert-manager", map[string]interface{}{
			"Role": p.CertManagerRoleArn,
		}); err != nil {
			panic(errors.WithStack(err))
		}
		return
	}

	if d.Spec.Template.Labels["app.kubernetes.io/version"] != CURRENT_CM_VERSION {
		fmt.Println("Updating cert-manager")
		p.deleteSystemTemplate("cert-manager", map[string]interface{}{
			"Role": p.CertManagerRoleArn,
		})

		currentRetry := 0
		for {
			_, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), CERT_MANAGER_NAMESPACE, am.GetOptions{})
			if ae.IsNotFound(err) {
				fmt.Println("Uninstalled old cert-manager")
				break
			}

			if currentRetry == MAX_RETRIES_UPDATE_CM {
				panic("Unable to install new cert-manager version, the old version was not uninstalled")
			}
			currentRetry++
			time.Sleep(time.Second * 10)
		}

		fmt.Println("Installing new cert-manager version")
		err := p.applySystemTemplate("cert-manager", map[string]interface{}{
			"Role": p.CertManagerRoleArn,
		})
		if err != nil {
			panic(errors.WithStack(fmt.Errorf("could not update cert-manager: %+v", err)))
		}
	}

	go p.installCertManagerConfig()
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
			d, err := p.Cluster.AppsV1().Deployments("cert-manager").Get(context.TODO(), "cert-manager-webhook", am.GetOptions{})
			if err != nil {
				fmt.Printf("could not get cert manager webhook deployment: %s\n", err)
				continue
			}

			config, err := p.LetsEncryptConfigGet()
			if err != nil {
				fmt.Printf("invalid config: %s\n", err)
			}

			for _, c := range d.Status.Conditions {
				if c.Type == "Available" && c.Status == "True" {
					fmt.Printf("installing cert manager letsencrypt config\n")

					if err := p.applySystemTemplate("cert-manager-letsencrypt", map[string]interface{}{
						"Config": config,
					}); err != nil {
						fmt.Printf("could not install cert manager letsencrypt config: %s\n", err)
						break
					}

					if cas, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), "ca", am.GetOptions{}); err == nil {
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
