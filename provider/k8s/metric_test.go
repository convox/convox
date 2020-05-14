package k8s_test

import (
	ac "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	mc "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func podMetric(ns, name string, cpu, mem int64) *mc.PodMetrics {
	return &mc.PodMetrics{
		ObjectMeta: am.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
		Timestamp: am.Now(),
		Containers: []mc.ContainerMetrics{
			{Name: "main", Usage: ac.ResourceList{
				"cpu":    *(resource.NewMilliQuantity(cpu, resource.DecimalSI)),
				"memory": *(resource.NewScaledQuantity(mem, resource.Mega)),
			}},
		},
	}
}
