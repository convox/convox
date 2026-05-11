package k8s

import (
	"context"
	"sync"
	"testing"

	"github.com/convox/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type gpuLabelEngine struct {
	MockEngine
}

func (*gpuLabelEngine) GPUIntanceList(instanceTypes []string) ([]string, error) {
	return instanceTypes, nil
}

func TestNodeControllerReconcileGpuLabels(t *testing.T) {
	cluster := fake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gpu-node",
			Labels: map[string]string{
				"node.kubernetes.io/instance-type": "Standard_nc24ads_a100_v4",
			},
		},
	})

	nc := &NodeController{
		provider: &Provider{
			Cluster: cluster,
			Engine:  &gpuLabelEngine{},
			ctx:     context.Background(),
		},
		nodeMap: &sync.Map{},
		logger:  logger.New("ns=node-controller-test"),
	}

	nc.ReconcileGpuLabels()

	node, err := cluster.CoreV1().Nodes().Get(context.Background(), "gpu-node", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "nvidia", node.Labels["convox.io/gpu-vendor"])
}
