package k8s_test

import (
	"strings"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"

	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func processCreate(c kubernetes.Interface, ns, name, labels string) error {
	return processCreatePorts(c, ns, name, labels, 123, 4567, "127.0.0.1")
}

func processCreatePorts(c kubernetes.Interface, ns, name, labels string, hostPort int32, containerPort int32, podIp string) error {
	om := am.ObjectMeta{
		Labels: map[string]string{},
		Name:   name,
	}

	for _, part := range strings.Split(labels, ",") {
		kv := strings.SplitN(part, "=", 2)
		om.Labels[kv[0]] = kv[1]
	}

	p := &ac.Pod{
		ObjectMeta: om,
		Status: ac.PodStatus{
			PodIP: podIp,
		},
		Spec: ac.PodSpec{
			Containers: []ac.Container{
				{
					Name: "main",
					Ports: []ac.ContainerPort{
						{
							ContainerPort: containerPort,
							HostPort:      hostPort,
						},
					},
				},
			},
		},
	}

	if _, err := c.CoreV1().Pods(ns).Create(p); err != nil {
		return err
	}

	return nil
}

func TestProcessList(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process1", "system=convox,rack=rack1,app=app1,service=service1,type=service", 111, 2222, "1.2.3.4"))
		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process2", "system=convox,rack=rack1,app=app1,service=service2,type=process", 333, 4444, "5.6.7.8"))
		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process3", "system=convox,rack=rack1,app=app1,service=service2,type=process", 0, 5555, "9.10.11.12"))

		pss, err := p.ProcessList("app1", structs.ProcessListOptions{})
		require.NoError(t, err)
		require.Len(t, pss, 3)

		require.Equal(t, "process1", pss[0].Id)
		require.Len(t, pss[0].Ports, 1)
		require.Equal(t, "111:2222", pss[0].Ports[0])
		require.Equal(t, "service1", pss[0].Name)
		require.Equal(t, "1.2.3.4", pss[0].Host)

		require.Equal(t, "process2", pss[1].Id)
		require.Len(t, pss[1].Ports, 1)
		require.Equal(t, "333:4444", pss[1].Ports[0])
		require.Equal(t, "service2", pss[1].Name)
		require.Equal(t, "5.6.7.8", pss[1].Host)

		require.Equal(t, "process3", pss[2].Id)
		require.Len(t, pss[2].Ports, 1)
		require.Equal(t, "5555", pss[2].Ports[0])
		require.Equal(t, "service2", pss[2].Name)
		require.Equal(t, "9.10.11.12", pss[2].Host)

	})
}
