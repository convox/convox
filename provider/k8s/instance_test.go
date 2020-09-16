package k8s_test

import (
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func nodeCreator(c kubernetes.Interface, name string, fn func(n *ac.Node)) error {
	n := &ac.Node{
		ObjectMeta: am.ObjectMeta{
			Name: name,
		},
	}

	if fn != nil {
		fn(n)
	}

	if _, err := c.CoreV1().Nodes().Create(n); err != nil {
		return err
	}

	return nil
}

func nodeCreateResources(c kubernetes.Interface, name, cpu, mem string) error {
	return nodeCreator(c, name, func(n *ac.Node) {
		n.Status.Capacity = ac.ResourceList{
			ac.ResourceCPU:    resource.MustParse(cpu),
			ac.ResourceMemory: resource.MustParse(mem),
		}
	})
}
