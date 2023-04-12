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
				"params1": "test1",
				"params2": "test2",
				"params3": "test3",
				"params4": "test4",
				"params5": "test5",
				"params6": "test6",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		cmSync := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-sync",
			},
			Data: map[string]string{
				"params1": "true",
				"params2": "true",
				"params3": "true",
			},
		}

		_, err = fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cmSync, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, map[string]interface{}{
			"params4": "test4",
			"params5": "test5",
			"params6": "test6",
		}, params)

		tps, err := fc.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", tps.Data["params1"])
		require.Equal(t, "true", tps.Data["params2"])
		require.Equal(t, "true", tps.Data["params3"])
		require.Equal(t, "false", tps.Data["params4"])
		require.Equal(t, "false", tps.Data["params5"])
		require.Equal(t, "false", tps.Data["params6"])
	})
}

func TestRackParams2(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"params1": "test1",
				"params2": "test2",
				"params3": "test3",
				"params4": "test4",
				"params5": "test5",
				"cidr":    "test6",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		cmSync := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-sync",
			},
			Data: map[string]string{
				"params1": "true",
				"params2": "false",
				"params3": "true",
			},
		}

		_, err = fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cmSync, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, map[string]interface{}{
			"params2": "test2",
			"params4": "test4",
			"params5": "test5",
			"cidr":    "a66df261120b6c2311c6ef0b1bab4e583afcbcc0",
		}, params)

		tps, err := fc.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", tps.Data["params1"])
		require.Equal(t, "false", tps.Data["params2"])
		require.Equal(t, "true", tps.Data["params3"])
		require.Equal(t, "false", tps.Data["params4"])
		require.Equal(t, "false", tps.Data["params5"])
		require.Equal(t, "false", tps.Data["cidr"])
	})
}

func TestRackParamsWithoutSyncCM(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-params",
			},
			Data: map[string]string{
				"params1": "test1",
				"params2": "test2",
				"params3": "test3",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		params := p.RackParams()
		require.Equal(t, map[string]interface{}{
			"params1": "test1",
			"params2": "test2",
			"params3": "test3",
		}, params)

		tps, err := fc.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "false", tps.Data["params1"])
		require.Equal(t, "false", tps.Data["params2"])
		require.Equal(t, "false", tps.Data["params3"])
	})
}

func TestRackParamsMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		params := p.RackParams()
		require.Nil(t, params)
	})
}

func TestSyncParams(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "telemetry-rack-sync",
			},
			Data: map[string]string{
				"params1": "false",
				"params2": "true",
				"params3": "false",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(context.TODO(), cm, am.CreateOptions{})
		require.NoError(t, err)

		err = p.SyncParams()
		require.NoError(t, err)

		tps, err := fc.CoreV1().ConfigMaps(p.Namespace).Get(context.TODO(), "telemetry-rack-sync", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", tps.Data["params1"])
		require.Equal(t, "true", tps.Data["params2"])
		require.Equal(t, "true", tps.Data["params3"])
	})
}
