package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/convox/convox/pkg/kctl"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/logger"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	amv1 "k8s.io/client-go/applyconfigurations/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type DeploymentFilter struct {
	Ns      string
	App     string
	Service string
}

type NodeController struct {
	provider   *Provider
	controller *kctl.Controller

	stopCh chan struct{}
	nodeCh chan string

	nodeMap *sync.Map

	logger *logger.Logger
	start  time.Time
}

func NewNodeController(p *Provider) (*NodeController, error) {
	nc := &NodeController{
		provider: p,
		stopCh:   make(chan struct{}),
		nodeCh:   make(chan string, 50),
		nodeMap:  &sync.Map{},
		logger:   logger.New("ns=node-controller"),
		start:    time.Now().UTC(),
	}

	c, err := kctl.NewController(p.Namespace, "convox-node-controller", nc)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	nc.controller = c

	return nc, nil
}

func (c *NodeController) Client() kubernetes.Interface {
	return c.provider.Cluster
}

func (c *NodeController) Informer() cache.SharedInformer {
	return informers.NewSharedInformerFactory(c.provider.Cluster, 3*time.Minute).Core().V1().Nodes().Informer()
}

func (c *NodeController) Run() {
	ch := make(chan error)

	go c.controller.Run(ch)

	for err := range ch {
		fmt.Printf("err = %+v\n", err)
	}
}

func (c *NodeController) Start() error {
	c.start = time.Now().UTC()

	return nil
}

func (c *NodeController) Stop() error {
	return nil
}

func (c *NodeController) Add(obj interface{}) error {
	nd, err := assertNode(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	go c.AddGpuLabel(nd)

	fmt.Printf("node add: %s/%s\n", nd.ObjectMeta.Namespace, nd.ObjectMeta.Name)

	if nd.Spec.Unschedulable &&
		nd.CreationTimestamp.Add(5*time.Minute).Before(time.Now()) {
		c.logger.Logf("Found unschedulable node: %s", nd.Name)
		c.findAndRescheduleDeploymentWithOneReplica(nd.Name)
	}

	return nil
}

func (c *NodeController) Delete(obj interface{}) error {
	nd, err := assertNode(obj)
	if err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("node delete: %s/%s\n", nd.ObjectMeta.Namespace, nd.ObjectMeta.Name)
	return nil
}

func (c *NodeController) Update(prev, cur interface{}) error {
	nd, err := assertNode(cur)
	if err != nil {
		return errors.WithStack(err)
	}

	go c.AddGpuLabel(nd)

	fmt.Printf("node update: %s/%s\n", nd.ObjectMeta.Namespace, nd.ObjectMeta.Name)
	if nd.Spec.Unschedulable &&
		nd.CreationTimestamp.Add(5*time.Minute).Before(time.Now()) {
		c.logger.Logf("Found unschedulable node: %s", nd.Name)
		c.findAndRescheduleDeploymentWithOneReplica(nd.Name)
	}
	return nil
}

func (c *NodeController) AddGpuLabel(nd *ac.Node) {
	intanceType := nd.Labels["node.kubernetes.io/instance-type"]
	if intanceType == "" {
		return
	}

	labelKey := "convox.io/gpu-vendor"
	labelValue := "nvidia"

	if nd.Labels[labelKey] != "" {
		return
	}

	nodeKey := fmt.Sprintf("%s-gpu-label", nd.Name)

	// check if already processed this node
	if _, ok := c.nodeMap.Load(nodeKey); ok {
		c.logger.Errorf("node label's are already processed: %s", nd.Name)
		return
	}

	gpus, err := c.provider.Engine.GPUIntanceList([]string{intanceType})
	if err != nil {
		c.logger.Errorf("failed to check gpu instance '%s': %s", intanceType, err)
		return
	}

	if len(gpus) > 0 {
		c.logger.Logf("found gpu instance: %s and applying label", intanceType)

		if err := c.PatchNodeLabel(nd, labelKey, labelValue); err != nil {
			c.logger.Errorf("failed to patch node: %s, err: %s", nd.Name, err)
			return
		}
	}

	c.nodeMap.Store(nodeKey, true)
}

func (c *NodeController) findAndRescheduleDeploymentWithOneReplica(node string) {

	continueVal := ""

	for {
		podList, err := c.provider.Cluster.CoreV1().Pods(ac.NamespaceAll).List(c.provider.ctx, am.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", node),
			Continue:      continueVal,
		})
		if err != nil {
			c.logger.Errorf("failed to list pods in a node: %s", err)
			return
		}

		labelsMap := map[string]DeploymentFilter{}
		labelsToCheck := []string{
			"app", "service", "system",
		}
		for i := range podList.Items {
			pod := &podList.Items[i]

			if pod.Labels["app"] == "cluster-autoscaler" {
				c.triggerDeploymentReschedule("cluster-autoscaler", pod.Namespace, node)
			}

			key := pod.Name
			for _, lb := range labelsToCheck {
				v, has := pod.Labels[lb]
				if !has {
					key = ""
					break
				}

				key = fmt.Sprintf("%s#$#%s", key, v)
			}

			if key != "" {
				labelsMap[key] = DeploymentFilter{
					Ns:      pod.Namespace,
					App:     pod.Labels["app"],
					Service: pod.Labels["service"],
				}
			}
		}

		for _, v := range labelsMap {
			pdbList, err := c.provider.Cluster.PolicyV1().PodDisruptionBudgets(v.Ns).List(c.provider.ctx, am.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s,service=%s,system=convox", v.App, v.Service),
			})
			if err != nil {
				c.logger.Errorf("failed to list deployment: %s", err)
				continue
			}
			if len(pdbList.Items) > 0 {
				d := pdbList.Items[0]
				if d.Status.DisruptionsAllowed == 0 {
					c.logger.Logf("Found a deployment blocking draing node %s/%s", d.Namespace, d.Name)
					// pdb will always have the same name as deployment
					if err := c.triggerDeploymentReschedule(d.Name, d.Namespace, node); err != nil {
						c.logger.Errorf("failed to trigger deployment reschedule: %s", err)
					}
				}
			}
		}

		if podList.Continue == "" {
			return
		}
		continueVal = podList.Continue
	}
}

func (c *NodeController) triggerDeploymentReschedule(name, ns, node string) error {
	c.logger.Logf("Trigger reschedule for deployment %s/%s", ns, name)
	sObj := &appsv1.DeploymentApplyConfiguration{
		TypeMetaApplyConfiguration: amv1.TypeMetaApplyConfiguration{
			Kind:       options.String("Deployment"),
			APIVersion: options.String("apps/v1"),
		},
		ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
			Name:      &name,
			Namespace: &ns,
		},
		Spec: &appsv1.DeploymentSpecApplyConfiguration{
			Template: &corev1.PodTemplateSpecApplyConfiguration{
				ObjectMetaApplyConfiguration: &amv1.ObjectMetaApplyConfiguration{
					Annotations: map[string]string{
						"convox.com/triggered-reschedule-for-node": node,
					},
				},
			},
		},
	}
	_, err := c.provider.Cluster.AppsV1().Deployments(ns).Apply(context.TODO(), sObj, am.ApplyOptions{
		FieldManager: "convox-system",
	})
	return err
}

func (c *NodeController) PatchNodeLabel(nd *ac.Node, key, value string) error {
	// Patch labels using strategic merge patch
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				key: value,
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("Error marshaling patch: %v", err)
	}

	_, err = c.provider.Cluster.CoreV1().Nodes().Patch(c.provider.ctx, nd.Name, types.StrategicMergePatchType, patchBytes, v1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("Error patching node: %v", err)
	}
	return nil
}

func assertNode(v interface{}) (*ac.Node, error) {
	nd, ok := v.(*ac.Node)
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("could not assert node for type: %T", v))
	}

	return nd, nil
}
