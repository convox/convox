package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/logger"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	amv1 "k8s.io/client-go/applyconfigurations/meta/v1"
)

type DeploymentFilter struct {
	Ns      string
	App     string
	Service string
}

type NodeController struct {
	Provider *Provider

	stopCh chan struct{}
	nodeCh chan string

	nodeMap *sync.Map

	logger *logger.Logger
	start  time.Time
}

func NewNodeController(p *Provider) (*NodeController, error) {
	nc := &NodeController{
		Provider: p,
		stopCh:   make(chan struct{}),
		nodeCh:   make(chan string, 50),
		nodeMap:  &sync.Map{},
		logger:   logger.New("ns=node-controller"),
		start:    time.Now().UTC(),
	}

	return nc, nil
}

func (c *NodeController) Stop() error {
	c.stopCh <- struct{}{}
	close(c.nodeCh)
	return nil
}

func (c *NodeController) Add(node string) error {
	if _, ok := c.nodeMap.Load(node); !ok {
		c.nodeMap.Store(node, true)
		c.nodeCh <- node
	}
	return nil
}

func (c *NodeController) Run() {
	for i := 0; i < 3; i++ {
		go c.processor()
	}

	ticker := time.NewTicker(15 * time.Minute)

	for {
		select {
		case <-ticker.C:
			c.ProcessDrainingNode()
		case <-c.stopCh:
			return
		}
	}
}

func (c *NodeController) processor() {
	for node := range c.nodeCh {
		c.nodeMap.Delete(node)
		c.logger.Logf("processing node: %s", node)
		nObj, err := c.Provider.Cluster.CoreV1().Nodes().Get(c.Provider.ctx, node, am.GetOptions{})
		if err != nil {
			c.logger.Errorf("failed to get node: %s", err)
			return
		}
		if nObj.Spec.Unschedulable {
			c.findAndRescheduleDeploymentWithOneReplica(node)
		}
	}
}

func (c *NodeController) ProcessDrainingNode() {
	continueVal := ""
	for {
		c.logger.Logf("Fetching nodes to process drain nodes")
		nodeList, err := c.Provider.Cluster.CoreV1().Nodes().List(c.Provider.ctx, am.ListOptions{
			Continue: continueVal,
		})
		if err != nil {
			c.logger.Errorf("failed to list pods in a node: %s", err)
			return
		}

		for i := range nodeList.Items {
			nd := &nodeList.Items[i]
			if nd.Spec.Unschedulable &&
				nd.CreationTimestamp.Add(5*time.Minute).Before(time.Now()) {
				c.logger.Logf("Found unschedulable node: %s", nd.Name)
				c.Add(nd.Name)
			}
		}

		if nodeList.Continue == "" {
			return
		}
		continueVal = nodeList.Continue
	}
}

func (c *NodeController) findAndRescheduleDeploymentWithOneReplica(node string) {

	continueVal := ""

	for {
		podList, err := c.Provider.Cluster.CoreV1().Pods(ac.NamespaceAll).List(c.Provider.ctx, am.ListOptions{
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
			pdbList, err := c.Provider.Cluster.PolicyV1().PodDisruptionBudgets(v.Ns).List(c.Provider.ctx, am.ListOptions{
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
	_, err := c.Provider.Cluster.AppsV1().Deployments(ns).Apply(context.TODO(), sObj, am.ApplyOptions{
		FieldManager: "convox-system",
	})
	return err
}
