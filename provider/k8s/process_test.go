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
		Spec: ac.PodSpec{
			Containers: []ac.Container{
				{
					Name: "main",
					Ports: []ac.ContainerPort{
						{
							ContainerPort: 4567,
							HostPort:      123,
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

		require.NoError(t, processCreate(kk, "ns1", "process10", "system=convox,rack=rack1,app=app1,service=service1"))

		output, err := p.ProcessList("app1", structs.ProcessListOptions{})

		require.NoError(t, err)
		require.Len(t, output, 1)
		require.Len(t, output[0].Ports, 1)
		require.Equal(t, output[0].Ports[0], "123:4567")
	})
}
