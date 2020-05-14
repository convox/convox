package k8s_test

import (
	"strings"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	kt "k8s.io/client-go/testing"
	mvfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestProcessList(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		mc := p.Metrics.(*mvfake.Clientset)

		mc.AddReactor("get", "pods", func(action kt.Action) (handled bool, ret runtime.Object, err error) {
			switch action.(kt.GetAction).GetName() {
			case "process1":
				return true, podMetric("ns1", "process1", 64, 520), nil
			default:
				return false, nil, nil
			}
		})

		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process1", "system=convox,rack=rack1,app=app1,service=service1,type=service", "1.2.3.4", 111, 2222))
		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process2", "system=convox,rack=rack1,app=app1,service=service2,type=process", "5.6.7.8", 333, 4444))
		require.NoError(t, processCreatePorts(kk, "rack1-app1", "process3", "system=convox,rack=rack1,app=app1,service=service2,type=process", "9.10.11.12", 0, 5555))
		require.NoError(t, processCreate(kk, "racp1-app2", "process4", "system=convox,rack=rack1,app=app1,service=service1,type=service"))
		require.NoError(t, processCreate(kk, "rack1-app1", "process5", "system=convox,rack=rack2,app=app1"))

		pss, err := p.ProcessList("app1", structs.ProcessListOptions{})
		require.NoError(t, err)
		require.Len(t, pss, 3)

		require.Equal(t, "app1", pss[0].App)
		require.Equal(t, 0.25, pss[0].Cpu)
		require.Equal(t, "1.2.3.4", pss[0].Host)
		require.Equal(t, "process1", pss[0].Id)
		require.Equal(t, 520.0, pss[0].Memory)
		require.Equal(t, "service1", pss[0].Name)
		require.Len(t, pss[0].Ports, 1)
		require.Equal(t, "111:2222", pss[0].Ports[0])

		require.Equal(t, "app1", pss[1].App)
		require.Equal(t, 0.0, pss[1].Cpu)
		require.Equal(t, "5.6.7.8", pss[1].Host)
		require.Equal(t, "process2", pss[1].Id)
		require.Equal(t, 0.0, pss[1].Memory)
		require.Equal(t, "service2", pss[1].Name)
		require.Len(t, pss[1].Ports, 1)
		require.Equal(t, "333:4444", pss[1].Ports[0])

		require.Equal(t, "app1", pss[2].App)
		require.Equal(t, 0.0, pss[2].Cpu)
		require.Equal(t, "9.10.11.12", pss[2].Host)
		require.Equal(t, "process3", pss[2].Id)
		require.Equal(t, 0.0, pss[2].Memory)
		require.Equal(t, "service2", pss[2].Name)
		require.Len(t, pss[1].Ports, 1)
		require.Equal(t, "5555", pss[2].Ports[0])
	})
}

func processCreate(c kubernetes.Interface, ns, name, labels string) error {
	return processCreatePorts(c, ns, name, labels, "127.0.0.1", 123, 4567)
}

func processCreatePorts(c kubernetes.Interface, ns, name, labels, podIp string, hostPort, containerPort int32) error {
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
					Name: om.Labels["app"],
					Ports: []ac.ContainerPort{
						{
							ContainerPort: containerPort,
							HostPort:      hostPort,
						},
					},
					Resources: ac.ResourceRequirements{
						Requests: ac.ResourceList{
							"cpu": *(resource.NewMilliQuantity(256, resource.DecimalSI)),
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
