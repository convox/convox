package k8s

import (
	"context"
	"testing"

	apps "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestStatefulSetControllerSyncPDB(t *testing.T) {
	c := syncPDBController(t)
	sc := &StatefulSetController{Provider: c.Provider, logger: c.logger}
	replicas := int32(3)
	s := &apps.StatefulSet{
		ObjectMeta: am.ObjectMeta{
			Name: "database", Namespace: "ns1",
			Labels: map[string]string{"system": "convox", "rack": "test", "app": "app1", "service": "database"},
		},
		Spec: apps.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &am.LabelSelector{MatchLabels: map[string]string{"service": "database"}},
			Template: ac.PodTemplateSpec{ObjectMeta: am.ObjectMeta{}},
		},
	}

	if err := sc.sync(s, false); err != nil {
		t.Fatalf("sync() error: %v", err)
	}
	pdb, err := c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "database", am.GetOptions{})
	if err != nil {
		t.Fatalf("get pdb: %v", err)
	}
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.String() != "50%" {
		t.Fatalf("MinAvailable = %v, want 50%%", pdb.Spec.MinAvailable)
	}

	if err := sc.Delete(cache.DeletedFinalStateUnknown{Key: "ns1/database", Obj: s}); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err = c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "database", am.GetOptions{})
	if !kerr.IsNotFound(err) {
		t.Fatalf("get deleted pdb error = %v, want NotFound", err)
	}
}
