package k8s

import (
	"context"
	"sync"
	"testing"

	"github.com/convox/convox/pkg/mock"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
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
