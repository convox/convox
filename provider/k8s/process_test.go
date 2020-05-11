package k8s_test

import (
	"strings"

	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
				{Name: "main"},
			},
		},
	}

	if _, err := c.CoreV1().Pods(ns).Create(p); err != nil {
		return err
	}

	return nil
}
