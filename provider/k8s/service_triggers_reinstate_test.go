package k8s_test

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/provider/k8s"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/stretchr/testify/require"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func TestServiceTriggersDisableReinstatesKedaOnlyForRange(t *testing.T) {
	cases := []struct {
		name          string
		minCount      int
		maxCount      int
		wantScaledObj bool
	}{
		{"fixed count", 1, 1, false},
		{"fixed count of zero", 0, 0, false},
		{"range", 0, 5, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProvider(t, func(p *k8s.Provider) {
				kk, _ := p.Cluster.(*kfake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))
				seedDeployment(t, kk, "rack1-app1", "worker", 2)
				dep, err := kk.AppsV1().Deployments("rack1-app1").Get(context.TODO(), "worker", am.GetOptions{})
				require.NoError(t, err)
				dep.Annotations = map[string]string{
					k8s.ServiceTriggersOverrideAnnotation:    k8s.ServiceTriggersOverrideValueOn,
					k8s.ServiceTriggersOverrideCRDAnnotation: k8s.TriggersCRDKeda,
				}
				_, err = kk.AppsV1().Deployments("rack1-app1").Update(context.TODO(), dep, am.UpdateOptions{})
				require.NoError(t, err)
				p.DynamicClient = fake.NewSimpleDynamicClient(newDynamicScheme(), scaledObjectUnstructured("rack1-app1", "worker", 2, 8))
				p.TriggersOverrideManifestServiceHook = func(_, s string) (*manifest.Service, error) {
					ms := &manifest.Service{Name: s}
					ms.Scale.Count.Min = tc.minCount
					ms.Scale.Count.Max = tc.maxCount
					ms.Scale.Keda = &manifest.ServiceScaleKeda{Triggers: []kedav1alpha1.ScaleTriggers{{
						Type:     "cron",
						Name:     "business-hours",
						Metadata: map[string]string{"timezone": "UTC", "start": "0 9 * * *", "end": "0 17 * * *", "desiredReplicas": "1"},
					}}}
					return ms, nil
				}

				require.NoError(t, p.ServiceTriggersDisable("app1", "worker", "alice@example.com"))

				_, err = p.DynamicClient.Resource(testScaledObjectGVR).Namespace("rack1-app1").Get(context.TODO(), "worker", am.GetOptions{})
				if tc.wantScaledObj {
					require.NoError(t, err)
				} else {
					require.True(t, kerr.IsNotFound(err), "want no ScaledObject, got %v", err)
				}
			})
		})
	}
}
