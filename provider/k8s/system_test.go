package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSystemProcesses(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, processCreate(kk, "ns1", "process1", "system=convox,rack=rack1,app=system,service=service1"))
		require.NoError(t, processCreate(kk, "ns1", "process2", "system=convox,rack=rack1,app=system,service=service2"))
		require.NoError(t, processCreate(kk, "ns1", "process3", "system=convox,rack=rack2,app=system,service=service3"))
		require.NoError(t, processCreate(kk, "ns2", "process4", "system=convox,rack=rack1,app=system,service=service4"))
		require.NoError(t, processCreate(kk, "ns1", "process5", "system=convox,rack=rack2,app=system"))

		pss, err := p.SystemProcesses(structs.SystemProcessesOptions{})
		require.NoError(t, err)
		require.Len(t, pss, 2)

		require.Equal(t, "system", pss[0].App)
		require.Equal(t, "service1", pss[0].Name)
		require.Equal(t, "process1", pss[0].Id)

		require.Equal(t, "system", pss[1].App)
		require.Equal(t, "service2", pss[1].Name)
		require.Equal(t, "process2", pss[1].Id)
	})
}

func TestSystemProcessesAll(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, processCreate(kk, "ns1", "process1", "system=convox,rack=rack1,app=system,service=service1"))
		require.NoError(t, processCreate(kk, "ns1", "process2", "system=convox,rack=rack1,app=system,service=service2"))
		require.NoError(t, processCreate(kk, "ns1", "process3", "system=convox,rack=rack2,app=system,service=service3"))
		require.NoError(t, processCreate(kk, "ns2", "process4", "system=convox,rack=rack1,app=app1,service=service4"))
		require.NoError(t, processCreate(kk, "ns1", "process5", "system=convox,rack=rack2,app=system"))

		pss, err := p.SystemProcesses(structs.SystemProcessesOptions{All: options.Bool(true)})
		require.NoError(t, err)
		require.Len(t, pss, 3)

		require.Equal(t, "app1", pss[0].App)
		require.Equal(t, "service4", pss[0].Name)
		require.Equal(t, "process4", pss[0].Id)

		require.Equal(t, "system", pss[1].App)
		require.Equal(t, "service1", pss[1].Name)
		require.Equal(t, "process1", pss[1].Id)

		require.Equal(t, "system", pss[2].App)
		require.Equal(t, "service2", pss[2].Name)
		require.Equal(t, "process2", pss[2].Id)
	})
}
