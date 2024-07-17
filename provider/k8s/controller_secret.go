package k8s

import (
	"fmt"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/kctl"
	"github.com/convox/convox/provider/aws/provisioner/elasticache"
	"github.com/convox/convox/provider/aws/provisioner/rds"
	"github.com/convox/logger"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	ic "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type SecretController struct {
	Controller *kctl.Controller
	Provider   *Provider

	log   *logger.Logger
	start time.Time
}

func NewSecretController(p *Provider) (*SecretController, error) {
	sc := &SecretController{
		Provider: p,
		log:      p.logger.At("ns=convox-k8s-secret"),
		start:    time.Now().UTC(),
	}

	c, err := kctl.NewController(p.Namespace, "convox-k8s-secret", sc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sc.Controller = c

	return sc, nil
}

func (c *SecretController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *SecretController) Informer() cache.SharedInformer {
	return ic.NewFilteredSecretInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *SecretController) ListOptions(opts *am.ListOptions) {
	opts.LabelSelector = "system=convox,type=state"
}

func (c *SecretController) Run() {
	ch := make(chan error)

	go c.Controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *SecretController) Start() error {
	c.start = time.Now().UTC()

	return nil
}

func (c *SecretController) Stop() error {
	return nil
}

func (c *SecretController) Add(obj interface{}) error {
	return c.Update(obj, obj)
}

func (c *SecretController) Delete(obj interface{}) error {
	return c.Update(obj, obj)
}

func (c *SecretController) Update(prev, cur interface{}) error {
	ss, err := assertSecret(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	if ss.Labels["system"] == "convox" && ss.Labels["type"] == "state" {
		if ss.DeletionTimestamp != nil && hasStateFinalizer(ss.Finalizers) {
			switch common.CoalesceString(ss.Labels["provisioner"], ss.Labels["provsioner"]) {
			case rds.ProvisionerName:
				err = c.Provider.uninstallRdsAssociatedWithStateSecret(ss)
				if err != nil {
					c.log.Errorf("failed to uninstall rds with associated secret: %s/%s, reason: %s", ss.Namespace, ss.Name, err)
				}
				return err
			case elasticache.ProvisionerName:
				err = c.Provider.uninstallElaticacheAssociatedWithStateSecret(ss)
				if err != nil {
					c.log.Errorf("failed to uninstall elasticache with associated secret: %s/%s, reason: %s", ss.Namespace, ss.Name, err)
				}
				return err

			}
		}
	}
	return nil
}

func assertSecret(v interface{}) (*ac.Secret, error) {
	s, ok := v.(*ac.Secret)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert pod for type: %T", v))
	}

	return s, nil
}
