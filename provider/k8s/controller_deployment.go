package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/convox/convox/pkg/kctl"
	"github.com/convox/logger"
	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	ic "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

var labelsForDeployment = []string{
	"app", "service", "system",
}

const (
	AnnotationPdbDisabled     = "convox.com/pdb-disbaled"
	AnnotationPdbMinAvailable = "convox.com/pdb-minavailable"
)

type DeployController struct {
	Controller *kctl.Controller
	Provider   *Provider

	logger *logger.Logger
	start  time.Time
}

func NewDeploymentController(p *Provider) (*DeployController, error) {
	dc := &DeployController{
		Provider: p,
		logger:   logger.New("ns=deployment-controller"),
		start:    time.Now().UTC(),
	}

	c, err := kctl.NewController(p.Namespace, "convox-k8s-deployment", dc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	dc.Controller = c

	return dc, nil
}

func (c *DeployController) Client() kubernetes.Interface {
	return c.Provider.Cluster
}

func (c *DeployController) Informer() cache.SharedInformer {
	return ic.NewFilteredDeploymentInformer(c.Provider.Cluster, ac.NamespaceAll, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, c.ListOptions)
}

func (c *DeployController) ListOptions(opts *am.ListOptions) {
	opts.LabelSelector = fmt.Sprintf("system=convox,rack=%s", c.Provider.Name)
}

func (c *DeployController) Run() {
	ch := make(chan error)

	go c.Controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *DeployController) Start() error {
	c.start = time.Now().UTC()

	return nil
}

func (c *DeployController) Stop() error {
	return nil
}

func (c *DeployController) Add(obj interface{}) error {
	d, err := assertDeployment(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	c.logger.Logf("deployment add: %s/%s\n", d.Namespace, d.Name)

	if c.isConvoxManaged(d) {
		err = c.SyncPDB(d, false)
		if err != nil {
			c.logger.Errorf("failed to sync pdb for deployment %s/%s: %s", d.Namespace, d.Name, err)
		}
	}

	return nil
}

func (c *DeployController) Delete(obj interface{}) error {
	d, err := assertDeployment(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	c.logger.Logf("deployment delete: %s/%s\n", d.Namespace, d.Name)

	if c.isConvoxManaged(d) {
		err = c.SyncPDB(d, true)
		if err != nil {
			c.logger.Errorf("failed to sync pdb for deployment %s/%s: %s", d.Namespace, d.Name, err)
		}
	}

	return nil
}

func (c *DeployController) Update(prev, cur interface{}) error {
	d, err := assertDeployment(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	c.logger.Logf("deployment update: %s/%s\n", d.Namespace, d.Name)

	if c.isConvoxManaged(d) {
		err = c.SyncPDB(d, false)
		if err != nil {
			c.logger.Errorf("failed to sync pdb for deployment %s/%s: %s", d.Namespace, d.Name, err)
		}
	}

	return nil
}

func (c *DeployController) SyncPDB(d *apps.Deployment, remove bool) error {
	if d.Spec.Selector == nil {
		return fmt.Errorf("invalid deployment selector: %s/%s", d.Namespace, d.Name)
	}

	if d.Annotations[AnnotationPdbDisabled] == "true" || d.Spec.Template.Annotations[AnnotationPdbDisabled] == "true" {
		remove = true
	}

	pdb_default_min_available_percentage := &intstr.IntOrString{
		Type:   intstr.String,
		StrVal: os.Getenv("PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE")+"%",
	}

	minAvailableAnnoVal := d.Annotations[AnnotationPdbMinAvailable]
	if minAvailableAnnoVal == "" {
		minAvailableAnnoVal = d.Spec.Template.Annotations[AnnotationPdbMinAvailable]
	}
	if minAvailableAnnoVal != "" {
		if strings.HasSuffix(minAvailableAnnoVal, "%") {
			pdb_default_min_available_percentage = &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: minAvailableAnnoVal,
			}
		} else if val, err := strconv.Atoi(minAvailableAnnoVal); err != nil {
			pdb_default_min_available_percentage = &intstr.IntOrString{
				Type:   intstr.Int,
				IntVal: int32(val),
			}
		}
	}

	if remove {
		c.Provider.Cluster.PolicyV1().PodDisruptionBudgets(d.Namespace).Delete(c.Provider.ctx, d.Name, am.DeleteOptions{})
		return nil
	} else {
		_, err := c.Provider.CreateOrPatchPDB(c.Provider.ctx, metav1.ObjectMeta{
			Name:      d.Name,
			Namespace: d.Namespace,
		}, func(pdb *policyv1.PodDisruptionBudget) *policyv1.PodDisruptionBudget {
			pdb.Labels = d.Labels
			pdb.Spec.MinAvailable = pdb_default_min_available_percentage 
			pdb.Spec.Selector = d.Spec.Selector
			pdb.Spec.Selector.MatchLabels["type"] = "service"
			return pdb
		}, metav1.PatchOptions{
			FieldManager: "convox",
		})
		return err
	}
}

func (c *DeployController) isConvoxManaged(d *apps.Deployment) bool {
	for _, v := range labelsForDeployment {
		if d.Labels[v] == "" {
			return false
		}
	}

	return strings.Contains(d.Labels["system"], "convox")
}

func assertDeployment(v interface{}) (*apps.Deployment, error) {
	d, ok := v.(*apps.Deployment)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert deployment for type: %T", v))
	}

	return d, nil
}

func (p *Provider) CreateOrPatchPDB(ctx context.Context, meta metav1.ObjectMeta, transform func(*policyv1.PodDisruptionBudget) *policyv1.PodDisruptionBudget, opts metav1.PatchOptions) (*policyv1.PodDisruptionBudget, error) {
	cur, err := p.Cluster.PolicyV1().PodDisruptionBudgets(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		p.logger.Logf("Creating PDB %s/%s.", meta.Namespace, meta.Name)
		out, err := p.Cluster.PolicyV1().PodDisruptionBudgets(meta.Namespace).Create(ctx, transform(&policyv1.PodDisruptionBudget{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PodDisruptionBudget",
				APIVersion: policyv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}), metav1.CreateOptions{
			DryRun:       opts.DryRun,
			FieldManager: opts.FieldManager,
		})
		return out, err
	} else if err != nil {
		return nil, err
	}
	return p.PatchPDB(ctx, cur, transform, opts)
}

func (p *Provider) PatchPDB(ctx context.Context, cur *policyv1.PodDisruptionBudget, transform func(*policyv1.PodDisruptionBudget) *policyv1.PodDisruptionBudget, opts metav1.PatchOptions) (*policyv1.PodDisruptionBudget, error) {
	return p.PatchPDBObject(ctx, cur, transform(cur.DeepCopy()), opts)
}

func (p *Provider) PatchPDBObject(ctx context.Context, cur, mod *policyv1.PodDisruptionBudget, opts metav1.PatchOptions) (*policyv1.PodDisruptionBudget, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, policyv1.PodDisruptionBudget{})
	if err != nil {
		return nil, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, nil
	}
	p.logger.Logf("Patching PDB %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	return p.Cluster.PolicyV1().PodDisruptionBudgets(cur.Namespace).Patch(ctx, cur.Name, types.StrategicMergePatchType, patch, opts)
}
