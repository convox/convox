package k8s

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/common"
	cxhmac "github.com/convox/convox/pkg/hmac"
	"github.com/convox/convox/pkg/jwt"
	"github.com/convox/convox/pkg/manifest"
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
	corev1 "k8s.io/api/core/v1"
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
	IsKarpenterEnabled                  bool
	IsKedaEnabled                       bool
	IsVpaEnabled                        bool
	Name                                string
	ReleasesToRetainAfterActive         int
	ReleasesToRetainTaskRunIntervalHour int
	MetricScraper                       *MetricScraperClient
	MetricsClient                       metricsclientset.Interface
	PromClient                          *PrometheusClient
	Namespace                           string
	Password                            string
	PdbDefaultMinAvailablePercentage    string
	Provider                            string
	RackName                            string
	Resolver                            string
	BuildDisableResolver                bool
	RestClient                          rest.Interface
	Router                              string
	RouterType                          string
	ProxyProtocol                       bool
	ContourInternalTLS                  bool
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

	// Test-only hook: bypasses AppGet→ReleaseGet→manifest.Load in ServiceTriggersEnable GPU preflight.
	TriggersOverrideManifestServiceHook func(app, service string) (*manifest.Service, error)

	ctx       context.Context
	logger    *logger.Logger
	metrics   *metrics.Metrics
	templater *templater.Templater

	// Pointer so WithContext's `pp := *p` shares the lock+slice across request copies.
	webhookState *webhookState
}

type webhookState struct {
	mu        sync.RWMutex
	populated bool // true once informer has observed the webhooks configmap
	urls      []string
	receivers []webhookEntry // parsed lazily from urls; per-URL timeout via JSON encoding
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

	pc, err := NewPrometheusClient(os.Getenv("PROMETHEUS_URL"))
	if err != nil {
		fmt.Printf("ns=k8s at=prometheus_client result=invalid url=%q error=%q metrics=disabled\n",
			redactURLForLog(os.Getenv("PROMETHEUS_URL")), err.Error())
	}

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
		PromClient:                       pc,
		Name:                             ns.Labels["rack"],
		Namespace:                        ns.Name,
		PdbDefaultMinAvailablePercentage: common.CoalesceString(os.Getenv("PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE"), "50"),
		Password:                         os.Getenv("PASSWORD"),
		Provider:                         common.CoalesceString(os.Getenv("PROVIDER"), "k8s"),
		RackName:                         rn,
		Resolver:                         os.Getenv("RESOLVER"),
		RestClient:                       kc.RESTClient(),
		Router:                           os.Getenv("ROUTER"),
		RouterType:                       common.CoalesceString(os.Getenv("ROUTER_TYPE"), "nginx"),
		ProxyProtocol:                    os.Getenv("PROXY_PROTOCOL") == "true",
		ContourInternalTLS:               os.Getenv("CONTOUR_INTERNAL_TLS") == "true",
		Socket:                           common.CoalesceString(os.Getenv("SOCKET"), "/var/run/docker.sock"),
		Storage:                          common.CoalesceString(os.Getenv("STORAGE"), "/var/storage"),
		SubnetIDs:                        os.Getenv("SUBNET_IDS"),
		Version:                          common.CoalesceString(os.Getenv("VERSION"), "dev"),
		VpcID:                            os.Getenv("VPC_ID"),
		DockerUsername:                   os.Getenv("DOCKER_HUB_USERNAME"),
		DockerPassword:                   os.Getenv("DOCKER_HUB_PASSWORD"),
		EcrDockerHubCachePrefix:          os.Getenv("ECR_DOCKER_HUB_CACHE_PREFIX"),
		IsKarpenterEnabled:               os.Getenv("KARPENTER_ENABLED") == "true",
		IsKedaEnabled:                    os.Getenv("KEDA_ENABLED") == "true",
		IsVpaEnabled:                     os.Getenv("VPA_ENABLED") == "true",
	}

	p.ReleasesToRetainAfterActive, _ = strconv.Atoi(os.Getenv("RELEASES_TO_RETAIN_AFTER_ACTIVE"))
	p.ReleasesToRetainTaskRunIntervalHour, _ = strconv.Atoi(os.Getenv("RELEASES_TO_RETAIN_TASK_RUN_INTERVAL_HOUR"))
	p.FeatureGates = options.GetFeatureGates()

	// Invalid signing key degrades to unsigned dispatch rather than crashing.
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

// ContextActor returns the JWT user from ctx, or "unknown". Nil-safe.
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
	if p.webhookState == nil {
		p.webhookState = &webhookState{}
	}

	// Apply env-configurable GC interval (clamps to 60s-1h, default 5m).
	applyReleaseWatcherGCIntervalEnv()

	// Re-validate SSRF allowlist in case env bypassed param-set.
	if raw := os.Getenv("PROMETHEUS_URL"); raw != "" {
		if err := ValidatePrometheusURL(raw); err != nil {
			fmt.Printf("ns=k8s at=prometheus_url result=invalid_ssrf url=%q error=%q metrics=disabled remediation=%q\n",
				redactURLForLog(raw), err.Error(),
				"convox rack params set prometheus_url=<valid http:// or https:// URL>; see docs/configuration/rack-parameters/aws/prometheus_url.md")
			p.PromClient = nil
		}
	}

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

// redactURLForLog strips credentials and query params from a URL for safe logging.
func redactURLForLog(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "[unparseable url; redacted]"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if parsed.User != nil {
		return parsed.Redacted()
	}
	return parsed.String()
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

	go p.runReleasePromoteWatchGC(p.ctx)

	go p.runOrphanedPodReaper(p.ctx)

	go common.Tick(1*time.Hour, p.heartbeat)

	go p.startApiProxy()

	if os.Getenv("TEST") != "true" {
		go p.RunSharedInformer(make(chan struct{}))
	}

	if p.costTrackingEnabled() {
		leaseNs := p.Namespace
		if err := RunUsingLeaderElection(context.Background(), leaseNs, budgetLeaseName, p.Cluster, p.runBudgetAccumulator, func() {
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
			"Role":             p.CertManagerRoleArn,
			"KarpenterEnabled": p.IsKarpenterEnabled,
		}); err != nil {
			panic(errors.WithStack(err))
		}
		return
	}

	if d.Spec.Template.Labels["app.kubernetes.io/version"] != CURRENT_CM_VERSION {
		fmt.Println("Updating cert-manager")
		p.deleteSystemTemplate("cert-manager", map[string]interface{}{
			"Role":             p.CertManagerRoleArn,
			"KarpenterEnabled": p.IsKarpenterEnabled,
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
			"Role":             p.CertManagerRoleArn,
			"KarpenterEnabled": p.IsKarpenterEnabled,
		})
		if err != nil {
			panic(errors.WithStack(fmt.Errorf("could not update cert-manager: %+v", err)))
		}
	} else {
		_, hasSystemNode := d.Spec.Template.Spec.NodeSelector["convox.io/system-node"]
		if p.IsKarpenterEnabled != hasSystemNode {
			fmt.Println("Reconciling cert-manager node scheduling")
			if err := p.applySystemTemplate("cert-manager", map[string]interface{}{
				"Role":             p.CertManagerRoleArn,
				"KarpenterEnabled": p.IsKarpenterEnabled,
			}); err != nil {
				fmt.Printf("cert-manager scheduling reconcile failed: %v\n", err)
			}
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
						"Config":       config,
						"IngressClass": p.Engine.IngressClass(),
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

// EnsureSelfSignedCA creates the rack self-signed CA secret if absent so cert-manager can mint the self-signed ClusterIssuer from it.
func (p *Provider) EnsureSelfSignedCA() error {
	if _, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), "ca", am.GetOptions{}); err == nil {
		return nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	serial, err := crand.Int(crand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Convox Rack CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(crand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal CA key: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: am.ObjectMeta{
			Name:      "ca",
			Namespace: p.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		},
		Type: corev1.SecretTypeTLS,
	}

	if _, err := p.Cluster.CoreV1().Secrets(p.Namespace).Create(context.TODO(), secret, am.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create CA secret: %w", err)
	}

	fmt.Printf("generated self-signed CA in %s/ca\n", p.Namespace)

	return nil
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
