package k8s

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSystemJwtSignKeyRotateTakesOverHandEdits(t *testing.T) {
	ctx := context.TODO()

	for _, tc := range []struct {
		name       string
		deployment func(cs *fake.Clientset) error
		secret     func(cs *fake.Clientset) error
	}{
		{
			name: "signKey patched by hand",
			deployment: func(cs *fake.Clientset) error {
				_, err := cs.AppsV1().Deployments("rack-system").Create(ctx, &appsv1.Deployment{ObjectMeta: am.ObjectMeta{Name: "api"}}, am.CreateOptions{})
				return err
			},
			secret: func(cs *fake.Clientset) error {
				_, err := cs.CoreV1().Secrets("rack-system").Patch(ctx, ConvoxJwtSecretName, types.StrategicMergePatchType, []byte(`{"data":{"signKey":"aGFuZC1tYWRl"}}`), am.PatchOptions{FieldManager: "kubectl-patch"})
				return err
			},
		},
		{
			name: "restartAt owned by another manager",
			deployment: func(cs *fake.Clientset) error {
				d := &appsv1.Deployment{ObjectMeta: am.ObjectMeta{Name: "api"}}
				d.Spec.Template.Annotations = map[string]string{"convox.com/restartAt": "old"}
				_, err := cs.AppsV1().Deployments("rack-system").Create(ctx, d, am.CreateOptions{FieldManager: "HashiCorp"})
				return err
			},
			secret: func(*fake.Clientset) error { return nil },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs := fake.NewClientset()
			p := &Provider{Name: "rack", Namespace: "rack-system", Cluster: cs}

			require.NoError(t, tc.deployment(cs))
			_, err := p.SystemJwtSignKey()
			require.NoError(t, err)
			require.NoError(t, tc.secret(cs))

			key, err := p.SystemJwtSignKeyRotate()
			require.NoError(t, err)

			s, err := cs.CoreV1().Secrets("rack-system").Get(ctx, ConvoxJwtSecretName, am.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, key, base64.StdEncoding.EncodeToString(s.Data["signKey"]))

			d, err := cs.AppsV1().Deployments("rack-system").Get(ctx, "api", am.GetOptions{})
			require.NoError(t, err)
			require.NotEqual(t, "old", d.Spec.Template.Annotations["convox.com/restartAt"])
			require.NotEmpty(t, d.Spec.Template.Annotations["convox.com/restartAt"])
		})
	}
}
