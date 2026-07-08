package k8s

import (
	"context"
	"testing"

	"github.com/convox/logger"
	apps "k8s.io/api/apps/v1"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIsPdbDisabled(t *testing.T) {
	tests := []struct {
		name           string
		deployAnnots   map[string]string
		templateAnnots map[string]string
		want           bool
	}{
		{
			name: "no annotations",
			want: false,
		},
		{
			name:         "new spelling on deployment",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "true"},
			want:         true,
		},
		{
			name:           "new spelling on template",
			templateAnnots: map[string]string{AnnotationPdbDisabled: "true"},
			want:           true,
		},
		{
			name:         "old spelling on deployment",
			deployAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:         true,
		},
		{
			name:           "old spelling on template",
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
		{
			name:           "both spellings both true",
			deployAnnots:   map[string]string{AnnotationPdbDisabled: "true"},
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
		{
			name:         "value is false",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "false"},
			want:         false,
		},
		{
			name:         "value is empty string",
			deployAnnots: map[string]string{AnnotationPdbDisabled: ""},
			want:         false,
		},
		{
			name:         "value is 1",
			deployAnnots: map[string]string{AnnotationPdbDisabled: "1"},
			want:         false,
		},
		{
			name:         "unrelated annotations only",
			deployAnnots: map[string]string{"foo": "bar"},
			want:         false,
		},
		{
			name:           "mixed conflict new false deploy old true template",
			deployAnnots:   map[string]string{AnnotationPdbDisabled: "false"},
			templateAnnots: map[string]string{AnnotationPdbDisabledDeprecated: "true"},
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &apps.Deployment{
				ObjectMeta: am.ObjectMeta{Annotations: tt.deployAnnots},
				Spec: apps.DeploymentSpec{
					Template: ac.PodTemplateSpec{
						ObjectMeta: am.ObjectMeta{Annotations: tt.templateAnnots},
					},
				},
			}
			if got := isPdbDisabled(d); got != tt.want {
				t.Errorf("isPdbDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func syncPDBFixture(name string, replicas int32, annots map[string]string) *apps.Deployment {
	return &apps.Deployment{
		ObjectMeta: am.ObjectMeta{
			Name:        name,
			Namespace:   "ns1",
			Labels:      map[string]string{"system": "convox", "rack": "test", "app": "app1", "service": "web"},
			Annotations: annots,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &am.LabelSelector{
				MatchLabels: map[string]string{"service": "web"},
			},
			Template: ac.PodTemplateSpec{
				ObjectMeta: am.ObjectMeta{},
			},
		},
	}
}

func syncPDBController(t *testing.T) *DeployController {
	t.Helper()
	p := &Provider{
		Cluster:                          fake.NewSimpleClientset(),
		PdbDefaultMinAvailablePercentage: "50",
		ctx:                              context.Background(),
		logger:                           logger.New("ns=test"),
	}
	return &DeployController{Provider: p, logger: p.logger}
}

func TestSyncPDBReplicaModes(t *testing.T) {
	tests := []struct {
		name               string
		replicas           int32
		annots             map[string]string
		wantMinAvailable   string
		wantMaxUnavailable bool
	}{
		{
			name:               "single replica gets maxUnavailable 1",
			replicas:           1,
			wantMaxUnavailable: true,
		},
		{
			name:             "multi replica keeps minAvailable percentage",
			replicas:         3,
			wantMinAvailable: "50%",
		},
		{
			name:             "single replica with explicit annotation keeps annotation",
			replicas:         1,
			annots:           map[string]string{AnnotationPdbMinAvailable: "100%"},
			wantMinAvailable: "100%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := syncPDBController(t)
			d := syncPDBFixture("web", tt.replicas, tt.annots)

			if err := c.SyncPDB(d, false); err != nil {
				t.Fatalf("SyncPDB() error: %v", err)
			}

			pdb, err := c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "web", am.GetOptions{})
			if err != nil {
				t.Fatalf("get pdb: %v", err)
			}

			if tt.wantMaxUnavailable {
				if pdb.Spec.MaxUnavailable == nil || pdb.Spec.MaxUnavailable.IntValue() != 1 {
					t.Errorf("MaxUnavailable = %v, want 1", pdb.Spec.MaxUnavailable)
				}
				if pdb.Spec.MinAvailable != nil {
					t.Errorf("MinAvailable = %v, want nil", pdb.Spec.MinAvailable)
				}
			} else {
				if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.String() != tt.wantMinAvailable {
					t.Errorf("MinAvailable = %v, want %s", pdb.Spec.MinAvailable, tt.wantMinAvailable)
				}
				if pdb.Spec.MaxUnavailable != nil {
					t.Errorf("MaxUnavailable = %v, want nil", pdb.Spec.MaxUnavailable)
				}
			}
		})
	}
}

func TestSyncPDBScaleTransition(t *testing.T) {
	c := syncPDBController(t)

	if err := c.SyncPDB(syncPDBFixture("web", 3, nil), false); err != nil {
		t.Fatalf("SyncPDB(3 replicas) error: %v", err)
	}

	pdb, err := c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "web", am.GetOptions{})
	if err != nil {
		t.Fatalf("get pdb: %v", err)
	}
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.String() != "50%" {
		t.Fatalf("after 3 replicas: MinAvailable = %v, want 50%%", pdb.Spec.MinAvailable)
	}

	if err := c.SyncPDB(syncPDBFixture("web", 1, nil), false); err != nil {
		t.Fatalf("SyncPDB(1 replica) error: %v", err)
	}

	pdb, err = c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "web", am.GetOptions{})
	if err != nil {
		t.Fatalf("get pdb after scale down: %v", err)
	}
	if pdb.Spec.MaxUnavailable == nil || pdb.Spec.MaxUnavailable.IntValue() != 1 {
		t.Errorf("after scale to 1: MaxUnavailable = %v, want 1", pdb.Spec.MaxUnavailable)
	}
	if pdb.Spec.MinAvailable != nil {
		t.Errorf("after scale to 1: MinAvailable = %v, want nil", pdb.Spec.MinAvailable)
	}

	if err := c.SyncPDB(syncPDBFixture("web", 4, nil), false); err != nil {
		t.Fatalf("SyncPDB(back to 4) error: %v", err)
	}

	pdb, err = c.Provider.Cluster.PolicyV1().PodDisruptionBudgets("ns1").Get(context.Background(), "web", am.GetOptions{})
	if err != nil {
		t.Fatalf("get pdb after scale up: %v", err)
	}
	if pdb.Spec.MinAvailable == nil || pdb.Spec.MinAvailable.String() != "50%" {
		t.Errorf("after scale to 4: MinAvailable = %v, want 50%%", pdb.Spec.MinAvailable)
	}
	if pdb.Spec.MaxUnavailable != nil {
		t.Errorf("after scale to 4: MaxUnavailable = %v, want nil", pdb.Spec.MaxUnavailable)
	}
}
