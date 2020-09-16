package k8s_test

import (
	"testing"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCapacityGet(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, nodeCreateResources(kk, "node1", "2000m", "3000M"))
		require.NoError(t, nodeCreateResources(kk, "node2", "2000m", "3000M"))
		require.NoError(t, nodeCreateResources(kk, "node3", "2000m", "3000M"))
		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, processCreateResources(kk, "rack1-app1", "process1", "system=convox,rack=rack1,app=app1,service=service1,type=service", "128m", "256M"))
		require.NoError(t, processCreateResources(kk, "rack1-app1", "process2", "system=convox,rack=rack1,app=app1,service=service2,type=process", "228m", "592M"))
		require.NoError(t, processCreateResources(kk, "rack1-app1", "process3", "system=convox,rack=rack1,app=app1,service=service2,type=process", "593m", "4388M"))

		c, err := p.CapacityGet()
		require.NoError(t, err)
		require.Equal(t, int64(6000), c.ClusterCPU)
		require.Equal(t, int64(9000), c.ClusterMemory)
		require.Equal(t, int64(949), c.ProcessCPU)
		require.Equal(t, int64(5236), c.ProcessMemory)
		require.Equal(t, int64(3), c.ProcessCount)
	})
}
