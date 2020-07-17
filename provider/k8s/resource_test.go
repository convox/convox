package k8s_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSystemResourceCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		opts1 := structs.ResourceCreateOptions{
			Parameters: map[string]string{"Url": "https://example1.org"},
		}

		r1, err := p.SystemResourceCreate("webhook", opts1)
		require.NoError(t, err)
		require.NotNil(t, r1)

		require.Regexp(t, "^webhook-[0-9a-f]+", r1.Name)
		require.Equal(t, map[string]string{"Url": "https://example1.org"}, r1.Parameters)
		require.Equal(t, "running", r1.Status)
		require.Equal(t, "webhook", r1.Type)

		opts2 := structs.ResourceCreateOptions{
			Name:       options.String("wh2"),
			Parameters: map[string]string{"Url": "https://example2.org"},
		}

		r2, err := p.SystemResourceCreate("webhook", opts2)
		require.NoError(t, err)
		require.NotNil(t, r2)

		require.Equal(t, "wh2", r2.Name)
		require.Equal(t, map[string]string{"Url": "https://example2.org"}, r2.Parameters)
		require.Equal(t, "running", r2.Status)
		require.Equal(t, "webhook", r2.Type)

		fc := p.Cluster.(*fake.Clientset)

		cm, err := fc.CoreV1().ConfigMaps(p.Namespace).Get("webhooks", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, cm)

		require.Equal(t, map[string]string{r1.Name: "https://example1.org", r2.Name: "https://example2.org"}, cm.Data)
	})
}

func TestSystemResourceDelete(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		opts := structs.ResourceCreateOptions{
			Parameters: map[string]string{"Url": "https://example.org"},
		}

		r, err := p.SystemResourceCreate("webhook", opts)
		require.NoError(t, err)
		require.NotNil(t, r)

		cm, err := fc.CoreV1().ConfigMaps(p.Namespace).Get("webhooks", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, cm)
		require.Equal(t, map[string]string{r.Name: "https://example.org"}, cm.Data)

		r2, err := p.SystemResourceGet(r.Name)
		require.NoError(t, err)
		require.NotNil(t, r2)
		require.Equal(t, r2, r)

		err = p.SystemResourceDelete(r.Name)
		require.NoError(t, err)

		r3, err := p.SystemResourceGet(r.Name)
		require.EqualError(t, err, fmt.Sprintf("no such resource: %s", r.Name))
		require.Nil(t, r3)

		cm2, err := fc.CoreV1().ConfigMaps(p.Namespace).Get("webhooks", am.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, cm2)
		require.Equal(t, map[string]string{}, cm2.Data)
	})
}

func TestSystemResourceGet(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "webhooks",
			},
			Data: map[string]string{
				"wh1": "https://example.org",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(cm)
		require.NoError(t, err)

		r, err := p.SystemResourceGet("wh1")
		require.NoError(t, err)
		require.NotNil(t, r)

		require.Equal(t, "wh1", r.Name)
		require.Equal(t, map[string]string{"Url": "https://example.org"}, r.Parameters)
		require.Equal(t, "running", r.Status)
		require.Equal(t, "webhook", r.Type)
	})
}

func TestSystemResourceList(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		fc := p.Cluster.(*fake.Clientset)

		cm := &ac.ConfigMap{
			ObjectMeta: am.ObjectMeta{
				Namespace: p.Namespace,
				Name:      "webhooks",
			},
			Data: map[string]string{
				"wh1": "https://example1.org",
				"wh2": "https://example2.org",
			},
		}

		_, err := fc.CoreV1().ConfigMaps(p.Namespace).Create(cm)
		require.NoError(t, err)

		rs, err := p.SystemResourceList()
		require.NoError(t, err)
		require.NotNil(t, rs)
		require.Len(t, rs, 2)

		require.Equal(t, "wh1", rs[0].Name)
		require.Equal(t, map[string]string{"Url": "https://example1.org"}, rs[0].Parameters)
		require.Equal(t, "running", rs[0].Status)
		require.Equal(t, "webhook", rs[0].Type)

		require.Equal(t, "wh2", rs[1].Name)
		require.Equal(t, map[string]string{"Url": "https://example2.org"}, rs[1].Parameters)
		require.Equal(t, "running", rs[1].Status)
		require.Equal(t, "webhook", rs[1].Type)
	})
}
