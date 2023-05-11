package k8s_test

import (
	"context"
	"testing"

	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRackParams(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"params1":   "test1",
				"params2":   "test2",
				"params3":   "test3",
				"params4":   "test4",
				"params5":   "test5",
				"rack_name": "rack",
				"cidr":      "test6",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, map[string]interface{}{
			"params1": "test1",
			"params2": "test2",
			"params3": "test3",
			"params4": "test4",
			"params5": "test5",
			"cidr":    "ed0cb90bdfa4f93981a7d03cff99213a86aa96a6cbcf89ec5e8889871f088727",
		}, params)
	})
}

func TestRackParamsMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		params := p.RackParams()
		require.Nil(t, params)
	})
}
