package k8s

import (
	"context"
	"sync"
	"testing"

	"github.com/convox/convox/pkg/mock"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type gpuTestEngine struct {
	mock.TestEngine
	gpuInstances map[string]bool
}

func (e *gpuTestEngine) GPUIntanceList(instanceTypes []string) ([]string, error) {
	var result []string
	for _, it := range instanceTypes {
		if e.gpuInstances[it] {
			result = append(result, it)
		}
	}
	return result, nil
}

func newGpuTestController(c *fake.Clientset, engine *gpuTestEngine) *NodeController {
	return &NodeController{
		provider: &Provider{
			Cluster: c,
			Engine:  engine,
			ctx:     context.Background(),
		},
		nodeMap: &sync.Map{},
		logger:  logger.New("ns=node-controller-test"),
	}
}

func TestReconcileGpuLabels(t *testing.T) {
	c := fake.NewSimpleClientset()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "gpu-node-1",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "p3.2xlarge",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "cpu-node-1",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "m5.xlarge",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	_, err = c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "gpu-node-2",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "g4dn.xlarge",
				"convox.io/gpu-vendor":             "nvidia",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	// Bare node — no instance-type label at all
	_, err = c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name:   "bare-node",
			Labels: map[string]string{},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	engine := &gpuTestEngine{
		gpuInstances: map[string]bool{
			"p3.2xlarge":  true,
			"g4dn.xlarge": true,
		},
	}

	nc := newGpuTestController(c, engine)

	// Clear actions from node creation so we can inspect reconciler actions
	c.ClearActions()

	nc.ReconcileGpuLabels()

	// Verify the List call used the correct label selector (k8s may reorder requirements)
	actions := c.Actions()
	require.NotEmpty(t, actions)
	listAction, ok := actions[0].(k8stesting.ListAction)
	require.True(t, ok, "first action should be a List")
	selectorStr := listAction.GetListRestrictions().Labels.String()
	require.Contains(t, selectorStr, "node.kubernetes.io/instance-type")
	require.Contains(t, selectorStr, "!convox.io/gpu-vendor")

	// Unlabeled GPU node got the label
	node, err := c.CoreV1().Nodes().Get(context.TODO(), "gpu-node-1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "nvidia", node.Labels["convox.io/gpu-vendor"])

	// Non-GPU node was NOT labeled
	cpuNode, err := c.CoreV1().Nodes().Get(context.TODO(), "cpu-node-1", am.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, cpuNode.Labels["convox.io/gpu-vendor"])

	// Already-labeled GPU node kept its label
	gpuNode2, err := c.CoreV1().Nodes().Get(context.TODO(), "gpu-node-2", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "nvidia", gpuNode2.Labels["convox.io/gpu-vendor"])

	// Bare node (no instance-type) was NOT labeled
	bareNode, err := c.CoreV1().Nodes().Get(context.TODO(), "bare-node", am.GetOptions{})
	require.NoError(t, err)
	require.Empty(t, bareNode.Labels["convox.io/gpu-vendor"])
}

func TestReconcileGpuLabelsIdempotent(t *testing.T) {
	c := fake.NewSimpleClientset()

	_, err := c.CoreV1().Nodes().Create(context.TODO(), &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: "gpu-node-1",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "p3.2xlarge",
			},
		},
	}, am.CreateOptions{})
	require.NoError(t, err)

	engine := &gpuTestEngine{
		gpuInstances: map[string]bool{
			"p3.2xlarge": true,
		},
	}

	nc := newGpuTestController(c, engine)

	nc.ReconcileGpuLabels()
	nc.ReconcileGpuLabels()

	node, err := c.CoreV1().Nodes().Get(context.TODO(), "gpu-node-1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "nvidia", node.Labels["convox.io/gpu-vendor"])
}

func TestReconcileGpuLabelsEmptyCluster(t *testing.T) {
	c := fake.NewSimpleClientset()

	engine := &gpuTestEngine{
		gpuInstances: map[string]bool{},
	}

	nc := newGpuTestController(c, engine)

	// Should not panic or error on empty node list
	nc.ReconcileGpuLabels()
}

func TestTriggerDeploymentRescheduleSkipsMissingDeployment(t *testing.T) {
	c := newGpuTestController(fake.NewSimpleClientset(), &gpuTestEngine{})

	require.NoError(t, c.triggerDeploymentReschedule("database", "rack1-app1", "node1"))

	_, err := c.provider.Cluster.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "database", am.GetOptions{})
	require.True(t, kerr.IsNotFound(err))
}

func TestRescheduleSkipsPodsThatCannotBlockADrain(t *testing.T) {
	const node = "node1"
	const ns = "rack1-app1"
	const annotation = "convox.com/triggered-reschedule-for-node"

	pod := func(namespace, name string, labels map[string]string, phase ac.PodPhase, terminating bool) *ac.Pod {
		p := &ac.Pod{
			ObjectMeta: am.ObjectMeta{Namespace: namespace, Name: name, Labels: labels},
			Spec:       ac.PodSpec{NodeName: node},
			Status:     ac.PodStatus{Phase: phase},
		}
		if terminating {
			now := am.Now()
			p.DeletionTimestamp = &now
		}
		return p
	}
	web := func(name string, phase ac.PodPhase, terminating bool) *ac.Pod {
		return pod(ns, name, map[string]string{"app": "app1", "service": "web", "system": "convox"}, phase, terminating)
	}
	autoscaler := func(terminating bool) *ac.Pod {
		return pod("kube-system", "cluster-autoscaler-1", map[string]string{"app": "cluster-autoscaler"}, ac.PodRunning, terminating)
	}

	tests := []struct {
		name            string
		pods            []*ac.Pod
		allowed         int32
		webRolls        bool
		autoscalerRolls bool
	}{
		{name: "live pod, budget closed", pods: []*ac.Pod{web("web-1", ac.PodRunning, false)}, webRolls: true},
		{name: "live pod, budget open", pods: []*ac.Pod{web("web-1", ac.PodRunning, false)}, allowed: 1},
		{name: "terminating pod", pods: []*ac.Pod{web("web-1", ac.PodRunning, true)}},
		{name: "terminating pod and finished timer pods", pods: []*ac.Pod{
			web("web-1", ac.PodRunning, true),
			web("timer-1", ac.PodSucceeded, false),
			web("timer-2", ac.PodFailed, false),
		}},
		{name: "terminating and live pod", pods: []*ac.Pod{
			web("web-1", ac.PodRunning, true),
			web("web-2", ac.PodRunning, false),
		}, webRolls: true},
		{name: "live autoscaler pod", pods: []*ac.Pod{autoscaler(false)}, allowed: 1, autoscalerRolls: true},
		{name: "terminating autoscaler pod", pods: []*ac.Pod{autoscaler(true)}, allowed: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(
				&appsv1.Deployment{ObjectMeta: am.ObjectMeta{Namespace: ns, Name: "web"}},
				&appsv1.Deployment{ObjectMeta: am.ObjectMeta{Namespace: "kube-system", Name: "cluster-autoscaler"}},
				&policyv1.PodDisruptionBudget{
					ObjectMeta: am.ObjectMeta{Namespace: ns, Name: "web", Labels: map[string]string{"app": "app1", "service": "web", "system": "convox"}},
					Status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: tt.allowed},
				},
			)
			for _, p := range tt.pods {
				_, err := client.CoreV1().Pods(p.Namespace).Create(context.TODO(), p, am.CreateOptions{})
				require.NoError(t, err)
			}

			newGpuTestController(client, &gpuTestEngine{}).findAndRescheduleDeploymentWithOneReplica(node)

			rolled := func(namespace, name string) bool {
				d, err := client.AppsV1().Deployments(namespace).Get(context.TODO(), name, am.GetOptions{})
				require.NoError(t, err)
				return d.Spec.Template.Annotations[annotation] == node
			}
			require.Equal(t, tt.webRolls, rolled(ns, "web"))
			require.Equal(t, tt.autoscalerRolls, rolled("kube-system", "cluster-autoscaler"))
		})
	}
}
